package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/models"
	"github.com/ilyas/vpn-service/backend/internal/store"
	"github.com/ilyas/vpn-service/backend/internal/utils"
)

func (h *Handler) botEnsureUser(c *gin.Context) {
	var body struct {
		TelegramID int64  `json:"telegram_id"`
		FirstName  string `json:"first_name"`
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
		hash, err := auth.HashPassword(randomPass)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		user, err = h.store.CreateUser(ctx, username, "", hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if err := h.store.SetTelegramID(ctx, user.ID, body.TelegramID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sub, err := h.store.GetUserSubscription(ctx, user.ID)
	if err == store.ErrNotFound {
		panelEmail := fmt.Sprintf("tg_%d@tg", body.TelegramID)

		if _, err := h.panel.GetClient(ctx, panelEmail); err != nil {
			fmt.Printf("DEBUG botEnsureUser: creating client email=%s err=%v\n", panelEmail, err)
			if err := h.panel.AddClient(ctx, panelEmail, 0, 0, h.cfg.DefaultInboundIDs); err != nil {
				fmt.Printf("DEBUG botEnsureUser: AddClient ERROR email=%s err=%v\n", panelEmail, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client"})
				return
			}
			fmt.Printf("DEBUG botEnsureUser: AddClient OK email=%s\n", panelEmail)
		}

		if err := h.panel.AddToGroup(ctx, []string{panelEmail}, h.cfg.DefaultGroup); err != nil {
			fmt.Printf("DEBUG botEnsureUser: AddToGroup ERROR email=%s group=%s err=%v\n", panelEmail, h.cfg.DefaultGroup, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add to group"})
			return
		}
		fmt.Printf("DEBUG botEnsureUser: AddToGroup OK email=%s group=%s\n", panelEmail, h.cfg.DefaultGroup)

		clientInfo, err := h.panel.GetClient(ctx, panelEmail)
		if err != nil {
			fmt.Printf("DEBUG botEnsureUser: GetClient ERROR email=%s err=%v\n", panelEmail, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client info"})
			return
		}
		fmt.Printf("DEBUG botEnsureUser: GetClient OK email=%s subId=%s\n", panelEmail, clientInfo.SubID)

		sub = &models.Subscription{
			UserID:     user.ID,
			PanelEmail: panelEmail,
			PanelSubID: sql.NullString{String: clientInfo.SubID, Valid: clientInfo.SubID != ""},
			GroupName:  h.cfg.DefaultGroup,
			Status:     "active",
		}
		if err := h.store.CreateSubscription(ctx, sub); err != nil {
			fmt.Printf("DEBUG botEnsureUser: CreateSubscription ERROR userID=%d err=%v\n", user.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		fmt.Printf("DEBUG botEnsureUser: CreateSubscription OK userID=%d subId=%s\n", user.ID, clientInfo.SubID)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	links, _ := h.panel.GetLinks(ctx, sub.PanelEmail)
	links = h.rewritePanelLinks(links)
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
	singbox, err := io.ReadAll(resp.Body)
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
		if strings.Contains(string(singbox), internalHost) {
			singbox = []byte(strings.ReplaceAll(string(singbox), internalHost, publicHost))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"provisioned":      true,
		"subscription_url": subURL,
		"links":            links,
		"singbox":          string(singbox),
		"username":         sub.PanelEmail,
	})
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
		"user":             user.Public(),
	})
}

func (h *Handler) botExpiring(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []interface{}{}})
}
