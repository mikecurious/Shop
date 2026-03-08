package notifications

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/michaelbrian/kiosk/internal/config"
)

// SMSService sends SMS via Celcom Africa.
// API docs: https://developers.celcomafrica.com
type SMSService struct {
	cfg    *config.CelcomConfig
	client *http.Client
}

func NewSMSService(cfg *config.CelcomConfig) *SMSService {
	return &SMSService{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type celcomResponse struct {
	ResponseCode        string `json:"ResponseCode"`
	Description         string `json:"Description"`
	MessageID           string `json:"MessageID"`
	MobileNumber        string `json:"MobileNumber"`
}

// Send delivers a plain-text SMS to a single recipient.
// phone should be in international format, e.g. "254712345678".
func (s *SMSService) Send(phone, message string) error {
	if s.cfg.APIKey == "" {
		return fmt.Errorf("Celcom Africa SMS not configured")
	}

	phone = normalizeSMSPhone(phone)

	form := url.Values{
		"apikey":    {s.cfg.APIKey},
		"partnerID": {s.cfg.PartnerID},
		"shortcode": {s.cfg.Shortcode},
		"mobile":    {phone},
		"message":   {message},
		"passType":  {"plain"},
	}

	req, err := http.NewRequest("POST", s.cfg.BaseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("sms request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms send: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result celcomResponse
	if err := json.Unmarshal(body, &result); err != nil {
		// Some Celcom endpoints return plain text on success
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		return fmt.Errorf("sms response parse: %w", err)
	}

	if result.ResponseCode != "200" && result.ResponseCode != "1" {
		return fmt.Errorf("sms failed [%s]: %s", result.ResponseCode, result.Description)
	}
	return nil
}

// SendBulk sends the same message to multiple recipients.
func (s *SMSService) SendBulk(phones []string, message string) []error {
	errs := make([]error, 0)
	for _, phone := range phones {
		if err := s.Send(phone, message); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", phone, err))
		}
	}
	return errs
}

func (s *SMSService) SendLowStockAlert(phone string, items []LowStockItem) error {
	msg := fmt.Sprintf("KIOSK LOW STOCK ALERT\n%d items need restocking:\n", len(items))
	for i, item := range items {
		if i >= 5 { // keep SMS concise
			msg += fmt.Sprintf("...and %d more.\n", len(items)-5)
			break
		}
		msg += fmt.Sprintf("- %s (SKU:%s) Qty:%d\n", item.Name, item.SKU, item.Quantity)
	}
	return s.Send(phone, msg)
}

func (s *SMSService) SendDailySummary(phone string, summary DailySummary) error {
	msg := fmt.Sprintf(
		"KIOSK DAILY SUMMARY %s\nSales: %d | Revenue: KES %.2f | Profit: KES %.2f",
		summary.Date, summary.TotalSales, summary.Revenue, summary.GrossProfit,
	)
	return s.Send(phone, msg)
}

func (s *SMSService) SendPaymentReceived(phone, receipt string, amount float64) error {
	msg := fmt.Sprintf(
		"KIOSK PAYMENT RECEIVED\nReceipt: %s\nAmount: KES %.2f\nThank you!",
		receipt, amount,
	)
	return s.Send(phone, msg)
}

// normalizeSMSPhone converts common Kenyan formats to international (254xxxxxxxxx).
func normalizeSMSPhone(phone string) string {
	// Strip non-digits
	digits := make([]byte, 0, len(phone))
	for i := 0; i < len(phone); i++ {
		if phone[i] >= '0' && phone[i] <= '9' {
			digits = append(digits, phone[i])
		}
	}
	s := string(digits)

	switch {
	case len(s) == 9: // 712345678
		return "254" + s
	case len(s) == 10 && s[0] == '0': // 0712345678
		return "254" + s[1:]
	case len(s) == 12 && strings.HasPrefix(s, "254"): // already international
		return s
	default:
		return s
	}
}
