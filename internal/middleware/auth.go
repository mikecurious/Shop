package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/services"
)

const (
	UserIDKey   = "user_id"
	UserRoleKey = "user_role"
	UserNameKey = "user_name"
	UserEmailKey = "user_email"
	ClaimsKey   = "claims"
)

func AuthRequired(authSvc *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			// Check cookie
			cookie, err := c.Cookie("auth_token")
			if err != nil || cookie == "" {
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
			token = cookie
		}

		claims, err := authSvc.ValidateToken(token)
		if err != nil {
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set(ClaimsKey, claims)
		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, string(claims.Role))
		c.Set(UserNameKey, claims.Name)
		c.Set(UserEmailKey, claims.Email)
		c.Next()
	}
}

func APIAuthRequired(authSvc *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		claims, err := authSvc.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set(ClaimsKey, claims)
		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, string(claims.Role))
		c.Set(UserNameKey, claims.Name)
		c.Set(UserEmailKey, claims.Email)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(UserRoleKey)
		if role != string(models.RoleAdmin) {
			if isHTMXRequest(c) || isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			} else {
				c.HTML(http.StatusForbidden, "error.html", gin.H{"message": "Access denied"})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

func GetClaims(c *gin.Context) *models.Claims {
	val, exists := c.Get(ClaimsKey)
	if !exists {
		return nil
	}
	claims, _ := val.(*models.Claims)
	return claims
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}
	return ""
}

func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

func isAPIRequest(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json")
}
