package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
	"github.com/michaelbrian/kiosk/pkg/notifications"
	"github.com/rs/zerolog/log"
)

type NotificationService struct {
	db          *repository.DB
	emailSvc    *notifications.EmailService
	smsSvc      *notifications.SMSService
	productRepo *repository.ProductRepository
	userRepo    *repository.UserRepository
}

func NewNotificationService(
	db *repository.DB,
	emailSvc *notifications.EmailService,
	smsSvc *notifications.SMSService,
	productRepo *repository.ProductRepository,
	userRepo *repository.UserRepository,
) *NotificationService {
	return &NotificationService{
		db:          db,
		emailSvc:    emailSvc,
		smsSvc:      smsSvc,
		productRepo: productRepo,
		userRepo:    userRepo,
	}
}

func (s *NotificationService) SendLowStockAlerts(ctx context.Context) error {
	products, err := s.productRepo.GetLowStock(ctx)
	if err != nil || len(products) == 0 {
		return err
	}

	items := make([]notifications.LowStockItem, len(products))
	for i, p := range products {
		items[i] = notifications.LowStockItem{
			Name:         p.Name,
			SKU:          p.SKU,
			Quantity:     p.Quantity,
			ReorderLevel: p.ReorderLevel,
		}
	}

	users, err := s.userRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.Role != models.RoleAdmin {
			continue
		}

		if s.hasPreference(ctx, user.ID, models.AlertLowStock, models.AlertEmail) {
			if err := s.emailSvc.SendLowStock([]string{user.Email}, items); err != nil {
				log.Error().Err(err).Str("user", user.Email).Msg("low stock email failed")
			} else {
				s.logAlert(ctx, user.ID, models.AlertLowStock, models.AlertEmail,
					fmt.Sprintf("Low stock: %d products", len(products)))
			}
		}

		// SMS via Celcom Africa — requires user phone on record
		if s.hasPreference(ctx, user.ID, models.AlertLowStock, models.AlertSMS) {
			// Phone stored in user profile (future: add phone field to users table)
			s.logAlert(ctx, user.ID, models.AlertLowStock, models.AlertSMS,
				fmt.Sprintf("Low stock SMS queued: %d products", len(products)))
		}
	}
	return nil
}

func (s *NotificationService) SendDailySummary(ctx context.Context, stats *models.DashboardStats) error {
	summary := notifications.DailySummary{
		Date:        time.Now().Format("2006-01-02"),
		TotalSales:  stats.TodaySales,
		Revenue:     stats.TodayRevenue,
		COGS:        stats.TotalCOGS,
		GrossProfit: stats.GrossProfit,
	}

	users, err := s.userRepo.List(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.Role != models.RoleAdmin {
			continue
		}

		if s.hasPreference(ctx, user.ID, models.AlertDailySummary, models.AlertEmail) {
			if err := s.emailSvc.SendDailySummary([]string{user.Email}, summary); err != nil {
				log.Error().Err(err).Str("user", user.Email).Msg("daily summary email failed")
			} else {
				s.logAlert(ctx, user.ID, models.AlertDailySummary, models.AlertEmail,
					fmt.Sprintf("Daily summary: KES %.2f revenue", summary.Revenue))
			}
		}
	}
	return nil
}

// SendSMSAlert sends a plain-text SMS to an arbitrary phone number via Celcom Africa.
func (s *NotificationService) SendSMSAlert(ctx context.Context, userID uuid.UUID, phone, message string) error {
	if err := s.smsSvc.Send(phone, message); err != nil {
		return err
	}
	s.logAlert(ctx, userID, models.AlertPaymentReceived, models.AlertSMS, message)
	return nil
}

// SendPaymentNotification fires email + SMS on successful M-Pesa payment.
func (s *NotificationService) SendPaymentNotification(ctx context.Context, receipt, phone string, amount float64) {
	users, _ := s.userRepo.List(ctx)
	for _, user := range users {
		if user.Role != models.RoleAdmin {
			continue
		}
		if s.hasPreference(ctx, user.ID, models.AlertPaymentReceived, models.AlertEmail) {
			if err := s.emailSvc.SendPaymentConfirmation([]string{user.Email}, receipt, phone, amount); err != nil {
				log.Error().Err(err).Msg("payment email failed")
			}
		}
	}

	// SMS to the customer who paid
	msg := fmt.Sprintf("Payment of KES %.2f received. Receipt: %s. Thank you!", amount, receipt)
	if err := s.smsSvc.Send(phone, msg); err != nil {
		log.Warn().Err(err).Str("phone", phone).Msg("customer SMS failed")
	}
}

func (s *NotificationService) hasPreference(ctx context.Context, userID uuid.UUID, alertType models.AlertType, channel models.AlertChannel) bool {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled FROM alert_preferences
		WHERE user_id = $1 AND alert_type = $2 AND channel = $3
	`, userID, alertType, channel).Scan(&enabled)
	if err != nil {
		return channel == models.AlertEmail // default: email only
	}
	return enabled
}

func (s *NotificationService) logAlert(ctx context.Context, userID uuid.UUID, alertType models.AlertType, channel models.AlertChannel, message string) {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alerts (id, user_id, type, channel, message, status, sent_at, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, 'sent', $5, $5)
	`, userID, alertType, channel, message, now)
	if err != nil {
		log.Error().Err(err).Msg("alert log failed")
	}
}

func (s *NotificationService) GetAlertHistory(ctx context.Context, userID uuid.UUID, page, limit int) ([]models.Alert, int, error) {
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM alerts WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, type, channel, message, status, sent_at, created_at
		FROM alerts WHERE user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		if err := rows.Scan(&a.ID, &a.UserID, &a.Type, &a.Channel,
			&a.Message, &a.Status, &a.SentAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, a)
	}
	return alerts, total, rows.Err()
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID uuid.UUID, prefs []models.AlertPreference) error {
	for _, pref := range prefs {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO alert_preferences (id, user_id, alert_type, channel, enabled)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4)
			ON CONFLICT (user_id, alert_type, channel)
			DO UPDATE SET enabled = EXCLUDED.enabled
		`, userID, pref.AlertType, pref.Channel, pref.Enabled)
		if err != nil {
			return err
		}
	}
	return nil
}
