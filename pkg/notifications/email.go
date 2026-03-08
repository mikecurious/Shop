package notifications

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/michaelbrian/kiosk/internal/config"
)

// EmailService sends mail via Google Workspace (smtp.gmail.com:587, STARTTLS).
// Use an App Password — not the account password — when 2FA is enabled.
type EmailService struct {
	cfg *config.EmailConfig
}

func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return &EmailService{cfg: cfg}
}

func (s *EmailService) Send(to []string, subject, body string) error {
	if s.cfg.SMTPUser == "" {
		return fmt.Errorf("email not configured")
	}

	from := fmt.Sprintf("%s <%s>", s.cfg.FromName, s.cfg.FromAddress)
	header := strings.Join([]string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	// Google Workspace uses STARTTLS on port 587 — smtp.PlainAuth handles this correctly
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	return smtp.SendMail(addr, auth, s.cfg.FromAddress, to, []byte(header))
}

func (s *EmailService) SendLowStock(to []string, products []LowStockItem) error {
	var rows strings.Builder
	for _, p := range products {
		fmt.Fprintf(&rows, `<tr>
			<td style="padding:8px;border:1px solid #ddd">%s</td>
			<td style="padding:8px;border:1px solid #ddd">%s</td>
			<td style="padding:8px;border:1px solid #ddd;color:red">%d</td>
			<td style="padding:8px;border:1px solid #ddd">%d</td>
		</tr>`, p.Name, p.SKU, p.Quantity, p.ReorderLevel)
	}

	body := fmt.Sprintf(`
	<div style="font-family:sans-serif;max-width:600px;margin:auto">
	  <h2 style="color:#e53e3e">Low Stock Alert</h2>
	  <p>The following products need restocking:</p>
	  <table style="border-collapse:collapse;width:100%%">
	    <thead>
	      <tr style="background:#f7fafc">
	        <th style="padding:8px;border:1px solid #ddd;text-align:left">Product</th>
	        <th style="padding:8px;border:1px solid #ddd;text-align:left">SKU</th>
	        <th style="padding:8px;border:1px solid #ddd;text-align:left">Stock</th>
	        <th style="padding:8px;border:1px solid #ddd;text-align:left">Reorder At</th>
	      </tr>
	    </thead>
	    <tbody>%s</tbody>
	  </table>
	  <p style="color:#718096;font-size:12px;margin-top:16px">Kiosk Manager — Automated Alert</p>
	</div>`, rows.String())

	return s.Send(to, "Low Stock Alert — Action Required", body)
}

func (s *EmailService) SendDailySummary(to []string, summary DailySummary) error {
	body := fmt.Sprintf(`
	<div style="font-family:sans-serif;max-width:500px;margin:auto">
	  <h2 style="color:#2d3748">Daily Sales Summary — %s</h2>
	  <table style="border-collapse:collapse;width:100%%">
	    <tr style="background:#f7fafc"><td style="padding:10px;font-weight:bold">Total Sales</td><td style="padding:10px">%d transactions</td></tr>
	    <tr><td style="padding:10px;font-weight:bold">Revenue</td><td style="padding:10px">KES %.2f</td></tr>
	    <tr style="background:#f7fafc"><td style="padding:10px;font-weight:bold">COGS</td><td style="padding:10px">KES %.2f</td></tr>
	    <tr><td style="padding:10px;font-weight:bold;color:green">Gross Profit</td><td style="padding:10px;color:green;font-weight:bold">KES %.2f</td></tr>
	  </table>
	  <p style="color:#718096;font-size:12px;margin-top:16px">Kiosk Manager — Daily Summary</p>
	</div>`, summary.Date, summary.TotalSales, summary.Revenue, summary.COGS, summary.GrossProfit)

	return s.Send(to, "Daily Sales Summary — "+summary.Date, body)
}

func (s *EmailService) SendPaymentConfirmation(to []string, receipt, phone string, amount float64) error {
	body := fmt.Sprintf(`
	<div style="font-family:sans-serif;max-width:500px;margin:auto">
	  <h2 style="color:#38a169">Payment Received</h2>
	  <table style="border-collapse:collapse;width:100%%">
	    <tr><td style="padding:8px;font-weight:bold">Receipt</td><td style="padding:8px">%s</td></tr>
	    <tr style="background:#f7fafc"><td style="padding:8px;font-weight:bold">Phone</td><td style="padding:8px">%s</td></tr>
	    <tr><td style="padding:8px;font-weight:bold">Amount</td><td style="padding:8px;color:green;font-weight:bold">KES %.2f</td></tr>
	  </table>
	  <p style="color:#718096;font-size:12px;margin-top:16px">Kiosk Manager — Payment Notification</p>
	</div>`, receipt, phone, amount)

	return s.Send(to, fmt.Sprintf("Payment Received — KES %.2f", amount), body)
}

type LowStockItem struct {
	Name         string
	SKU          string
	Quantity     int
	ReorderLevel int
}

type DailySummary struct {
	Date        string
	TotalSales  int
	Revenue     float64
	COGS        float64
	GrossProfit float64
}
