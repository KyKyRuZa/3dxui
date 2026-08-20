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

	internalHost := extractHost(h.cfg.PanelURL)
	publicURL = h.cfg.PanelPublicURL
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

	panelEmail := user.Username
	if user.TelegramID.Valid {
		panelEmail = user.Username
	}

	if _, err := h.panel.GetClient(ctx, panelEmail); err != nil {
		fmt.Printf("DEBUG activate: creating client in panel email=%s err=%v\n", panelEmail, err)
		if err := h.panel.AddClient(ctx, panelEmail, 0, 0, h.cfg.DefaultInboundIDs); err != nil {
			fmt.Printf("DEBUG activate: AddClient ERROR email=%s err=%v\n", panelEmail, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client in panel"})
			return
		}
		fmt.Printf("DEBUG activate: AddClient OK email=%s\n", panelEmail)
	}

	if err := h.panel.AddToGroup(ctx, []string{panelEmail}, h.cfg.DefaultGroup); err != nil {
		fmt.Printf("DEBUG activate: AddToGroup ERROR email=%s group=%s err=%v\n", panelEmail, h.cfg.DefaultGroup, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to group"})
		return
	}
	fmt.Printf("DEBUG activate: AddToGroup OK email=%s group=%s\n", panelEmail, h.cfg.DefaultGroup)

	clientInfo, err := h.panel.GetClient(ctx, panelEmail)
	if err != nil {
		fmt.Printf("DEBUG activate: GetClient ERROR email=%s err=%v\n", panelEmail, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client info"})
		return
	}
	fmt.Printf("DEBUG activate: GetClient OK email=%s subId=%s inboundIds=%v\n", panelEmail, clientInfo.SubID, clientInfo.InboundIDs)

	sub = &models.Subscription{
		UserID:     userID,
		PanelEmail: panelEmail,
		PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
		GroupName:  h.cfg.DefaultGroup,
		Status:     "active",
	}
	if err := h.store.CreateSubscription(ctx, sub); err != nil {
		fmt.Printf("DEBUG activate: CreateSubscription ERROR userID=%d err=%v\n", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subscription"})
		return
	}
	fmt.Printf("DEBUG activate: CreateSubscription OK userID=%d subId=%s\n", userID, clientInfo.SubID)

	publicURL := h.cfg.PanelPublicURL
	if publicURL == "" {
		publicURL = h.cfg.PanelURL
	}
	links, _ := h.panel.GetLinks(ctx, panelEmail)
	links = h.rewritePanelLinks(links)
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
