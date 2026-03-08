package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	DB        DBConfig
	JWT       JWTConfig
	CSRF      CSRFConfig
	MPesa     MPesaConfig
	Email     EmailConfig
	Celcom    CelcomConfig
	RateLimit RateLimitConfig
	Log       LogConfig
}

type AppConfig struct {
	Env       string
	Port      string
	SecretKey string
	BaseURL   string
}

type DBConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

type CSRFConfig struct {
	Secret string
}

type MPesaConfig struct {
	ConsumerKey    string
	ConsumerSecret string
	Shortcode      string
	Passkey        string
	CallbackURL    string
	Env            string
}

type EmailConfig struct {
	// Google Workspace — use smtp.gmail.com:587 with an App Password
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string // Google Workspace email address
	SMTPPassword string // Google Workspace App Password (not account password)
	FromAddress  string
	FromName     string
}

// CelcomConfig holds Celcom Africa SMS API credentials.
// Docs: https://developers.celcomafrica.com
type CelcomConfig struct {
	APIKey    string
	PartnerID string
	Shortcode string
	BaseURL   string // defaults to https://sms.celcomafrica.com/api/services/sendsms/
}

type RateLimitConfig struct {
	Auth string
	API  string
}

type LogConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.App = AppConfig{
		Env:       getEnv("APP_ENV", "development"),
		Port:      getEnv("APP_PORT", "8080"),
		SecretKey: mustGetEnv("APP_SECRET_KEY"),
		BaseURL:   getEnv("APP_BASE_URL", "http://localhost:8080"),
	}

	maxOpen, _ := strconv.Atoi(getEnv("DB_MAX_OPEN_CONNS", "25"))
	maxIdle, _ := strconv.Atoi(getEnv("DB_MAX_IDLE_CONNS", "5"))
	cfg.DB = DBConfig{
		Host:         getEnv("DB_HOST", "localhost"),
		Port:         getEnv("DB_PORT", "5432"),
		User:         mustGetEnv("DB_USER"),
		Password:     mustGetEnv("DB_PASSWORD"),
		Name:         mustGetEnv("DB_NAME"),
		SSLMode:      getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns: maxOpen,
		MaxIdleConns: maxIdle,
	}

	jwtExpiry, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	cfg.JWT = JWTConfig{
		Secret:      mustGetEnv("JWT_SECRET"),
		ExpiryHours: jwtExpiry,
	}

	cfg.CSRF = CSRFConfig{
		Secret: mustGetEnv("CSRF_SECRET"),
	}

	cfg.MPesa = MPesaConfig{
		ConsumerKey:    getEnv("MPESA_CONSUMER_KEY", ""),
		ConsumerSecret: getEnv("MPESA_CONSUMER_SECRET", ""),
		Shortcode:      getEnv("MPESA_SHORTCODE", ""),
		Passkey:        getEnv("MPESA_PASSKEY", ""),
		CallbackURL:    getEnv("MPESA_CALLBACK_URL", ""),
		Env:            getEnv("MPESA_ENV", "sandbox"),
	}

	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	cfg.Email = EmailConfig{
		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     smtpPort,
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		FromAddress:  getEnv("EMAIL_FROM", ""),
		FromName:     getEnv("EMAIL_FROM_NAME", "Kiosk Manager"),
	}

	cfg.Celcom = CelcomConfig{
		APIKey:    getEnv("CELCOM_AFRICA_API_KEY", ""),
		PartnerID: getEnv("CELCOM_AFRICA_PARTNER_ID", ""),
		Shortcode: getEnv("CELCOM_AFRICA_SHORTCODE", ""),
		BaseURL:   getEnv("CELCOM_AFRICA_BASE_URL", "https://sms.celcomafrica.com/api/services/sendsms/"),
	}

	cfg.RateLimit = RateLimitConfig{
		Auth: getEnv("RATE_LIMIT_AUTH", "5-M"),
		API:  getEnv("RATE_LIMIT_API", "100-M"),
	}

	cfg.Log = LogConfig{
		Level:  getEnv("LOG_LEVEL", "info"),
		Format: getEnv("LOG_FORMAT", "json"),
	}

	return cfg, nil
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func (c *DBConfig) URL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		// In test environments, return empty string to avoid panic
		return ""
	}
	return val
}
