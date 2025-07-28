package llm

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ClaudeCodeService struct {
	claudeCodePath string
}

func NewClaudeCodeService(claudeCodePath string) (*ClaudeCodeService, error) {
	if claudeCodePath == "" {
		claudeCodePath = "claude"
	}

	if _, err := exec.LookPath(claudeCodePath); err != nil {
		return nil, fmt.Errorf("claude code not found in PATH. Please ensure Claude Code is installed and accessible. Error: %v", err)
	}

	return &ClaudeCodeService{claudeCodePath: claudeCodePath}, nil
}

func (ccs *ClaudeCodeService) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	// Use --print flag for non-interactive output and pass prompt directly
	cmd := exec.CommandContext(ctx, ccs.claudeCodePath, "--print", prompt)
	
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude code execution failed: %v, stderr: %s", err, string(exitError.Stderr))
		}
		return "", fmt.Errorf("failed to execute claude code: %v", err)
	}

	response := strings.TrimSpace(string(output))
	if response == "" {
		return "I apologize, but I couldn't generate a response at the moment.", nil
	}

	return response, nil
}

func (ccs *ClaudeCodeService) GenerateResponseInteractive(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, ccs.claudeCodePath)
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude code: %v", err)
	}

	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, prompt)
		fmt.Fprintln(stdin, "")
	}()

	var response strings.Builder
	scanner := bufio.NewScanner(stdout)
	
	timeout := time.After(30 * time.Second)
	done := make(chan bool)
	
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Claude:") || strings.Contains(line, "Assistant:") {
				continue
			}
			if line != "" {
				response.WriteString(line + "\n")
			}
		}
		done <- true
	}()

	select {
	case <-done:
		break
	case <-timeout:
		cmd.Process.Kill()
		return "", fmt.Errorf("claude code response timeout")
	case <-ctx.Done():
		cmd.Process.Kill()
		return "", ctx.Err()
	}

	cmd.Wait()

	result := strings.TrimSpace(response.String())
	if result == "" {
		return "I apologize, but I couldn't generate a response at the moment.", nil
	}

	return result, nil
}

func (ccs *ClaudeCodeService) ProcessCalendarCommand(ctx context.Context, userMessage string) (string, error) {
	// Get current time in Indonesia timezone
	indonesiaLocation, _ := time.LoadLocation("Asia/Jakarta")
	currentTime := time.Now().In(indonesiaLocation)
	currentDateStr := currentTime.Format("2006-01-02")
	
	prompt := fmt.Sprintf(`You are a helpful virtual assistant for managing Google Calendar events and meetings. 
The user said: "%s"

IMPORTANT CONTEXT:
- Current date and time in Indonesia (Asia/Jakarta timezone): %s
- Today's date is: %s
- Use Indonesia timezone (+07:00) for all times
- When user says "today", use today's date: %s
- When user says "tomorrow", use: %s

Please analyze this message and determine what the user wants to do:
1. Create a calendar event - extract title, description, date/time, attendees
2. Check today's meetings - list today's schedule
3. General query - provide helpful response

Respond in a structured way that clearly indicates the action needed and any extracted information.
If creating an event, provide the details in this format:
ACTION: CREATE_EVENT
TITLE: [event title]
DESCRIPTION: [event description]  
START_TIME: [ISO format date-time like %sT14:00:00+07:00 for Indonesia timezone]
END_TIME: [ISO format date-time like %sT15:00:00+07:00 for Indonesia timezone]
ATTENDEES: [comma-separated email addresses if mentioned, or empty if none]

If checking meetings:
ACTION: CHECK_TODAY

For general queries:
ACTION: GENERAL
RESPONSE: [your helpful response]

Be concise and format the response exactly as shown above.`, 
		userMessage, 
		currentTime.Format("2006-01-02 15:04:05 MST"), 
		currentDateStr,
		currentDateStr,
		currentTime.AddDate(0, 0, 1).Format("2006-01-02"),
		currentDateStr,
		currentDateStr)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return ccs.GenerateResponse(ctx, prompt)
}

func (ccs *ClaudeCodeService) GeneralChat(ctx context.Context, userMessage string) (string, error) {
	prompt := fmt.Sprintf(`You are a helpful AI assistant. The user is chatting with you directly.

User message: "%s"

Please provide a helpful, conversational response. Keep it friendly and concise.`, userMessage)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return ccs.GenerateResponse(ctx, prompt)
}