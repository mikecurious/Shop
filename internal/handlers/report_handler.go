package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/services"
)

type ReportHandler struct {
	reportSvc *services.ReportService
	saleSvc   *services.SaleService
}

func NewReportHandler(reportSvc *services.ReportService, saleSvc *services.SaleService) *ReportHandler {
	return &ReportHandler{reportSvc: reportSvc, saleSvc: saleSvc}
}

func (h *ReportHandler) Index(c *gin.Context) {
	c.HTML(http.StatusOK, "reports/index.html", gin.H{
		"title":  "Reports",
		"claims": middleware.GetClaims(c),
	})
}

func (h *ReportHandler) ExportSalesCSV(c *gin.Context) {
	from, to := parseDateRange(c)
	data, err := h.reportSvc.ExportSalesCSV(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename := fmt.Sprintf("sales_%s_%s.csv", from.Format("20060102"), to.Format("20060102"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv", data)
}

func (h *ReportHandler) ExportInventoryCSV(c *gin.Context) {
	data, err := h.reportSvc.ExportInventoryCSV(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=inventory.csv")
	c.Data(http.StatusOK, "text/csv", data)
}

func (h *ReportHandler) ExportPLCSV(c *gin.Context) {
	from, to := parseDateRange(c)
	groupBy := c.DefaultQuery("group_by", "day")
	data, err := h.reportSvc.ExportPLCSV(c.Request.Context(), from, to, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename := fmt.Sprintf("pl_%s_%s.csv", from.Format("20060102"), to.Format("20060102"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv", data)
}

func (h *ReportHandler) ExportPLPDF(c *gin.Context) {
	from, to := parseDateRange(c)
	data, err := h.reportSvc.GeneratePLPDF(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename := fmt.Sprintf("pl_%s_%s.pdf", from.Format("20060102"), to.Format("20060102"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/pdf", data)
}

func parseDateRange(c *gin.Context) (time.Time, time.Time) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		t, _ := time.Parse("2006-01-02", toStr)
		to = t.Add(24*time.Hour - time.Second)
	}
	return from, to
}
