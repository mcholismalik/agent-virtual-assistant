package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarService struct {
	service *calendar.Service
}

func NewCalendarService(credentialsPath string) (*CalendarService, error) {
	ctx := context.Background()
	
	b, err := ioutil.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	
	client := getClient(config)
	
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %v", err)
	}

	return &CalendarService{service: srv}, nil
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	// Start a local HTTP server to handle the callback
	codeCh := make(chan string)
	errCh := make(chan error)
	
	server := &http.Server{Addr: ":8000"}
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		
		fmt.Fprintf(w, `
			<html>
			<head><title>Authorization Successful</title></head>
			<body>
				<h2>✅ Authorization successful!</h2>
				<p>You can close this window and return to your terminal.</p>
			</body>
			</html>
		`)
		codeCh <- code
	})
	
	go func() {
		log.Println("Starting OAuth callback server on :8000")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	
	// Update config to use localhost:8000
	config.RedirectURL = "http://localhost:8000"
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("🔗 Open this link in your browser to authorize the application:\n%v\n\n", authURL)
	fmt.Println("⏳ Waiting for authorization... (will timeout in 5 minutes)")
	
	var authCode string
	select {
	case authCode = <-codeCh:
		fmt.Println("✅ Authorization received successfully!")
	case err := <-errCh:
		log.Fatalf("❌ Error during authorization: %v", err)
	case <-time.After(5 * time.Minute):
		log.Fatalf("❌ Authorization timed out after 5 minutes")
	}
	
	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (cs *CalendarService) CreateEvent(title, description, startTime, endTime string) error {
	return cs.CreateEventWithAttendees(title, description, startTime, endTime, nil)
}

func (cs *CalendarService) CreateEventWithAttendees(title, description, startTime, endTime string, attendeeEmails []string) error {
	event := &calendar.Event{
		Summary:     title,
		Description: description,
		Start: &calendar.EventDateTime{
			DateTime: startTime,
			TimeZone: "Asia/Jakarta", // Indonesia timezone
		},
		End: &calendar.EventDateTime{
			DateTime: endTime,
			TimeZone: "Asia/Jakarta", // Indonesia timezone
		},
	}
	
	// Add attendees if provided
	if len(attendeeEmails) > 0 {
		var attendees []*calendar.EventAttendee
		for _, email := range attendeeEmails {
			attendees = append(attendees, &calendar.EventAttendee{
				Email: email,
			})
		}
		event.Attendees = attendees
	}

	_, err := cs.service.Events.Insert("primary", event).Do()
	return err
}

func (cs *CalendarService) GetTodayEvents() ([]*calendar.Event, error) {
	// Use Indonesia timezone
	indonesiaLocation, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(indonesiaLocation)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, indonesiaLocation)
	endOfDay := startOfDay.Add(24 * time.Hour)

	events, err := cs.service.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(startOfDay.Format(time.RFC3339)).
		TimeMax(endOfDay.Format(time.RFC3339)).
		OrderBy("startTime").Do()
	
	if err != nil {
		return nil, err
	}

	return events.Items, nil
}

func (cs *CalendarService) GetUpcomingEvents(duration time.Duration) ([]*calendar.Event, error) {
	now := time.Now()
	later := now.Add(duration)

	events, err := cs.service.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(later.Format(time.RFC3339)).
		OrderBy("startTime").Do()
	
	if err != nil {
		return nil, err
	}

	return events.Items, nil
}