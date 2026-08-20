package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/models"
)

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

	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	subURL := fmt.Sprintf("%s/sub/%s", strings.TrimRight(publicURL, "/"), sub.PanelSubID.String)

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": subURL,
		"username":         sub.PanelEmail,
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

	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	subURL := fmt.Sprintf("%s/sub/%s", strings.TrimRight(publicURL, "/"), sub.PanelSubID.String)

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
		links, _ := h.panel.GetLinks(ctx, sub.PanelEmail)
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

		c.JSON(http.StatusOK, gin.H{
			"subscription_url": subURL,
			"links":            links,
			"username":         sub.PanelEmail,
			"group":            sub.GroupName,
			"vless":            vlessLink,
		})
		return
	}

	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	panelEmail := fmt.Sprintf("user_%d@vpn.local", userID)
	if user.TelegramID.Valid {
		panelEmail = fmt.Sprintf("tg_%d@tg", user.TelegramID.Int64)
	}

	if _, err := h.panel.GetClient(ctx, panelEmail); err != nil {
		if err := h.panel.AddClient(ctx, panelEmail, 0, 0, h.cfg.DefaultInboundIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client in panel"})
			return
		}
	}

	if err := h.panel.AddToGroup(ctx, []string{panelEmail}, h.cfg.DefaultGroup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to group"})
		return
	}

	clientInfo, err := h.panel.GetClient(ctx, panelEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client info"})
		return
	}

	sub = &models.Subscription{
		UserID:     userID,
		PanelEmail: panelEmail,
		PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
		GroupName:  h.cfg.DefaultGroup,
		Status:     "active",
	}
	if err := h.store.CreateSubscription(ctx, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subscription"})
		return
	}

	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	links, _ := h.panel.GetLinks(ctx, panelEmail)
	subURL := fmt.Sprintf("%s/sub/%s", strings.TrimRight(publicURL, "/"), clientInfo.SubID)

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
	})
}
