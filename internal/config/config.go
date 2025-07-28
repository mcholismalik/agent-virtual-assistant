package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken     string
	GoogleCredentialsPath string
	ClaudeCodePath       string
	WebhookURL           string
	Port                 string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		TelegramBotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
		GoogleCredentialsPath: getEnv("GOOGLE_CREDENTIALS_PATH", "credentials.json"),
		ClaudeCodePath:       getEnv("CLAUDE_CODE_PATH", "claude"),
		WebhookURL:           getEnv("WEBHOOK_URL", ""),
		Port:                 getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}