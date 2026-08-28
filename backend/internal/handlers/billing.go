package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/billing"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

func (h *Handler) listPlans(c *gin.Context) {
	plans, err := h.store.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

func (h *Handler) createPayment(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.billing == nil || h.cfg.YookassaShopID == "" || h.cfg.YookassaSecretKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "billing not configured"})
		return
	}

	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.PlanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	plan, err := h.store.GetPlan(c.Request.Context(), body.PlanID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown plan"})
		return
	}

	idem := billing.GenIdempotencyKey()
	amountValue := fmt.Sprintf("%.2f", float64(plan.PriceMinor)/100.0)
	metadata := map[string]string{
		"user_id": fmt.Sprintf("%d", userID),
		"plan_id": plan.ID,
	}
	description := fmt.Sprintf("NoMoreBlocks VPN — %s", plan.Name)

	p, err := h.billing.CreatePayment(idem, amountValue, plan.Currency, description, h.cfg.YookassaReturnURL, metadata)
	if err != nil {
		h.log.Errorw("createPayment: yookassa error", "userID", maskInt(userID), "plan", plan.ID, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create payment"})
		return
	}

	row := &models.PaymentRow{
		ID:          p.ID,
		UserID:      userID,
		PlanID:      plan.ID,
		Status:      p.Status,
		AmountMinor: plan.PriceMinor,
		Currency:    plan.Currency,
	}
	if err := h.store.CreatePayment(c.Request.Context(), row); err != nil {
		h.log.Errorw("createPayment: store error", "paymentID", maskStr(p.ID), "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":       p.ID,
		"status":           p.Status,
		"confirmation_url": p.Confirmation.ConfirmationURL,
	})
}

// billingWebhook handles YooKassa notifications. YooKassa does not sign
// webhooks, so we re-fetch the payment via the authenticated API to verify it
// actually succeeded before provisioning.
func (h *Handler) billingWebhook(c *gin.Context) {
	if h.billing == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "billing not configured"})
		return
	}

	var payload struct {
		Event  string `json:"event"`
		Object struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"object"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ctx := c.Request.Context()

	// Acknowledge everything YooKassa sends; only act on successful captures.
	// Cancellations: record and stop. Nothing to provision.
	if payload.Event == "payment.canceled" {
		_ = h.store.UpdatePaymentStatus(ctx, payload.Object.ID, "canceled")
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if payload.Event != "payment.succeeded" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	if payload.Object.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment id"})
		return
	}

	// Re-fetch from the API to verify authenticity and status.
	verified, err := h.billing.GetPayment(payload.Object.ID)
	if err != nil {
		h.log.Errorw("webhook: verify fetch error", "paymentID", maskStr(payload.Object.ID), "error", err)
		c.JSON(http.StatusOK, gin.H{"ok": true}) // keep retrying later
		return
	}
	if verified.Status != "succeeded" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	userID, planID := resolvePaymentTarget(ctx, h, verified, payload.Object.ID)
	if userID == 0 || planID == "" {
		h.log.Warnw("webhook: cannot resolve target", "paymentID", maskStr(payload.Object.ID))
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	// Idempotency: skip if we already activated this payment.
	if local, lerr := h.store.GetPayment(ctx, payload.Object.ID); lerr == nil && local.Status == "succeeded" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	plan, perr := h.store.GetPlan(ctx, planID)
	if perr != nil {
		h.log.Errorw("webhook: unknown plan", "paymentID", maskStr(payload.Object.ID), "plan", planID, "error", perr)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	// Reconcile the actually captured amount (major units, e.g. "299.00") against
	// the plan price (kopecks) and warn, but still provision — YooKassa already
	// captured, so we must fulfil.
	gotMinor := amountMinorFromString(verified.Amount.Value)
	if gotMinor > 0 && gotMinor != plan.PriceMinor {
		h.log.Warnw("webhook: amount mismatch", "paymentID", maskStr(payload.Object.ID), "plan", plan.ID, "expected", plan.PriceMinor, "got", gotMinor)
	}

	if err := h.provisionPlan(ctx, userID, plan); err != nil {
		h.log.Errorw("webhook: provisionPlan error", "userID", maskInt(userID), "plan", plan.ID, "error", err)
		c.JSON(http.StatusOK, gin.H{"ok": true}) // YooKassa will retry
		return
	}

	// Record the authoritative status and captured amount, then credit the
	// referrer (no-op if there is no pending referral).
	amountMinor := plan.PriceMinor
	if gotMinor > 0 {
		amountMinor = gotMinor
	}
	_ = h.store.SetPaymentResult(ctx, payload.Object.ID, "succeeded", amountMinor, verified.Amount.Currency)
	h.CreditReferralReward(ctx, userID)

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// resolvePaymentTarget extracts the user/plan for a payment, preferring the
// local DB row, then falling back to YooKassa metadata (useful when the
// create call's local write failed but the payment still succeeded).
func resolvePaymentTarget(ctx context.Context, h *Handler, verified *billing.Payment, paymentID string) (int64, string) {
	if local, err := h.store.GetPayment(ctx, paymentID); err == nil {
		return local.UserID, local.PlanID
	}
	if verified.Metadata != nil {
		if uid, ok := verified.Metadata["user_id"]; ok {
			if pid, ok2 := verified.Metadata["plan_id"]; ok2 {
				var parsed int64
				fmt.Sscanf(uid, "%d", &parsed)
				return parsed, pid
			}
		}
	}
	return 0, ""
}

// provisionPlan creates or extends the user's panel subscription according to
// the purchased plan's duration. It mirrors the renewal logic used by the bot
// and web activation, but uses the plan's duration instead of the global default.
func (h *Handler) provisionPlan(ctx context.Context, userID int64, plan *models.Plan) error {
	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	panelEmail := user.Username
	group := plan.GroupName
	if group == "" {
		group = h.cfg.DefaultGroup
	}
	days := plan.DurationDays
	if days <= 0 {
		days = h.cfg.DefaultSubscriptionDays
	}

	sub, err := h.store.GetUserSubscription(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		expiryMs := time.Now().AddDate(0, 0, days).UnixMilli()
		var clientInfo *panel.ClientInfo
		if _, gerr := h.panel.GetClient(ctx, panelEmail); gerr != nil {
			addClientInfo, aerr := h.panel.AddClient(ctx, panelEmail, 0, expiryMs, h.cfg.DefaultInboundIDs)
			if aerr != nil {
				return fmt.Errorf("AddClient: %w", aerr)
			}
			// AddClient already generated the subId sent to the panel; prefer it
			// over the (sometimes empty) GetClient response.
			if addClientInfo != nil && addClientInfo.SubID != "" {
				clientInfo = addClientInfo
			}
		}
		if clientInfo == nil {
			var gerr error
			clientInfo, gerr = h.panel.GetClient(ctx, panelEmail)
			if gerr != nil {
				return fmt.Errorf("GetClient: %w", gerr)
			}
		}
		if aerr := h.panel.AddToGroup(ctx, []string{panelEmail}, group); aerr != nil {
			return fmt.Errorf("AddToGroup: %w", aerr)
		}
		newSub := &models.Subscription{
			UserID:     userID,
			PanelEmail: panelEmail,
			PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
			GroupName:  group,
			Status:     "active",
			ExpiresAt:  sql.NullTime{Time: time.UnixMilli(expiryMs), Valid: true},
		}
		if cerr := h.store.CreateSubscription(ctx, newSub); cerr != nil {
			return fmt.Errorf("CreateSubscription: %w", cerr)
		}
		return nil
	}
	if err != nil {
		return err
	}

	subID := sub.PanelSubID.String
	if subID == "" {
		if ci, gerr := h.panel.GetClient(ctx, panelEmail); gerr == nil && ci.SubID != "" {
			subID = ci.SubID
			_ = h.store.UpdateSubscriptionSubID(ctx, sub.ID, subID)
		}
	}
	base := time.Now()
	if sub.ExpiresAt.Valid && sub.ExpiresAt.Time.After(base) {
		base = sub.ExpiresAt.Time
	}
	newExpiry := base.AddDate(0, 0, days)
	if uerr := h.panel.UpdateClient(ctx, panelEmail, subID, newExpiry.UnixMilli(), h.cfg.DefaultInboundIDs); uerr != nil {
		return fmt.Errorf("UpdateClient: %w", uerr)
	}
	if uerr := h.store.UpdateSubscriptionExpiry(ctx, sub.ID, newExpiry); uerr != nil {
		return fmt.Errorf("UpdateSubscriptionExpiry: %w", uerr)
	}
	return nil
}

// amountMinorFromString parses a YooKassa amount value (major units, e.g.
// "299.00" RUB) into kopecks. Returns 0 if empty or unparseable.
func amountMinorFromString(v string) int64 {
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return int64(f * 100)
}
