package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfFieldName  = "_csrf"
	csrfContextKey = "csrf_token"
)

// CSRF returns middleware that implements the double-submit cookie pattern.
// It is safe to skip on GET/HEAD/OPTIONS. Apply after auth middleware on
// protected routes. API routes (Bearer auth) are unaffected.
func CSRF(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getOrCreateToken(c, secret)
		c.Set(csrfContextKey, token)

		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		if !validateToken(c, token, secret) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}

		c.Next()
	}
}

// GetCSRFToken returns the CSRF token set by the middleware, or empty string.
func GetCSRFToken(c *gin.Context) string {
	v, _ := c.Get(csrfContextKey)
	s, _ := v.(string)
	return s
}

func getOrCreateToken(c *gin.Context, secret string) string {
	if cookie, err := c.Cookie(csrfCookieName); err == nil && isValidToken(cookie, secret) {
		// Refresh cookie on each request to extend its life
		setCSRFCookie(c, cookie)
		return cookie
	}

	token := newSignedToken(secret)
	setCSRFCookie(c, token)
	return token
}

func setCSRFCookie(c *gin.Context, token string) {
	c.SetCookie(csrfCookieName, token, 3600*8, "/", "", false, false) // not httpOnly so JS/HTMX can read it
}

func validateToken(c *gin.Context, cookieToken, secret string) bool {
	submitted := c.GetHeader(csrfHeaderName)
	if submitted == "" {
		submitted = c.PostForm(csrfFieldName)
	}
	if submitted == "" || cookieToken == "" {
		return false
	}
	if !isValidToken(submitted, secret) {
		return false
	}
	// Constant-time comparison
	return hmac.Equal([]byte(submitted), []byte(cookieToken))
}

func newSignedToken(secret string) string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic("csrf: failed to generate random bytes: " + err.Error())
	}
	return sign(hex.EncodeToString(raw), secret)
}

// sign returns hex(random) + "." + hex(HMAC(random, secret))
func sign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return data + "." + hex.EncodeToString(mac.Sum(nil))
}

func isValidToken(token, secret string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expected := sign(parts[0], secret)
	return hmac.Equal([]byte(expected), []byte(token))
}
