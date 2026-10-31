package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/panel"
	"github.com/ilyas/vpn-service/backend/internal/utils"
)

func extractHost(raw string) string {
	if idx := strings.Index(raw, "://"); idx >= 0 {
		raw = raw[idx+3:]
	}
	if idx := strings.Index(raw, "/"); idx >= 0 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, ":"); idx >= 0 {
		raw = raw[:idx]
	}
	return raw
}

func rewriteLinkHost(link, oldHost, newHost string) string {
	if oldHost == "" || newHost == "" || oldHost == newHost {
		return link
	}
	if idx := strings.Index(link, "://"); idx >= 0 {
		afterProto := link[idx+3:]
		var authority, rest string
		if qIdx := strings.Index(afterProto, "?"); qIdx >= 0 {
			authority = afterProto[:qIdx]
			rest = afterProto[qIdx:]
		} else if hashIdx := strings.Index(afterProto, "#"); hashIdx >= 0 {
			authority = afterProto[:hashIdx]
			rest = afterProto[hashIdx:]
		} else {
			authority = afterProto
			rest = ""
		}
		if strings.Contains(authority, oldHost) {
			newAuthority := strings.Replace(authority, oldHost, newHost, 1)
			return link[:idx+3] + newAuthority + rest
		}
	}
	return link
}

func (h *Handler) rewritePanelLinks(links []string) []string {
	internalHost := extractHost(h.cfg.PanelURL)
	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	publicHost := extractHost(publicURL)
	if internalHost == "" || publicHost == "" || internalHost == publicHost {
		return links
	}
	for i, link := range links {
		links[i] = rewriteLinkHost(link, internalHost, publicHost)
	}
	return links
}

func (h *Handler) getSubBaseURL() string {
	baseSubURL := h.cfg.PanelSubURL
	if baseSubURL == "" {
		baseSubURL = h.cfg.PanelPublicURL
	}
	if baseSubURL == "" {
		baseSubURL = h.cfg.PanelURL
	}
	return strings.TrimRight(baseSubURL, "/")
}

func (h *Handler) getSubscription(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	sub, err := h.store.GetUserSubscription(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no subscription"})
		return
	}

	subURL := fmt.Sprintf("%s/%s", h.getSubBaseURL(), sub.PanelSubID.String)

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": subURL,
		"username":         sub.PanelEmail,
		"expires_at":       expiresAtMillis(sub.ExpiresAt),
	})
}

func (h *Handler) getVLESSConfig(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	sub, err := h.store.GetUserSubscription(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no subscription"})
		return
	}

	links, err := h.panel.GetLinks(c.Request.Context(), sub.PanelEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get links"})
		return
	}

	links = h.rewritePanelLinks(links)

	var vlessLink string
	for _, link := range links {
		if strings.HasPrefix(link, "vless://") {
			vlessLink = link
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"config_url": vlessLink,
		"username":   sub.PanelEmail,
	})
}

func (h *Handler) getSingBoxConfig(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	sub, err := h.store.GetUserSubscription(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no subscription"})
		return
	}

	subURL := fmt.Sprintf("%s/%s", h.getSubBaseURL(), sub.PanelSubID.String)

	resp, err := http.Get(subURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch subscription"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription not found"})
		return
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read subscription"})
		return
	}

	internalHost := extractHost(h.cfg.PanelURL)
	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	publicHost := extractHost(publicURL)
	if internalHost != "" && publicHost != "" && internalHost != publicHost {
		if strings.Contains(string(content), internalHost) {
			content = []byte(strings.ReplaceAll(string(content), internalHost, publicHost))
		}
	}

	c.Data(http.StatusOK, "application/json", content)
}

func (h *Handler) activateSubscription(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx := c.Request.Context()

	sub, err := h.store.GetUserSubscription(ctx, userID)
	if err == nil && sub.Status == "active" {
		clientInfo, getErr := h.panel.GetClient(ctx, sub.PanelEmail)
		if getErr == nil {
			if sub.PanelSubID.String == "" && clientInfo.SubID != "" {
				sub.PanelSubID = sql.NullString{String: clientInfo.SubID, Valid: true}
				h.store.UpdateSubscriptionSubID(ctx, sub.ID, clientInfo.SubID)
			}

			if !sub.ExpiresAt.Valid || sub.ExpiresAt.Time.Before(utils.NowMSK()) {
				if rerr := h.renewSubscription(ctx, sub); rerr != nil {
					h.log.Errorw("activate: renew error", "userID", maskInt(userID), "error", rerr)
				}
			}
		}

		links, _ := h.panel.GetLinks(ctx, sub.PanelEmail)
		links = h.rewritePanelLinks(links)

		subID := sub.PanelSubID.String
		if subID == "" && clientInfo != nil {
			subID = clientInfo.SubID
		}
		subURL := fmt.Sprintf("%s/%s", h.getSubBaseURL(), subID)

		var vlessLink string
		for _, link := range links {
			if strings.HasPrefix(link, "vless://") {
				vlessLink = link
				break
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"subscription_url": subURL,
			"links":            links,
			"username":         sub.PanelEmail,
			"group":            sub.GroupName,
			"vless":            vlessLink,
			"expires_at":       expiresAtMillis(sub.ExpiresAt),
		})
		return
	}

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	panelEmail := user.Username
	expiryMs := utils.NowMSK().AddDate(0, 0, h.cfg.DefaultSubscriptionDays).UnixMilli()

	var clientInfo *panel.ClientInfo

	existingClient, err := h.panel.GetClient(ctx, panelEmail)
	if err != nil {
		h.log.Debugw("activate: creating client in panel", "email", maskStr(panelEmail), "error", err)
		clientInfo, err = h.panel.AddClient(ctx, panelEmail, 0, expiryMs, h.cfg.DefaultInboundIDs)
		if err != nil {
			h.log.Errorw("activate: AddClient error", "email", maskStr(panelEmail), "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client in panel"})
			return
		}
	} else {
		h.log.Debugw("activate: updating existing client in panel", "email", maskStr(panelEmail))
		subID := existingClient.SubID
		if subID == "" {
			subID = panel.RandomSubID()
		}
		err = h.panel.UpdateClient(ctx, panelEmail, subID, expiryMs, h.cfg.DefaultInboundIDs)
		if err != nil {
			h.log.Errorw("activate: UpdateClient error", "email", maskStr(panelEmail), "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update client in panel"})
			return
		}
		clientInfo = existingClient
		clientInfo.SubID = subID
	}

	if err := h.panel.AddToGroup(ctx, []string{panelEmail}, h.cfg.DefaultGroup); err != nil {
		h.log.Errorw("activate: AddToGroup error", "email", maskStr(panelEmail), "group", h.cfg.DefaultGroup, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to group"})
		return
	}
	h.log.Infow("activate: AddToGroup ok", "email", maskStr(panelEmail), "group", h.cfg.DefaultGroup)

	if clientInfo == nil || clientInfo.SubID == "" {
		clientInfo, err = h.panel.GetClient(ctx, panelEmail)
		if err != nil {
			h.log.Errorw("activate: GetClient error", "email", maskStr(panelEmail), "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client info"})
			return
		}
	}
	h.log.Infow("activate: GetClient ok", "email", maskStr(panelEmail), "subId", clientInfo.SubID, "inboundIds", clientInfo.InboundIDs)

	sub = &models.Subscription{
		UserID:     userID,
		PanelEmail: panelEmail,
		PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
		GroupName:  h.cfg.DefaultGroup,
		Status:     "active",
		ExpiresAt:  sql.NullTime{Time: time.UnixMilli(expiryMs), Valid: true},
	}
	if err := h.store.CreateSubscription(ctx, sub); err != nil {
		h.log.Errorw("activate: CreateSubscription error", "userID", maskInt(userID), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subscription"})
		return
	}
	h.log.Infow("activate: CreateSubscription ok", "userID", maskInt(userID), "subId", clientInfo.SubID)

	subURL := fmt.Sprintf("%s/%s", h.getSubBaseURL(), clientInfo.SubID)

	links, _ := h.panel.GetLinks(ctx, panelEmail)
	links = h.rewritePanelLinks(links)

	var vlessLink string
	for _, link := range links {
		if strings.HasPrefix(link, "vless://") {
			vlessLink = link
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": subURL,
		"links":            links,
		"username":         panelEmail,
		"group":            sub.GroupName,
		"vless":            vlessLink,
		"expires_at":       expiryMs,
	})
}
