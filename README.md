# Agent Virtual Assistant 

A Go-based virtual assistant that integrates with Google Calendar and Telegram, using your local Claude Code subscription for natural language processing. The assistant can create calendar events, check today's meetings, and send automated reminders via Telegram.

## Features

- 🔥 **Langchain Go**: Agent orchestrator with langchain go for simplicity
- 🧠 **Claude Code**: Uses your local Claude Code subscriptions, without API Key (for education / dev purposes)
- 🤖 **Telegram Bot**: Interactive chat interface with natural language processing 
- 📅 **Google Calendar**: Create events and check today's schedule
- 🌐 **Ngrok**: Works with ngrok for local development with webhooks support
- 🔔 **Cron**: Automatic notifications 10 min before meetings with cron


## Prerequisites

- Go 1.21 or higher
- GCP with Calendar API enabled
- Telegram Bot Token
- Claude Code subscription (installed locally)
- Ngrok

## Setup Instructions

### 1. Clone and Setup Project

```bash
git clone <your-repo-url>
cd virtual-assistant
go mod tidy
```

### 2. Prepare All Requirements

#### Google Calendar API Setup
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Calendar API:
   - Go to "APIs & Services" > "Library"
   - Search for "Google Calendar API" and enable it
4. Create credentials:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth 2.0 Client IDs"
   - Choose "Desktop application"
   - Download the JSON file and save it as `credentials.json` in the project root

#### Telegram Bot Setup
1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow the instructions
3. Choose a name and username for your bot
4. Copy the bot token provided by BotFather

#### Claude Code Setup
Since you already have a Claude Code subscription, the system will use your local Claude Code installation:
1. Ensure Claude Code is installed and accessible in your PATH
2. Test by running `claude --version` in your terminal
3. If Claude Code is installed in a different location, note the full path for configuration

#### Ngrok Setup
1. Install ngrok:
```bash
brew install ngrok/ngrok/ngrok
```
2. Start ngrok to expose your local server:
```bash
ngrok http 8080
```
3. Copy the HTTPS URL from ngrok output (e.g., `https://abc123.ngrok.io`)

### 3. Environment Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your prepared values:
```env
# Required
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGhIjKlMnOpQrStUvWxYz
CLAUDE_CODE_PATH=claude
GOOGLE_CREDENTIALS_PATH=credentials.json

# Optional (for webhook mode - use ngrok URL from above)
WEBHOOK_URL=https://abc123.ngrok.io
PORT=8080
```

### 4. Running the Application

#### Polling Mode (Simpler)
```bash
go run cmd/main.go
```

#### Webhook Mode (with ngrok running)
Ensure ngrok is running in another terminal, then:
```bash
go run cmd/main.go
```

### 5. First Run Authorization

1. When you first run the application, it will prompt you to authorize Google Calendar access
2. Open the provided URL in your browser
3. Sign in to Google and authorize the application
4. Copy the authorization code back to the terminal
5. The token will be saved automatically for future use

### 6. Using the Bot

1. Find your bot on Telegram using the username you created
2. Start a conversation with `/start`
3. Try these commands:
   - "Create a meeting tomorrow at 2 PM called 'Team Standup'"
   - "What meetings do I have today?"
   - "/today" - Quick command to check today's schedule
   - "Schedule a call with John next Monday at 10 AM"

## Project Structure

```
virtual-assistant/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── bot/                 # Telegram bot handling
│   │   └── telegram.go
│   ├── calendar/            # Google Calendar integration  
│   │   └── calendar.go
│   ├── config/              # Configuration management
│   │   └── config.go
│   ├── llm/                 # Claude AI integration
│   │   └── claude.go
│   └── reminder/            # Meeting reminder system
│       └── reminder.go
├── pkg/
│   └── utils/               # Utility functions
├── .env.example             # Environment variables template
├── go.mod                   # Go modules file
└── README.md               # This file
```

## How It Works

1. **Message Processing**: User sends message to Telegram bot
2. **AI Analysis**: Claude AI analyzes the message to determine intent
3. **Action Execution**: Based on AI analysis, the bot either:
   - Creates a calendar event
   - Retrieves today's meetings
   - Provides a general response
4. **Response**: Bot sends formatted response back to user
5. **Background Reminders**: Cron job checks for upcoming meetings every 10 minutes

## API Usage Examples

### Creating Events
"Create a meeting called 'Project Review' tomorrow at 3 PM for 1 hour"

### Checking Schedule  
"What meetings do I have today?" or "/today"

### General Queries
"How do I reschedule a meeting?" (Gets AI-powered response)

## Free Tier Limitations

- **Google Calendar API**: 1,000,000 requests/day (free)
- **Telegram Bot API**: Completely free
- **Claude Code**: Uses your existing subscription (no additional API costs)
- **Ngrok**: Free tier allows basic tunneling

## Troubleshooting

### Common Issues

1. **"Failed to create calendar service"**
   - Ensure `credentials.json` is in the correct location
   - Verify Google Calendar API is enabled in your project

2. **"TELEGRAM_BOT_TOKEN is required"**
   - Check your `.env` file exists and contains the bot token
   - Ensure no spaces around the `=` sign in `.env`

3. **Webhook not receiving updates**
   - Verify ngrok is running and URL is correct
   - Check that webhook URL uses HTTPS (ngrok provides this)
   - Ensure port matches between ngrok and your application

4. **"Failed to create Claude Code service"**
   - Ensure Claude Code is installed and accessible in your PATH
   - Try running `claude --version` to verify installation
   - If installed elsewhere, update CLAUDE_CODE_PATH in .env with the full path

### Automatic Reminder Setup

**🎉 No manual setup required!** The bot automatically captures your chat ID when you first interact with it.

**How it works:**
1. **Send any message to your bot** (like `/start`)
2. **Your chat ID is automatically saved** to `chat_ids.json`
3. **You'll receive reminders automatically** for upcoming meetings

**Multiple users supported:** The bot can send reminders to multiple users who have interacted with it.

**Chat ID storage location:** `chat_ids.json` (created automatically)

## Development

### Adding New Features

1. Create new modules in `internal/` directory
2. Update `cmd/main.go` to wire dependencies
3. Test thoroughly before deployment

### Testing

```bash
go test ./...
```

## Security Notes

- Never commit your `.env` file or `credentials.json`
- Keep your API keys secure
- Consider using environment variables in production
- The `token.json` file contains OAuth tokens - keep it secure

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

This project is open source and available under the MIT License.
