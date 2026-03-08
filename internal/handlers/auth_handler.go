package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/services"
)

type AuthHandler struct {
	authSvc *services.AuthService
}

func NewAuthHandler(authSvc *services.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	// Redirect if already logged in
	if _, err := c.Cookie("auth_token"); err == nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.HTML(http.StatusOK, "auth/login.html", gin.H{
		"title": "Login",
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "auth/login.html", gin.H{
			"title": "Login",
			"error": "Invalid email or password format",
		})
		return
	}

	token, user, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"title": "Login",
			"error": "Invalid email or password",
			"email": req.Email,
		})
		return
	}

	// Set JWT in httpOnly cookie (24 hours)
	c.SetCookie("auth_token", token, 86400, "/", "", false, true)

	_ = user
	c.Redirect(http.StatusFound, "/dashboard")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func (h *AuthHandler) ShowRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/register.html", gin.H{"title": "Register"})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "auth/register.html", gin.H{
			"title": "Register",
			"error": err.Error(),
		})
		return
	}

	_, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		c.HTML(http.StatusBadRequest, "auth/register.html", gin.H{
			"title": "Register",
			"error": err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/login?registered=1")
}

func (h *AuthHandler) ShowProfile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	c.HTML(http.StatusOK, "auth/profile.html", gin.H{
		"title":  "Profile",
		"claims": claims,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims := middleware.GetClaims(c)

	var form struct {
		OldPassword string `form:"old_password" binding:"required"`
		NewPassword string `form:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.HTML(http.StatusBadRequest, "auth/profile.html", gin.H{
			"title":  "Profile",
			"claims": claims,
			"error":  "New password must be at least 8 characters",
		})
		return
	}

	if err := h.authSvc.ChangePassword(c.Request.Context(), claims.UserID, form.OldPassword, form.NewPassword); err != nil {
		c.HTML(http.StatusBadRequest, "auth/profile.html", gin.H{
			"title":  "Profile",
			"claims": claims,
			"error":  err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "auth/profile.html", gin.H{
		"title":   "Profile",
		"claims":  claims,
		"success": "Password changed successfully",
	})
}

// API endpoints
func (h *AuthHandler) APILogin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, user, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": time.Now().Add(24 * time.Hour),
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}
