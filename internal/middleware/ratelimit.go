package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
)

func RateLimit(rateStr string) gin.HandlerFunc {
	rate, err := limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		// Default: 10 per minute
		rate, _ = limiter.NewRateFromFormatted("10-M")
	}

	store := memory.NewStore()
	instance := limiter.New(store, rate)
	middleware := ginlimiter.NewMiddleware(instance)

	return func(c *gin.Context) {
		middleware(c)
		if c.IsAborted() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please slow down",
			})
		}
	}
}
