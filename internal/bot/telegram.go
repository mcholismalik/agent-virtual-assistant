package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"virtual-assistant/internal/calendar"
	"virtual-assistant/internal/llm"
)

type TelegramBot struct {
	bot             *tgbotapi.BotAPI
	calendarService *calendar.CalendarService
	claudeService   *llm.ClaudeCodeService
	webhookURL      string
}

func NewTelegramBot(token, webhookURL string, calendarService *calendar.CalendarService, claudeService *llm.ClaudeCodeService) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %v", err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &TelegramBot{
		bot:             bot,
		calendarService: calendarService,
		claudeService:   claudeService,
		webhookURL:      webhookURL,
	}, nil
}

func (tb *TelegramBot) SetWebhook() error {
	webhookConfig, err := tgbotapi.NewWebhook(tb.webhookURL + "/webhook")
	if err != nil {
		return err
	}
	_, err = tb.bot.Request(webhookConfig)
	return err
}

func (tb *TelegramBot) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return
	}

	var update tgbotapi.Update
	err = json.Unmarshal(body, &update)
	if err != nil {
		log.Printf("Error unmarshaling update: %v", err)
		return
	}

	tb.handleUpdate(update)
}

func (tb *TelegramBot) StartPolling() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tb.bot.GetUpdatesChan(u)

	for update := range updates {
		tb.handleUpdate(update)
	}
}

func (tb *TelegramBot) handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	userMessage := update.Message.Text
	chatID := update.Message.Chat.ID
	firstName := update.Message.From.FirstName
	if firstName == "" {
		firstName = update.Message.From.UserName
	}

	// Automatically save chat ID for reminders
	tb.saveChatID(chatID, firstName)

	log.Printf("Received message from %d (%s): %s", chatID, firstName, userMessage)

	response, err := tb.processMessage(userMessage)
	if err != nil {
		log.Printf("Error processing message: %v", err)
		response = "Sorry, I encountered an error processing your request."
	}

	msg := tgbotapi.NewMessage(chatID, response)
	tb.bot.Send(msg)
}

func (tb *TelegramBot) processMessage(userMessage string) (string, error) {
	ctx := context.Background()

	if strings.HasPrefix(strings.ToLower(userMessage), "/start") {
		return "Hello! I'm your virtual assistant. I can help you:\n" +
			"• Create calendar events\n" +
			"• Check today's meetings (/today)\n" +
			"• Send reminders for upcoming meetings\n" +
			"• General chat (/chat <message>)\n\n" +
			"Just tell me what you'd like to do!", nil
	}

	if strings.HasPrefix(strings.ToLower(userMessage), "/today") {
		return tb.getTodayEvents()
	}

	if strings.HasPrefix(strings.ToLower(userMessage), "/chat ") {
		// Extract the message after "/chat "
		chatMessage := strings.TrimSpace(userMessage[6:])
		return tb.handleGeneralChat(ctx, chatMessage)
	}

	claudeResponse, err := tb.claudeService.ProcessCalendarCommand(ctx, userMessage)
	if err != nil {
		return "", fmt.Errorf("failed to get Claude response: %v", err)
	}

	return tb.handleClaudeResponse(claudeResponse)
}

func (tb *TelegramBot) handleClaudeResponse(claudeResponse string) (string, error) {
	lines := strings.Split(claudeResponse, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "ACTION:") {
			action := strings.TrimSpace(strings.TrimPrefix(line, "ACTION:"))
			
			switch action {
			case "CREATE_EVENT":
				return tb.createEventFromResponse(claudeResponse)
			case "CHECK_TODAY":
				return tb.getTodayEvents()
			case "GENERAL":
				return tb.getGeneralResponse(claudeResponse)
			}
		}
	}

	return claudeResponse, nil
}

func (tb *TelegramBot) createEventFromResponse(response string) (string, error) {
	lines := strings.Split(response, "\n")
	var title, description, startTime, endTime, attendeesStr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		} else if strings.HasPrefix(line, "START_TIME:") {
			startTime = strings.TrimSpace(strings.TrimPrefix(line, "START_TIME:"))
		} else if strings.HasPrefix(line, "END_TIME:") {
			endTime = strings.TrimSpace(strings.TrimPrefix(line, "END_TIME:"))
		} else if strings.HasPrefix(line, "ATTENDEES:") {
			attendeesStr = strings.TrimSpace(strings.TrimPrefix(line, "ATTENDEES:"))
		}
	}

	if title == "" || startTime == "" || endTime == "" {
		return "I need more information to create the event. Please provide a title, start time, and end time.", nil
	}

	// Parse attendees
	var attendees []string
	if attendeesStr != "" && attendeesStr != "empty" {
		// Split by comma and clean up emails
		for _, email := range strings.Split(attendeesStr, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				attendees = append(attendees, email)
			}
		}
	}

	// Create event with attendees
	err := tb.calendarService.CreateEventWithAttendees(title, description, startTime, endTime, attendees)
	if err != nil {
		return "", fmt.Errorf("failed to create event: %v", err)
	}

	// Build response message
	responseMsg := fmt.Sprintf("✅ Event created successfully!\n\nTitle: %s\nDescription: %s\nStart: %s\nEnd: %s", 
		title, description, startTime, endTime)
	
	if len(attendees) > 0 {
		responseMsg += fmt.Sprintf("\nAttendees: %s", strings.Join(attendees, ", "))
	}

	return responseMsg, nil
}

func (tb *TelegramBot) getTodayEvents() (string, error) {
	events, err := tb.calendarService.GetTodayEvents()
	if err != nil {
		return "", fmt.Errorf("failed to get today's events: %v", err)
	}

	if len(events) == 0 {
		return "📅 No meetings scheduled for today!", nil
	}

	response := "📅 Today's meetings:\n\n"
	for i, event := range events {
		startTime := ""
		if event.Start.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, event.Start.DateTime); err == nil {
				startTime = t.Format("15:04")
			}
		}
		
		response += fmt.Sprintf("%d. %s", i+1, event.Summary)
		if startTime != "" {
			response += fmt.Sprintf(" at %s", startTime)
		}
		if event.Description != "" {
			response += fmt.Sprintf("\n   📝 %s", event.Description)
		}
		response += "\n\n"
	}

	return response, nil
}

func (tb *TelegramBot) getGeneralResponse(claudeResponse string) (string, error) {
	lines := strings.Split(claudeResponse, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "RESPONSE:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "RESPONSE:")), nil
		}
	}

	return claudeResponse, nil
}

func (tb *TelegramBot) SendReminder(chatID int64, message string) error {
	preview := message
	if len(message) > 50 {
		preview = message[:50] + "..."
	}
	log.Printf("📤 Sending reminder to chat %d: %s", chatID, preview)
	
	msg := tgbotapi.NewMessage(chatID, "🔔 Meeting Reminder:\n"+message)
	
	response, err := tb.bot.Send(msg)
	if err != nil {
		log.Printf("❌ Telegram API error: %v", err)
		return err
	}
	
	log.Printf("✅ Telegram message sent successfully. Message ID: %d", response.MessageID)
	return nil
}

func (tb *TelegramBot) handleGeneralChat(ctx context.Context, message string) (string, error) {
	// Use Claude Code for general conversation
	response, err := tb.claudeService.GeneralChat(ctx, message)
	if err != nil {
		return "", fmt.Errorf("failed to get chat response: %v", err)
	}
	return "💬 " + response, nil
}

// Chat ID storage management
const chatIDsFile = "chat_ids.json"

type ChatIDStore struct {
	ChatIDs map[int64]string `json:"chat_ids"` // chatID -> user first name
}

func (tb *TelegramBot) saveChatID(chatID int64, firstName string) {
	store := tb.loadChatIDs()
	store.ChatIDs[chatID] = firstName
	
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		log.Printf("Error marshaling chat IDs: %v", err)
		return
	}
	
	err = os.WriteFile(chatIDsFile, data, 0644)
	if err != nil {
		log.Printf("Error saving chat IDs: %v", err)
	} else {
		log.Printf("Saved chat ID %d for user %s", chatID, firstName)
	}
}

func (tb *TelegramBot) loadChatIDs() *ChatIDStore {
	store := &ChatIDStore{
		ChatIDs: make(map[int64]string),
	}
	
	data, err := os.ReadFile(chatIDsFile)
	if err != nil {
		// File doesn't exist or can't be read, return empty store
		return store
	}
	
	err = json.Unmarshal(data, store)
	if err != nil {
		log.Printf("Error unmarshaling chat IDs: %v", err)
		return &ChatIDStore{ChatIDs: make(map[int64]string)}
	}
	
	return store
}

func (tb *TelegramBot) GetAllChatIDs() []int64 {
	store := tb.loadChatIDs()
	var chatIDs []int64
	
	for chatID := range store.ChatIDs {
		chatIDs = append(chatIDs, chatID)
	}
	
	return chatIDs
}

func (tb *TelegramBot) GetChatID() int64 {
	// Return the first chat ID for backwards compatibility
	chatIDs := tb.GetAllChatIDs()
	if len(chatIDs) > 0 {
		return chatIDs[0]
	}
	return 0
}