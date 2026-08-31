package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
	"github.com/ilyas/vpn-service/backend/internal/utils"
)

func (h *Handler) botEnsureUser(c *gin.Context) {
	var body struct {
		TelegramID   int64  `json:"telegram_id"`
		FirstName    string `json:"first_name"`
		ReferralCode string `json:"referral_code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TelegramID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx := c.Request.Context()

	user, err := h.store.GetUserByTelegramID(ctx, body.TelegramID)
	if err == store.ErrNotFound {
		username := fmt.Sprintf("tg_%d", body.TelegramID)
		randomPass := utils.RandString(16)
		hash, herr := auth.HashPassword(randomPass)
		if herr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		user, err = h.store.CreateUser(ctx, username, "", hash)
		if errors.Is(err, store.ErrConflict) {
			// Username already taken (e.g. registered earlier) — reuse it.
			user, err = h.store.GetUserByUsername(ctx, username)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if serr := h.store.SetTelegramID(ctx, user.ID, body.TelegramID); serr != nil && !errors.Is(serr, store.ErrConflict) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sub, err := h.store.GetUserSubscription(ctx, user.ID)
	if err == store.ErrNotFound {
		panelEmail := user.Username
		expiryMs := time.Now().AddDate(0, 0, h.cfg.DefaultSubscriptionDays).UnixMilli()

		var addClientInfo *panel.ClientInfo
		if _, getErr := h.panel.GetClient(ctx, panelEmail); getErr != nil {
			h.log.Debugw("botEnsureUser: creating client", "email", maskStr(panelEmail), "error", getErr)
			addClientInfo, err = h.panel.AddClient(ctx, panelEmail, 0, expiryMs, h.cfg.DefaultInboundIDs)
			if err != nil {
				h.log.Errorw("botEnsureUser: AddClient error", "email", maskStr(panelEmail), "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client"})
				return
			}
			if addClientInfo != nil && addClientInfo.SubID != "" {
				h.log.Infow("botEnsureUser: AddClient ok", "email", maskStr(panelEmail), "subId", addClientInfo.SubID)
			} else {
				h.log.Infow("botEnsureUser: AddClient ok", "email", maskStr(panelEmail))
			}
		}

		if err := h.panel.AddToGroup(ctx, []string{panelEmail}, h.cfg.DefaultGroup); err != nil {
			h.log.Errorw("botEnsureUser: AddToGroup error", "email", maskStr(panelEmail), "group", h.cfg.DefaultGroup, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to group"})
			return
		}
		h.log.Infow("botEnsureUser: AddToGroup ok", "email", maskStr(panelEmail), "group", h.cfg.DefaultGroup)

		var clientInfo *panel.ClientInfo
		if addClientInfo != nil && addClientInfo.SubID != "" {
			clientInfo = addClientInfo
		} else {
			clientInfo, err = h.panel.GetClient(ctx, panelEmail)
			if err != nil {
				h.log.Errorw("botEnsureUser: GetClient error", "email", maskStr(panelEmail), "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client info"})
				return
			}
		}
		h.log.Infow("botEnsureUser: GetClient ok", "email", maskStr(panelEmail), "subId", clientInfo.SubID)

		sub = &models.Subscription{
			UserID:     user.ID,
			PanelEmail: panelEmail,
			PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
			GroupName:  h.cfg.DefaultGroup,
			Status:     "active",
			ExpiresAt:  sql.NullTime{Time: time.UnixMilli(expiryMs), Valid: true},
		}
		if err := h.store.CreateSubscription(ctx, sub); err != nil {
			if errors.Is(err, store.ErrConflict) {
				sub, err = h.store.GetUserSubscription(ctx, user.ID)
			} else {
				h.log.Errorw("botEnsureUser: CreateSubscription error", "userID", maskInt(user.ID), "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
		}
		h.log.Infow("botEnsureUser: CreateSubscription ok", "userID", maskInt(user.ID), "subId", clientInfo.SubID)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Renew a subscription that has no expiry yet or has already expired
	// (e.g. user pressed "Купить ключ" again). This also binds the new
	// time-limited model to previously unlimited (grandfathered) subscriptions.
	if !sub.ExpiresAt.Valid || sub.ExpiresAt.Time.Before(time.Now()) {
		if err := h.renewSubscription(ctx, sub); err != nil {
			h.log.Errorw("botEnsureUser: renew error", "userID", maskInt(user.ID), "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to renew subscription"})
			return
		}
	}

	// Referral: attribute the user to the referrer (rewarded on the referred
	// user's first paid purchase) and grant the referred user a one-time signup
	// bonus. This runs for both new and existing users — a referral link must
	// work even for accounts created before the link was clicked. The bonus and
	// the pending referral are recorded only once per referrer/referred pair.
	if body.ReferralCode != "" {
		if referrer, rerr := h.store.GetUserByReferralCode(ctx, body.ReferralCode); rerr == nil && referrer.ID != user.ID {
			if _, gerr := h.store.GetReferral(ctx, referrer.ID, user.ID); errors.Is(gerr, store.ErrNotFound) {
				if cerr := h.store.CreateReferral(ctx, referrer.ID, user.ID, h.cfg.ReferralRewardDays); cerr == nil {
					h.applyReferralSignupBonus(ctx, sub)
					// Notify the referrer that a friend joined via their link.
					if referrer.TelegramID.Valid && referrer.TelegramID.Int64 != 0 {
						if payload, jerr := json.Marshal(map[string]any{
							"referred_id": user.ID,
							"friend_name": body.FirstName,
							"reward_days": h.cfg.ReferralRewardDays,
						}); jerr == nil {
							_ = h.store.CreateBotNotification(ctx, referrer.TelegramID.Int64, "referral_signup",
								fmt.Sprintf("signup:%d:%d", referrer.ID, user.ID), payload)
						}
					}
				}
			}
		}
	}

	links, _ := h.panel.GetLinks(ctx, sub.PanelEmail)
	links = h.rewritePanelLinks(links)
	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	subURL := fmt.Sprintf("%s/sub/%s", strings.TrimRight(publicURL, "/"), sub.PanelSubID.String)

	var vlessLink string
	for _, link := range links {
		if strings.HasPrefix(link, "vless://") {
			vlessLink = link
			break
		}
	}

	singbox := []byte{}
	resp, gerr := http.Get(subURL)
	if gerr != nil {
		h.log.Errorw("botEnsureUser: fetch subscription error", "url", subURL, "error", gerr)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			h.log.Warnw("botEnsureUser: subscription not found", "url", subURL, "status", resp.StatusCode)
		} else if body, rerr := io.ReadAll(resp.Body); rerr == nil {
			singbox = body
		}
	}

	internalHost := extractHost(h.cfg.PanelURL)
	publicURL = h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	publicHost := extractHost(publicURL)
	if internalHost != "" && publicHost != "" && internalHost != publicHost {
		if strings.Contains(string(singbox), internalHost) {
			singbox = []byte(strings.ReplaceAll(string(singbox), internalHost, publicHost))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"provisioned":      true,
		"subscription_url": subURL,
		"links":            links,
		"vless":            vlessLink,
		"singbox":          string(singbox),
		"username":         sub.PanelEmail,
		"expires_at":       expiresAtMillis(sub.ExpiresAt),
	})
}

// renewSubscription extends a panel client's expiry by the configured default
// duration and persists the new expiry in our DB.
func (h *Handler) applyReferralSignupBonus(ctx context.Context, sub *models.Subscription) {
	if h.cfg.ReferralSignupBonusDays <= 0 {
		return
	}
	days := h.cfg.ReferralSignupBonusDays
	newExpiry := time.Now().AddDate(0, 0, days)
	if sub.ExpiresAt.Valid {
		newExpiry = sub.ExpiresAt.Time.AddDate(0, 0, days)
	}
	subID := sub.PanelSubID.String
	if subID == "" {
		if ci, gerr := h.panel.GetClient(ctx, sub.PanelEmail); gerr == nil && ci.SubID != "" {
			subID = ci.SubID
			_ = h.store.UpdateSubscriptionSubID(ctx, sub.ID, subID)
		}
	}
	if err := h.panel.UpdateClient(ctx, sub.PanelEmail, subID, newExpiry.UnixMilli(), h.cfg.DefaultInboundIDs); err != nil {
		h.log.Errorw("applyReferralSignupBonus: UpdateClient error", "userID", maskInt(sub.UserID), "error", err)
		return
	}
	if err := h.store.UpdateSubscriptionExpiry(ctx, sub.ID, newExpiry); err != nil {
		h.log.Errorw("applyReferralSignupBonus: UpdateSubscriptionExpiry error", "userID", maskInt(sub.UserID), "error", err)
		return
	}
	sub.ExpiresAt = sql.NullTime{Time: newExpiry, Valid: true}
}

func (h *Handler) renewSubscription(ctx context.Context, sub *models.Subscription) error {
	subID := sub.PanelSubID.String
	if subID == "" {
		if ci, gerr := h.panel.GetClient(ctx, sub.PanelEmail); gerr == nil && ci.SubID != "" {
			subID = ci.SubID
			_ = h.store.UpdateSubscriptionSubID(ctx, sub.ID, subID)
		}
	}
	expiryMs := time.Now().AddDate(0, 0, h.cfg.DefaultSubscriptionDays).UnixMilli()
	if err := h.panel.UpdateClient(ctx, sub.PanelEmail, subID, expiryMs, h.cfg.DefaultInboundIDs); err != nil {
		return err
	}
	expiresAt := time.UnixMilli(expiryMs)
	if err := h.store.UpdateSubscriptionExpiry(ctx, sub.ID, expiresAt); err != nil {
		return err
	}
	sub.ExpiresAt = sql.NullTime{Time: expiresAt, Valid: true}
	return nil
}

// CreditReferralReward rewards the referrer of the given (just paid) user with
// bonus days, extending the referrer's subscription. Called from the billing
// webhook once a paid purchase succeeds. No-op if there is no pending referral
// or the referrer has no active subscription.
func (h *Handler) CreditReferralReward(ctx context.Context, referredUserID int64) {
	ref, err := h.store.GetPendingReferral(ctx, referredUserID)
	if err != nil || ref == nil {
		return
	}
	if ref.ReferrerID == referredUserID {
		return
	}
	referrerSub, err := h.store.GetUserSubscription(ctx, ref.ReferrerID)
	if err != nil {
		h.log.Debugw("CreditReferralReward: referrer has no subscription", "referrer", maskInt(ref.ReferrerID), "error", err)
		return
	}
	days := h.cfg.ReferralRewardDays
	newExpiry := time.Now().AddDate(0, 0, days)
	if referrerSub.ExpiresAt.Valid {
		newExpiry = referrerSub.ExpiresAt.Time.AddDate(0, 0, days)
	}
	subID := referrerSub.PanelSubID.String
	if subID == "" {
		if ci, gerr := h.panel.GetClient(ctx, referrerSub.PanelEmail); gerr == nil && ci.SubID != "" {
			subID = ci.SubID
			_ = h.store.UpdateSubscriptionSubID(ctx, referrerSub.ID, subID)
		}
	}
	if err := h.panel.UpdateClient(ctx, referrerSub.PanelEmail, subID, newExpiry.UnixMilli(), h.cfg.DefaultInboundIDs); err != nil {
		h.log.Errorw("CreditReferralReward: UpdateClient error", "referrer", maskInt(ref.ReferrerID), "error", err)
		return
	}
	if err := h.store.UpdateSubscriptionExpiry(ctx, referrerSub.ID, newExpiry); err != nil {
		h.log.Errorw("CreditReferralReward: UpdateSubscriptionExpiry error", "referrer", maskInt(ref.ReferrerID), "error", err)
		return
	}
	if err := h.store.CompleteReferral(ctx, ref.ReferrerID, ref.ReferredID, days); err != nil {
		h.log.Errorw("CreditReferralReward: CompleteReferral error", "error", err)
		return
	}
	// Notify the referrer that the bonus was credited.
	if referrer, rerr := h.store.GetUserByID(ctx, ref.ReferrerID); rerr == nil && referrer.TelegramID.Valid && referrer.TelegramID.Int64 != 0 {
		if payload, jerr := json.Marshal(map[string]any{
			"referred_id": ref.ReferredID,
			"reward_days": days,
		}); jerr == nil {
			_ = h.store.CreateBotNotification(ctx, referrer.TelegramID.Int64, "referral_reward",
				fmt.Sprintf("reward:%d:%d", ref.ReferrerID, ref.ReferredID), payload)
		}
	}
	h.log.Infow("CreditReferralReward: rewarded", "referrer", maskInt(ref.ReferrerID), "days", days)
}

func (h *Handler) botReferral(c *gin.Context) {
	var body struct {
		TelegramID int64 `json:"telegram_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TelegramID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	user, err := h.store.GetUserByTelegramID(c.Request.Context(), body.TelegramID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	code, invited, earned, err := h.store.GetReferralStats(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"referral_code": code,
		"invited":       invited,
		"earned_days":   earned,
	})
}

// webReferral returns the current web user's referral stats and the bot
// username needed to build a shareable t.me deep link. Mirrors botReferral but
// is authenticated with the user's JWT instead of the bot API secret.
func (h *Handler) webReferral(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	code, invited, earned, err := h.store.GetReferralStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"referral_code": code,
		"invited":       invited,
		"earned_days":   earned,
		"bot_username":  h.cfg.BotUsername,
	})
}

func expiresAtMillis(nt sql.NullTime) int64 {
	if nt.Valid {
		return nt.Time.UnixMilli()
	}
	return 0
}

func (h *Handler) botGetUser(c *gin.Context) {
	telegramIDStr := c.Param("telegram_id")
	telegramID, _ := strconv.ParseInt(telegramIDStr, 10, 64)

	user, err := h.store.GetUserByTelegramID(c.Request.Context(), telegramID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	sub, err := h.store.GetUserSubscription(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"provisioned": false,
			"user":        user.Public(),
		})
		return
	}

	if sub.PanelSubID.String == "" {
		clientInfo, getErr := h.panel.GetClient(c.Request.Context(), sub.PanelEmail)
		if getErr == nil && clientInfo.SubID != "" {
			sub.PanelSubID = sql.NullString{String: clientInfo.SubID, Valid: true}
			h.store.UpdateSubscriptionSubID(c.Request.Context(), sub.ID, clientInfo.SubID)
		}
	}

	links, _ := h.panel.GetLinks(c.Request.Context(), sub.PanelEmail)
	links = h.rewritePanelLinks(links)
	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	subURL := fmt.Sprintf("%s/sub/%s", strings.TrimRight(publicURL, "/"), sub.PanelSubID.String)

	c.JSON(http.StatusOK, gin.H{
		"provisioned":      true,
		"subscription_url": subURL,
		"links":            links,
		"username":         sub.PanelEmail,
		"group":            sub.GroupName,
		"expires_at":       expiresAtMillis(sub.ExpiresAt),
		"user":             user.Public(),
	})
}

func (h *Handler) botExpiring(c *gin.Context) {
	ctx := c.Request.Context()
	before := time.Now().AddDate(0, 0, h.cfg.ExpiryNotifyDays)
	items, err := h.store.GetExpiringSubscriptions(ctx, before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ids := make([]int64, 0, len(items))
	users := make([]gin.H, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
		users = append(users, gin.H{
			"telegram_id": it.TelegramID,
			"username":    it.Username,
			"expires_at":  it.ExpiresAt.UnixMilli(),
		})
	}

	// Mark as notified for today so the loop does not re-send on the same day.
	if len(ids) > 0 {
		if err := h.store.MarkExpiryNotified(ctx, ids); err != nil {
			h.log.Errorw("botExpiring: mark notified error", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// botExpired returns subscriptions that just expired (last 24h) and have not
// been notified about the expiry yet, marking them notified for today.
func (h *Handler) botExpired(c *gin.Context) {
	ctx := c.Request.Context()
	since := time.Now().Add(-24 * time.Hour)
	items, err := h.store.GetExpiredSubscriptions(ctx, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ids := make([]int64, 0, len(items))
	users := make([]gin.H, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
		users = append(users, gin.H{
			"telegram_id": it.TelegramID,
			"username":    it.Username,
			"expires_at":  it.ExpiresAt.UnixMilli(),
		})
	}

	if len(ids) > 0 {
		if err := h.store.MarkExpiredNotified(ctx, ids); err != nil {
			h.log.Errorw("botExpired: mark notified error", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// botRenewed returns recently renewed subscriptions that have not been
// notified about the renewal yet, marking them as notified.
func (h *Handler) botRenewed(c *gin.Context) {
	ctx := c.Request.Context()
	items, err := h.store.GetPendingRenewalNotifications(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ids := make([]int64, 0, len(items))
	users := make([]gin.H, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
		users = append(users, gin.H{
			"telegram_id": it.TelegramID,
			"expires_at":  it.ExpiresAt.UnixMilli(),
		})
	}

	if len(ids) > 0 {
		if err := h.store.MarkRenewalNotified(ctx, ids); err != nil {
			h.log.Errorw("botRenewed: mark notified error", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// botNotifications returns generic pending bot notifications (referral
// signups/rewards, payment failures, etc.) and marks them as delivered so the
// loop does not re-send the same event.
func (h *Handler) botNotifications(c *gin.Context) {
	ctx := c.Request.Context()
	items, err := h.store.GetPendingBotNotifications(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ids := make([]int64, 0, len(items))
	notifs := make([]gin.H, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
		notifs = append(notifs, gin.H{
			"id":          it.ID,
			"telegram_id": it.TelegramID,
			"kind":        it.Kind,
			"data":        it.Data,
		})
	}

	if len(ids) > 0 {
		if err := h.store.MarkBotNotificationsNotified(ctx, ids); err != nil {
			h.log.Errorw("botNotifications: mark notified error", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifs})
}
