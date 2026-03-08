package mpesa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/michaelbrian/kiosk/internal/config"
)

const (
	sandboxBase    = "https://sandbox.safaricom.co.ke"
	productionBase = "https://api.safaricom.co.ke"
)

type Client struct {
	cfg        *config.MPesaConfig
	httpClient *http.Client
	baseURL    string
}

func NewClient(cfg *config.MPesaConfig) *Client {
	base := sandboxBase
	if cfg.Env == "production" {
		base = productionBase
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    base,
	}
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func (c *Client) GetAccessToken() (string, error) {
	creds := base64.StdEncoding.EncodeToString(
		[]byte(c.cfg.ConsumerKey + ":" + c.cfg.ConsumerSecret))

	req, err := http.NewRequest("GET", c.baseURL+"/oauth/v1/generate?grant_type=client_credentials", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	return tok.AccessToken, nil
}

type STKPushRequest struct {
	PhoneNumber string
	Amount      float64
	AccountRef  string
	Description string
}

type STKPushResponse struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
}

func (c *Client) STKPush(req STKPushRequest) (*STKPushResponse, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	password := base64.StdEncoding.EncodeToString(
		[]byte(c.cfg.Shortcode + c.cfg.Passkey + timestamp))

	phone := normalizePhone(req.PhoneNumber)

	payload := map[string]any{
		"BusinessShortCode": c.cfg.Shortcode,
		"Password":          password,
		"Timestamp":         timestamp,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            int(req.Amount),
		"PartyA":            phone,
		"PartyB":            c.cfg.Shortcode,
		"PhoneNumber":       phone,
		"CallBackURL":       c.cfg.CallbackURL,
		"AccountReference":  req.AccountRef,
		"TransactionDesc":   req.Description,
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequest("POST", c.baseURL+"/mpesa/stkpush/v1/processrequest", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stk push request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var stkResp STKPushResponse
	if err := json.Unmarshal(respBody, &stkResp); err != nil {
		return nil, fmt.Errorf("parse stk response: %w", err)
	}

	if stkResp.ResponseCode != "0" {
		return nil, fmt.Errorf("stk push failed: %s", stkResp.ResponseDescription)
	}

	return &stkResp, nil
}

// CallbackBody is the M-Pesa callback payload
type CallbackBody struct {
	Body struct {
		StkCallback struct {
			MerchantRequestID string `json:"MerchantRequestID"`
			CheckoutRequestID string `json:"CheckoutRequestID"`
			ResultCode        int    `json:"ResultCode"`
			ResultDesc        string `json:"ResultDesc"`
			CallbackMetadata  *struct {
				Item []struct {
					Name  string `json:"Name"`
					Value any    `json:"Value"`
				} `json:"Item"`
			} `json:"CallbackMetadata"`
		} `json:"stkCallback"`
	} `json:"Body"`
}

func (cb *CallbackBody) MpesaReceipt() string {
	if cb.Body.StkCallback.CallbackMetadata == nil {
		return ""
	}
	for _, item := range cb.Body.StkCallback.CallbackMetadata.Item {
		if item.Name == "MpesaReceiptNumber" {
			if v, ok := item.Value.(string); ok {
				return v
			}
		}
	}
	return ""
}

func (cb *CallbackBody) Amount() float64 {
	if cb.Body.StkCallback.CallbackMetadata == nil {
		return 0
	}
	for _, item := range cb.Body.StkCallback.CallbackMetadata.Item {
		if item.Name == "Amount" {
			switch v := item.Value.(type) {
			case float64:
				return v
			}
		}
	}
	return 0
}

func normalizePhone(phone string) string {
	phone = trimNonDigits(phone)
	if len(phone) == 9 {
		return "254" + phone
	}
	if len(phone) == 10 && phone[0] == '0' {
		return "254" + phone[1:]
	}
	return phone
}

func trimNonDigits(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			result = append(result, s[i])
		}
	}
	return string(result)
}
