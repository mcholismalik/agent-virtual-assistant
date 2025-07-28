package reminder

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"virtual-assistant/internal/bot"
	"virtual-assistant/internal/calendar"
)

type ReminderService struct {
	calendarService *calendar.CalendarService
	telegramBot     *bot.TelegramBot
	cron            *cron.Cron
	userChatID      int64
	sentReminders   map[string]bool // Track sent reminders to prevent duplicates
	reminderMutex   sync.RWMutex    // Protect the sentReminders map
}

func NewReminderService(calendarService *calendar.CalendarService, telegramBot *bot.TelegramBot) *ReminderService {
	// Create cron with seconds support
	c := cron.New(cron.WithSeconds())
	return &ReminderService{
		calendarService: calendarService,
		telegramBot:     telegramBot,
		cron:            c,
		userChatID:      0,
		sentReminders:   make(map[string]bool),
	}
}

func (rs *ReminderService) SetUserChatID(chatID int64) {
	rs.userChatID = chatID
}

func (rs *ReminderService) Start() error {
	// Check every 5 seconds instead of 10 minutes
	// Cron format: second minute hour day month weekday
	_, err := rs.cron.AddFunc("*/5 * * * * *", rs.checkUpcomingMeetings)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %v", err)
	}

	rs.cron.Start()
	log.Println("Reminder service started - checking every 5 seconds")
	return nil
}

func (rs *ReminderService) Stop() {
	rs.cron.Stop()
	log.Println("Reminder service stopped")
}

func (rs *ReminderService) checkUpcomingMeetings() {
	// Get all chat IDs from the bot's storage
	chatIDs := rs.telegramBot.GetAllChatIDs()
	if len(chatIDs) == 0 {
		return // Don't spam logs when no users
	}

	// Get events within the next 15 minutes (to catch 10-minute reminders)
	events, err := rs.calendarService.GetUpcomingEvents(15 * time.Minute)
	if err != nil {
		log.Printf("❌ Error getting upcoming events: %v", err)
		return
	}


	now := time.Now()
	tenMinutesFromNow := now.Add(10 * time.Minute)
	
	// Count upcoming and past events
	upcomingCount := 0
	pastCount := 0
	
	// First pass to count events
	for _, event := range events {
		if event.Start.DateTime == "" {
			continue
		}
		eventTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			continue
		}
		
		if eventTime.Before(now) {
			pastCount++
		} else {
			upcomingCount++
		}
	}
	
	// Log the counts if there are any events
	if upcomingCount > 0 || pastCount > 0 {
		log.Printf("🔍 Found %d upcoming events and %d past events", upcomingCount, pastCount)
	}

	for _, event := range events {
		if event.Start.DateTime == "" {
			log.Printf("⚠️ Event '%s' has no start time", event.Summary)
			continue
		}

		eventTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			log.Printf("❌ Error parsing event time for '%s': %v", event.Summary, err)
			continue
		}

		// Create unique reminder key for this event
		reminderKey := fmt.Sprintf("%s_%s", event.Id, eventTime.Format("2006-01-02T15:04"))
		
		// Debug: Log event details
		timeUntilEvent := eventTime.Sub(now)
		// Convert to Indonesia timezone for display
		indonesiaLocation, _ := time.LoadLocation("Asia/Jakarta")
		eventTimeLocal := eventTime.In(indonesiaLocation)
		
		// Check if event is in the past (negative time)
		if eventTime.Before(now) {
			// Only cleanup if exists in memory
			rs.reminderMutex.Lock()
			if _, exists := rs.sentReminders[reminderKey]; exists {
				delete(rs.sentReminders, reminderKey)
				log.Printf("🧹 Cleaned up memory for past event: '%s'", event.Summary)
			}
			rs.reminderMutex.Unlock()
			continue // Skip past events
		}
		
		log.Printf("📅 Event: '%s' in %s (at %s WIB)", event.Summary, formatDuration(timeUntilEvent), eventTimeLocal.Format("15:04"))

		// Send reminder if meeting is between 0-10 minutes away
		if eventTime.Before(tenMinutesFromNow) {
			log.Printf("🎯 Event '%s' is in reminder window (0-10 minutes)!", event.Summary)
			
			// Check if already sent reminder
			rs.reminderMutex.RLock()
			alreadySent := rs.sentReminders[reminderKey]
			rs.reminderMutex.RUnlock()
			
			if alreadySent {
				log.Printf("⏭️ Reminder already sent for '%s' - skipping", event.Summary)
				continue // Skip if already sent
			}

			timeUntil := eventTime.Sub(now)
			message := fmt.Sprintf("🔔 **Meeting Reminder**\n\n📅 **%s**\n\n⏰ Starting in %s\n\n", 
				event.Summary, 
				formatDuration(timeUntil))

			if event.Description != "" {
				message += fmt.Sprintf("📝 %s\n\n", event.Description)
			}

			if event.Location != "" {
				message += fmt.Sprintf("📍 %s\n\n", event.Location)
			}

			// Show attendees if any
			if len(event.Attendees) > 0 {
				var attendeeNames []string
				for _, attendee := range event.Attendees {
					if attendee.Email != "" {
						attendeeNames = append(attendeeNames, attendee.Email)
					}
				}
				if len(attendeeNames) > 0 {
					message += fmt.Sprintf("👥 Attendees: %s\n\n", strings.Join(attendeeNames, ", "))
				}
			}

			message += fmt.Sprintf("🕐 %s", eventTime.Format("15:04 MST"))

			// Send reminder to all active users
			for _, chatID := range chatIDs {
				log.Printf("🚀 Attempting to send reminder for '%s' to chat %d", event.Summary, chatID)
				err = rs.telegramBot.SendReminder(chatID, message)
				if err != nil {
					log.Printf("❌ FAILED to send reminder to chat %d: %v", chatID, err)
				} else {
					log.Printf("✅ SUCCESS: Sent reminder for '%s' to chat %d", event.Summary, chatID)
				}
			}

			// Mark as sent to prevent duplicates
			rs.reminderMutex.Lock()
			rs.sentReminders[reminderKey] = true
			log.Printf("💾 Saved reminder flag for '%s' in memory", event.Summary)
			rs.reminderMutex.Unlock()
		}
	}
	
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d hour", hours)
	}
	return fmt.Sprintf("%d hour %d minutes", hours, minutes)
}

func (rs *ReminderService) cleanupOldReminders() {
	rs.reminderMutex.Lock()
	defer rs.reminderMutex.Unlock()
	
	// Clean up reminder keys older than 2 hours
	cutoff := time.Now().Add(-2 * time.Hour)
	cleanedCount := 0
	for key := range rs.sentReminders {
		// Extract timestamp from key (format: eventId_2006-01-02T15:04)
		parts := strings.Split(key, "_")
		if len(parts) >= 2 {
			timeStr := parts[len(parts)-1]
			if eventTime, err := time.Parse("2006-01-02T15:04", timeStr); err == nil {
				if eventTime.Before(cutoff) {
					delete(rs.sentReminders, key)
					cleanedCount++
				}
			}
		}
	}
	if cleanedCount > 0 {
		log.Printf("🧹 Cleaned up %d old reminder entries from memory", cleanedCount)
	}
}


func (rs *ReminderService) SetChatIDFromEnv(chatIDStr string) error {
	if chatIDStr == "" {
		return fmt.Errorf("chat ID not provided")
	}
	
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID format: %v", err)
	}
	
	rs.SetUserChatID(chatID)
	return nil
}