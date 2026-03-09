package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Mongo     MongoConfig
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

type MongoConfig struct {
	URI    string
	DBName string
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
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// CelcomConfig holds Celcom Africa SMS API credentials.
type CelcomConfig struct {
	APIKey    string
	PartnerID string
	Shortcode string
	BaseURL   string
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
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.App = AppConfig{
		Env:       getEnv("APP_ENV", "development"),
		Port:      getEnv("APP_PORT", "8080"),
		SecretKey: mustGetEnv("APP_SECRET_KEY"),
		BaseURL:   getEnv("APP_BASE_URL", "http://localhost:8080"),
	}

	cfg.Mongo = MongoConfig{
		URI:    getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		DBName: getEnv("MONGODB_DB_NAME", "kiosk_db"),
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

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("required environment variable not set: " + key)
	}
	return val
}
