package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/models"
)

const adminCookieName = "admin_session"

type adminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) adminLogin(c *gin.Context) {
	var body adminLoginRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	user, err := h.store.GetUserByUsername(c.Request.Context(), body.Username)
	if err != nil || !user.IsActive || !user.IsAdmin {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := h.store.VerifyPassword(c.Request.Context(), body.Username, body.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	cookie := &http.Cookie{
		Name:     adminCookieName,
		Value:    "ok",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	}
	http.SetCookie(c.Writer, cookie)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) adminLogout(c *gin.Context) {
	cookie := &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
	http.SetCookie(c.Writer, cookie)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) adminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(adminCookieName)
		if err != nil || cookie.Value != "ok" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func (h *Handler) adminListPlans(c *gin.Context) {
	plans, err := h.store.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

func (h *Handler) adminCreatePlan(c *gin.Context) {
	var p models.Plan
	if err := c.ShouldBindJSON(&p); err != nil || p.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if p.DurationDays <= 0 || p.PriceMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_days must be > 0 and price_minor >= 0"})
		return
	}
	if err := h.store.CreatePlan(c.Request.Context(), &p); err != nil {
		h.log.Errorw("admin create plan error", "plan", p.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) adminUpdatePlan(c *gin.Context) {
	id := c.Param("id")
	var p models.Plan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	p.ID = id
	if p.DurationDays <= 0 || p.PriceMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_days must be > 0 and price_minor >= 0"})
		return
	}
	if err := h.store.UpdatePlan(c.Request.Context(), &p); err != nil {
		h.log.Errorw("admin update plan error", "plan", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) adminDeletePlan(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeletePlan(c.Request.Context(), id); err != nil {
		h.log.Errorw("admin delete plan error", "plan", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) adminListDiscounts(c *gin.Context) {
	items, err := h.store.ListDiscounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discounts": items})
}

func (h *Handler) adminCreateDiscount(c *gin.Context) {
	var d models.Discount
	if err := c.ShouldBindJSON(&d); err != nil || d.ID == "" || d.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if d.Percent < 0 || d.Percent > 100 || d.FixedMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid discount values"})
		return
	}
	if d.StartsAt.After(d.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "starts_at must be before expires_at"})
		return
	}
	if err := h.store.CreateDiscount(c.Request.Context(), &d); err != nil {
		h.log.Errorw("admin create discount error", "discount", d.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *Handler) adminUpdateDiscount(c *gin.Context) {
	id := c.Param("id")
	var d models.Discount
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	d.ID = id
	if d.Percent < 0 || d.Percent > 100 || d.FixedMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid discount values"})
		return
	}
	if d.StartsAt.After(d.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "starts_at must be before expires_at"})
		return
	}
	if err := h.store.UpdateDiscount(c.Request.Context(), &d); err != nil {
		h.log.Errorw("admin update discount error", "discount", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *Handler) adminDeleteDiscount(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteDiscount(c.Request.Context(), id); err != nil {
		h.log.Errorw("admin delete discount error", "discount", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
