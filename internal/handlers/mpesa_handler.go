package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/michaelbrian/kiosk/internal/middleware"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/pkg/mpesa"
)

type MPesaHandler struct {
	paymentRepo *repository.PaymentRepository
	mpesaClient *mpesa.Client
}

func NewMPesaHandler(paymentRepo *repository.PaymentRepository, mpesaClient *mpesa.Client) *MPesaHandler {
	return &MPesaHandler{paymentRepo: paymentRepo, mpesaClient: mpesaClient}
}

func (h *MPesaHandler) Index(c *gin.Context) {
	page, limit := paginationParams(c)
	fromStr := c.Query("from")
	toStr := c.Query("to")
	status := c.Query("status")

	var from, to time.Time
	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		t, _ := time.Parse("2006-01-02", toStr)
		to = t.Add(24*time.Hour - time.Second)
	}

	payments, total, err := h.paymentRepo.List(c.Request.Context(), from, to, status, page, limit)
	if err != nil {
		renderError(c, err)
		return
	}

	c.HTML(http.StatusOK, "sales/payments.html", gin.H{
		"title":      "M-Pesa Payments",
		"payments":   payments,
		"pagination": newPagination(page, limit, total),
		"filter": gin.H{
			"from":   fromStr,
			"to":     toStr,
			"status": status,
		},
		"claims": middleware.GetClaims(c),
	})
}

func (h *MPesaHandler) STKPush(c *gin.Context) {
	var req struct {
		PhoneNumber string  `json:"phone_number" binding:"required"`
		Amount      float64 `json:"amount" binding:"required,min=1"`
		AccountRef  string  `json:"account_ref"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.AccountRef == "" {
		req.AccountRef = "KioskPay"
	}
	if req.Description == "" {
		req.Description = "Kiosk Manager Payment"
	}

	resp, err := h.mpesaClient.STKPush(mpesa.STKPushRequest{
		PhoneNumber: req.PhoneNumber,
		Amount:      req.Amount,
		AccountRef:  req.AccountRef,
		Description: req.Description,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save pending payment record
	payment := &models.Payment{
		PhoneNumber:       req.PhoneNumber,
		Amount:            req.Amount,
		Status:            models.PaymentPending,
		CheckoutRequestID: resp.CheckoutRequestID,
		MerchantRequestID: resp.MerchantRequestID,
		TransactionDate:   time.Now(),
	}
	_ = h.paymentRepo.Create(c.Request.Context(), payment)

	c.JSON(http.StatusOK, gin.H{
		"checkout_request_id": resp.CheckoutRequestID,
		"merchant_request_id": resp.MerchantRequestID,
		"message":             "STK push sent. Please check your phone.",
	})
}

func (h *MPesaHandler) Callback(c *gin.Context) {
	var callback mpesa.CallbackBody
	if err := json.NewDecoder(c.Request.Body).Decode(&callback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ResultCode": 1, "ResultDesc": "invalid payload"})
		return
	}

	stk := callback.Body.StkCallback
	checkoutID := stk.CheckoutRequestID

	var status models.PaymentStatus
	if stk.ResultCode == 0 {
		status = models.PaymentCompleted
	} else {
		status = models.PaymentFailed
	}

	receipt := callback.MpesaReceipt()

	if err := h.paymentRepo.UpdateStatus(c.Request.Context(),
		checkoutID, receipt,
		fmt.Sprintf("%d", stk.ResultCode),
		stk.ResultDesc,
		status,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ResultCode": 1, "ResultDesc": "update failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ResultCode": 0, "ResultDesc": "Accepted"})
}

func (h *MPesaHandler) CheckStatus(c *gin.Context) {
	checkoutID := c.Param("checkout_id")
	payment, err := h.paymentRepo.GetByCheckoutID(c.Request.Context(), checkoutID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}
	c.JSON(http.StatusOK, payment)
}

func (h *MPesaHandler) LinkToSale(c *gin.Context) {
	var req struct {
		PaymentID string `json:"payment_id" binding:"required"`
		SaleID    string `json:"sale_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.paymentRepo.LinkToSale(c.Request.Context(), req.PaymentID, req.SaleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "payment linked to sale"})
}
