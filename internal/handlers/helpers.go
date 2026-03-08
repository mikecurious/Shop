package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// mustParseUUID returns the string as-is (IDs are now plain strings).
func mustParseUUID(s string) string {
	return s
}

// parseUUIDParam extracts a string ID from a URL param and validates it's non-empty.
func parseUUIDParam(c *gin.Context, param string) (string, bool) {
	id := c.Param(param)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return "", false
	}
	return id, true
}

func paginationParams(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return
}

type Pagination struct {
	Page       int
	Limit      int
	Total      int
	TotalPages int
	HasPrev    bool
	HasNext    bool
}

func newPagination(page, limit, total int) Pagination {
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
	}
}
