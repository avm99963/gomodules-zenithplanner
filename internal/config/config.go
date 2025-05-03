package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Google GoogleConfig
	App    AppConfig
	DB     DBConfig
	SMTP   SMTPConfig
}

type GoogleConfig struct {
	CalendarID               string
	RefreshToken             string
	WebhookVerificationToken string
	ClientID                 string
	ClientSecret             string
}

type AppConfig struct {
	// Base URL where the server will be available.
	BaseURL string
	// Default location code used for new Calendar events.
	DefaultLocationCode string
	// Number of days into the future where we will create new events.
	FutureHorizonDays int
	// Number of days into the past where we will reconciliate events
	// weekly.
	PastSyncWindowDays int
	// Enable sending email confirmations when an event change is
	// acknowledged.
	EnableEmailConfirmations bool
	// Enable subscribing to Calendar event updates in Google Calendar via
	// the webhook. Even if disabled, the webhook endpoint can be called.
	EnableCalendarSubscription bool
	Scheduler                  SchedulerConfig
}

type SchedulerConfig struct {
	// Enable running the periodic task to create missing events.
	EnableHorizonMaintenance bool
	// Cron string for which to run the horizon maintenance
	//
	// Spec: Minute Hour DayOfMonth Month DayOfWeek
	HorizonMaintenanceCron string
	// Enable running the periodic full sync task.
	EnablePeriodicFullSync bool
	// Cron string for which to run the periodic full sync
	//
	// Spec: Minute Hour DayOfMonth Month DayOfWeek
	PeriodicFullSyncCron string
	// Cron string for which to run the calendar subscription maintenance:
	// the process which renews the subscription if it is close to expire.
	//
	// Spec: Minute Hour DayOfMonth Month DayOfWeek
	CalendarSubscriptionMaintenanceCron string
}

type DBConfig struct {
	ConnectionString string
}

type SMTPConfig struct {
	Host             string
	Port             int
	User             string
	Password         string
	SenderAddress    string
	RecipientAddress string
	SkipTLSVerify    bool
}

// LoadConfig loads configuration from environment variables.
// It explicitly loads a .env file if the path is provided via
// CONFIG_ENV_FILE (this is useful for local development).
func LoadConfig() (*Config, error) {
	envFilePath := os.Getenv("CONFIG_ENV_FILE")
	if envFilePath != "" {
		err := godotenv.Load(envFilePath)
		if err != nil {
			log.Printf("Warning: Could not load .env file from %s: %v", envFilePath, err)
		} else {
			log.Printf("Info: Loaded environment variables from %s", envFilePath)
		}
	}

	horizonDays, err := getIntEnv("FUTURE_HORIZON_DAYS", "90")
	if err != nil {
		return nil, err
	}

	pastSyncDays, err := getIntEnv("PAST_SYNC_WINDOW_DAYS", "30")
	if err != nil {
		return nil, err
	}

	enableEmail, err := getBoolEnv("ENABLE_EMAIL_CONFIRMATIONS", "false")
	if err != nil {
		return nil, err
	}

	enableCalendarSubscription, err := getBoolEnv("ENABLE_CALENDAR_SUBSCRIPTION", "false")
	if err != nil {
		return nil, err
	}

	enableHorizonMaintenance, err := getBoolEnv("ENABLE_HORIZON_MAINTENANCE", "false")
	if err != nil {
		return nil, err
	}

	enablePeriodicFullSync, err := getBoolEnv("ENABLE_PERIODIC_FULL_SYNC", "false")
	if err != nil {
		return nil, err
	}

	smtpPort, err := getIntEnv("SMTP_PORT", "587")
	if err != nil {
		return nil, err
	}

	skipTLSVerify, err := getBoolEnv("SMTP_SKIP_TLS_VERIFY", "false")
	if err != nil {
		return nil, err
	}

	refreshToken, err := getEnvOrErr("GOOGLE_REFRESH_TOKEN")
	if err != nil {
		return nil, err
	}

	webhookVerificationToken, err := getEnvOrErr("WEBHOOK_VERIFICATION_TOKEN")
	if err != nil {
		return nil, err
	}

	appBaseUrl, err := getEnvOrErr("APP_BASE_URL")
	if err != nil {
		return nil, err
	}

	dbConnectionString, err := getEnvOrErr("DB_CONNECTION_STRING")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Google: GoogleConfig{
			CalendarID:               getEnv("GOOGLE_CALENDAR_ID", "primary"),
			RefreshToken:             refreshToken,
			WebhookVerificationToken: webhookVerificationToken,
			ClientID:                 getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret:             getEnv("GOOGLE_CLIENT_SECRET", ""),
		},
		App: AppConfig{
			BaseURL:                    appBaseUrl,
			DefaultLocationCode:        getEnv("DEFAULT_LOCATION_CODE", "HOM"),
			FutureHorizonDays:          horizonDays,
			PastSyncWindowDays:         pastSyncDays,
			EnableEmailConfirmations:   enableEmail,
			EnableCalendarSubscription: enableCalendarSubscription,
			Scheduler: SchedulerConfig{
				EnableHorizonMaintenance:            enableHorizonMaintenance,
				HorizonMaintenanceCron:              getEnv("HORIZON_MAINTENANCE_CRON", "0 2 * * *"),
				EnablePeriodicFullSync:              enablePeriodicFullSync,
				PeriodicFullSyncCron:                getEnv("PERIODIC_FULL_SYNC_CRON", "0 3 * * SUN"),
				CalendarSubscriptionMaintenanceCron: getEnv("CALENDAR_SUBSCRIPTION_MAINTENANCE_CRON", "0 1 * * *"),
			},
		},
		DB: DBConfig{
			ConnectionString: dbConnectionString,
		},
		SMTP: SMTPConfig{
			Host:             getEnv("SMTP_HOST", ""),
			Port:             smtpPort,
			User:             getEnv("SMTP_USER", ""),
			Password:         getEnv("SMTP_PASSWORD", ""),
			SenderAddress:    getEnv("SMTP_SENDER_ADDRESS", ""),
			RecipientAddress: getEnv("RECIPIENT_EMAIL_ADDRESS", ""),
			SkipTLSVerify:    skipTLSVerify,
		},
	}

	// Basic validation for required fields
	if cfg.Google.RefreshToken == "" {
		return nil, fmt.Errorf("missing required environment variable: GOOGLE_REFRESH_TOKEN")
	}
	if cfg.Google.WebhookVerificationToken == "" {
		return nil, fmt.Errorf("missing required environment variable: WEBHOOK_VERIFICATION_TOKEN")
	}
	if cfg.App.BaseURL == "" {
		return nil, fmt.Errorf("missing required environment variable: APP_BASE_URL")
	}
	if cfg.DB.ConnectionString == "" {
		return nil, fmt.Errorf("missing required environment variable: DB_CONNECTION_STRING")
	}
	if cfg.App.EnableEmailConfirmations {
		if cfg.SMTP.Host == "" || cfg.SMTP.SenderAddress == "" || cfg.SMTP.RecipientAddress == "" {
			return nil, fmt.Errorf("missing required SMTP environment variables when ENABLE_EMAIL_CONFIRMATIONS is true")
		}
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvOrErr retrieves an environment variable or returns an error if
// not found.
func getEnvOrErr(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("missing required environment variable: %s", key)
	}
	return value, nil
}

// getIntEnv parses an env as an integer value.
func getIntEnv(key, fallback string) (int, error) {
	rawValue := getEnv(key, fallback)
	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, fmt.Errorf("invalid integer environment variable %s: %w", key, err)
	}
	return value, nil
}

// getBoolEnv parses an env as a boolean value.
func getBoolEnv(key, fallback string) (bool, error) {
	rawValue := getEnv(key, fallback)
	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return false, fmt.Errorf("invalid boolean environment variable %s: %w", key, err)
	}
	return value, nil
}
