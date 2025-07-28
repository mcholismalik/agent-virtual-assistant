package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"virtual-assistant/internal/bot"
	"virtual-assistant/internal/calendar"
	"virtual-assistant/internal/config"
	"virtual-assistant/internal/llm"
	"virtual-assistant/internal/reminder"
)

func main() {
	cfg := config.Load()

	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	if cfg.ClaudeCodePath == "" {
		log.Fatal("CLAUDE_CODE_PATH is required (path to claude executable)")
	}

	calendarService, err := calendar.NewCalendarService(cfg.GoogleCredentialsPath)
	if err != nil {
		log.Fatalf("Failed to create calendar service: %v", err)
	}

	claudeService, err := llm.NewClaudeCodeService(cfg.ClaudeCodePath)
	if err != nil {
		log.Fatalf("Failed to create Claude Code service: %v", err)
	}

	telegramBot, err := bot.NewTelegramBot(cfg.TelegramBotToken, cfg.WebhookURL, calendarService, claudeService)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	reminderService := reminder.NewReminderService(calendarService, telegramBot)

	if cfg.WebhookURL != "" {
		log.Println("Starting webhook mode...")
		
		err = telegramBot.SetWebhook()
		if err != nil {
			log.Fatalf("Failed to set webhook: %v", err)
		}

		http.HandleFunc("/webhook", telegramBot.HandleWebhook)
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		})

		reminderService.Start()

		log.Printf("Server starting on port %s", cfg.Port)
		log.Printf("Webhook URL: %s/webhook", cfg.WebhookURL)
		
		go func() {
			if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
				log.Fatalf("Server failed: %v", err)
			}
		}()
	} else {
		log.Println("Starting polling mode...")
		
		reminderService.Start()
		
		go telegramBot.StartPolling()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("Virtual Assistant is running. Press Ctrl+C to exit.")
	<-c

	log.Println("Shutting down...")
	reminderService.Stop()
}