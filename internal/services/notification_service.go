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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

		if s.hasPreference(ctx, user.ID, models.AlertLowStock, models.AlertSMS) {
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

func (s *NotificationService) SendSMSAlert(ctx context.Context, userID string, phone, message string) error {
	if err := s.smsSvc.Send(phone, message); err != nil {
		return err
	}
	s.logAlert(ctx, userID, models.AlertPaymentReceived, models.AlertSMS, message)
	return nil
}

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

	msg := fmt.Sprintf("Payment of KES %.2f received. Receipt: %s. Thank you!", amount, receipt)
	if err := s.smsSvc.Send(phone, msg); err != nil {
		log.Warn().Err(err).Str("phone", phone).Msg("customer SMS failed")
	}
}

func (s *NotificationService) hasPreference(ctx context.Context, userID string, alertType models.AlertType, channel models.AlertChannel) bool {
	var pref models.AlertPreference
	err := s.db.Collection("alert_preferences").FindOne(ctx, bson.M{
		"user_id":    userID,
		"alert_type": alertType,
		"channel":    channel,
	}).Decode(&pref)
	if err != nil {
		return channel == models.AlertEmail
	}
	return pref.Enabled
}

func (s *NotificationService) logAlert(ctx context.Context, userID string, alertType models.AlertType, channel models.AlertChannel, message string) {
	now := time.Now()
	alert := models.Alert{
		ID:        uuid.New().String(),
		UserID:    userID,
		Type:      alertType,
		Channel:   channel,
		Message:   message,
		Status:    "sent",
		SentAt:    &now,
		CreatedAt: now,
	}
	if _, err := s.db.Collection("alerts").InsertOne(ctx, alert); err != nil {
		log.Error().Err(err).Msg("alert log failed")
	}
}

func (s *NotificationService) GetAlertHistory(ctx context.Context, userID string, page, limit int) ([]models.Alert, int, error) {
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	col := s.db.Collection("alerts")
	filter := bson.M{"user_id": userID}

	total64, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64((page - 1) * limit))

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var alerts []models.Alert
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, 0, err
	}
	return alerts, int(total64), nil
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID string, prefs []models.AlertPreference) error {
	col := s.db.Collection("alert_preferences")
	for _, pref := range prefs {
		filter := bson.M{
			"user_id":    userID,
			"alert_type": pref.AlertType,
			"channel":    pref.Channel,
		}
		update := bson.M{
			"$set": bson.M{"enabled": pref.Enabled},
			"$setOnInsert": bson.M{
				"_id":        uuid.New().String(),
				"user_id":    userID,
				"alert_type": pref.AlertType,
				"channel":    pref.Channel,
			},
		}
		_, err := col.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
	}
	return nil
}
