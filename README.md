# WhatsApp Automation

A robust Go-based automation tool for sending personalized WhatsApp messages to multiple contacts from a CSV file using browser automation. Features include retry logic, rate limiting, and template-based messaging through WhatsApp Web.

## Features

- **Browser Automation**: Uses chromedp to automate WhatsApp Web
- **CSV-based Contact Management**: Load contacts from a CSV file
- **Template-based Messaging**: Use customizable message templates with variable substitution
- **Retry Logic**: Automatic retry with exponential backoff for failed messages
- **Rate Limiting**: Configurable rate limiting to avoid detection
- **Session Persistence**: Save WhatsApp Web session to avoid repeated QR scans
- **Config-Driven**: All settings managed through a YAML configuration file
- **Comprehensive Logging**: Detailed logs with configurable log levels
- **Dry Run Mode**: Test the automation without sending actual messages
- **Error Reporting**: Detailed summary of successes and failures

## Prerequisites

- Go 1.21 or higher
- Google Chrome or Chromium installed
- WhatsApp account with phone number
- Active internet connection

## Setup

### 1. Install Dependencies

```bash
cd whatsapp-automation
go mod tidy
```

### 2. Build the Application

```bash
go build -o whatsapp-automation
```

### 3. Configure the Application

The configuration file `config.yaml` should already be present. Edit it if needed:

```yaml
browser:
  headless: false              # Set to true to run in background
  user_data_dir: "./chrome-data"  # Session storage directory
  qr_timeout_seconds: 60       # Time to wait for QR code scan
  page_load_timeout: 30        # Timeout for page loads
```

### 4. Prepare Your Data

#### CSV File (`contacts.csv`)

Create a CSV file with the following format:

```csv
name,phone_number
John Doe,+1234567890
Jane Smith,+1987654321
```

**Important**:
- Phone numbers must be in international format with country code (e.g., +1 for US)
- No spaces or special characters except the + prefix

#### Template File (`template.txt`)

Create a message template using Go template syntax:

```
Hello {{.Name}}!

This is a personalized message for you.

Best regards,
The Team
```

Available variables:
- `{{.Name}}`: Contact's name from CSV
- `{{.PhoneNumber}}`: Contact's phone number from CSV

## Usage

### First Run - QR Code Scan

On your first run, you'll need to scan the WhatsApp Web QR code:

```bash
./whatsapp-automation
```

1. The program will open a Chrome browser window
2. WhatsApp Web will load with a QR code
3. Open WhatsApp on your phone
4. Go to Settings > Linked Devices > Link a Device
5. Scan the QR code displayed in the browser
6. Once logged in, the automation will start sending messages

**Note**: After the first login, your session is saved in the `chrome-data` directory, so you won't need to scan the QR code again.

### Subsequent Runs

After the first successful login:

```bash
./whatsapp-automation
```

The browser will open and automatically use your saved session.

### Run with Custom Config

```bash
./whatsapp-automation -config /path/to/config.yaml
```

### Dry Run (Test Without Sending)

```bash
./whatsapp-automation -dry-run
```

This will show you what messages would be sent without actually sending them.

### Headless Mode

To run without showing the browser window, edit `config.yaml`:

```yaml
browser:
  headless: true
```

**Note**: You must complete the QR code scan in non-headless mode first to establish a session.

## Configuration Reference

### `config.yaml` Structure

```yaml
browser:
  headless: false              # Run browser in background (requires existing session)
  user_data_dir: "./chrome-data"  # Directory to store session data
  qr_timeout_seconds: 60       # Time to wait for QR code scan on first run
  page_load_timeout: 30        # Timeout for page loads

files:
  csv_path: "contacts.csv"
  template_path: "template.txt"

retry:
  max_retries: 3                # Number of retry attempts per message
  initial_delay_seconds: 2      # Initial delay before first retry
  max_delay_seconds: 30         # Maximum delay between retries
  backoff_multiplier: 2         # Exponential backoff multiplier

rate_limiting:
  messages_per_second: 1        # Rate limit for sending messages
  enabled: true                 # Enable/disable rate limiting

logging:
  level: "info"                 # debug, info, warn, error
  output_file: "automation.log" # Log file path (empty for stdout only)
```

## Retry Logic

The application implements exponential backoff retry logic:

1. **Initial Delay**: Starts with the configured `initial_delay_seconds`
2. **Exponential Backoff**: Each retry multiplies the delay by `backoff_multiplier`
3. **Max Delay**: Caps the delay at `max_delay_seconds`
4. **Retryable Errors**: Automatically retries on:
   - Page load failures
   - Element not found errors
   - Network timeouts
   - Connection issues

## Rate Limiting

To avoid triggering WhatsApp's spam detection:

- Configure `messages_per_second` (recommended: 1 message/second or slower)
- The application will automatically pace message sending
- Wait times between messages help maintain account safety

## How It Works

1. **Browser Launch**: Opens Chrome/Chromium with your saved session
2. **WhatsApp Web**: Navigates to web.whatsapp.com
3. **Authentication**: Uses saved session or waits for QR code scan
4. **Message Loop**: For each contact:
   - Opens direct chat URL with phone number
   - Waits for chat to load
   - Types the personalized message
   - Clicks send button
   - Applies rate limiting before next message
5. **Error Handling**: Retries failed messages with exponential backoff
6. **Summary**: Reports success/failure statistics

## Error Handling

The application handles errors gracefully:

- **Invalid phone numbers**: Logged and skipped
- **Chat load failures**: Automatically retried
- **Send button not found**: Retried with alternative selectors
- **Network issues**: Retried with exponential backoff
- **Summary report**: Lists all failed contacts at the end

## Logging

Logs include:

- Browser initialization status
- QR code scan status
- Contact processing progress
- Message send confirmations
- Retry attempts
- Final summary with success/failure counts

Log levels:
- `debug`: Verbose output including element selectors and DOM interactions
- `info`: Normal operation information (default)
- `warn`: Warnings and retryable errors
- `error`: Critical errors that stop execution

## Security & Best Practices

1. **Session Security**:
   - The `chrome-data` directory contains your WhatsApp session
   - Never share or commit this directory
   - Restrict permissions: `chmod 700 chrome-data`

2. **Rate Limiting**:
   - Use conservative rate limits (1 message/second or slower)
   - WhatsApp may flag accounts sending too many messages quickly

3. **Account Safety**:
   - Test with small batches first
   - Don't send spam or unsolicited messages
   - Comply with WhatsApp's Terms of Service

4. **Privacy**:
   - Never commit `contacts.csv` with real phone numbers
   - Use `.gitignore` to protect sensitive files

## Troubleshooting

### Browser Won't Open

- Ensure Chrome/Chromium is installed
- Check if Chrome is in your system PATH
- Try running with `headless: false` in config

### QR Code Timeout

- Increase `qr_timeout_seconds` in config
- Ensure your phone has internet connection
- Make sure WhatsApp is updated on your phone

### Messages Not Sending

1. Verify phone numbers are in international format (+countrycode)
2. Check that WhatsApp Web session is still active
3. Review logs for specific error messages
4. Try with `logging.level: "debug"` for more details

### "Element Not Found" Errors

- WhatsApp Web UI changes periodically
- The tool uses multiple selector fallbacks
- Try running with `headless: false` to see what's happening
- Check if WhatsApp Web is showing any popups or notifications

### Session Expired

- Delete the `chrome-data` directory
- Run the application again to scan a new QR code
- Make sure to complete the scan within the timeout period

### Rate Limiting / Account Restrictions

- Reduce `messages_per_second` in config
- Increase delays between messages
- Send smaller batches
- Wait 24 hours before retrying if account is flagged

## Example Output

```
[2025-10-27 22:00:00] Loading configuration from config.yaml
[2025-10-27 22:00:00] WhatsApp Automation started
[2025-10-27 22:00:00] Loading contacts from contacts.csv
[2025-10-27 22:00:00] Loaded 10 contacts
[2025-10-27 22:00:00] Loading message template from template.txt
[2025-10-27 22:00:00] Initializing browser automation...
[2025-10-27 22:00:01] Opening WhatsApp Web...
[2025-10-27 22:00:02] Waiting for WhatsApp Web to load...
[2025-10-27 22:00:05] WhatsApp Web loaded successfully!
[2025-10-27 22:00:08] Processing contact 1/10: Yehouda (+15102168856)
[2025-10-27 22:00:10] Message sent successfully to +15102168856
[2025-10-27 22:00:11] Processing contact 2/10: Yehouda (+15102168856)
[2025-10-27 22:00:13] Message sent successfully to +15102168856
...
[2025-10-27 22:00:30] === Automation Summary ===
[2025-10-27 22:00:30] Total contacts: 10
[2025-10-27 22:00:30] Successful: 10
[2025-10-27 22:00:30] Failed: 0
[2025-10-27 22:00:30] Duration: 22s
[2025-10-27 22:00:30] Closing browser...
[2025-10-27 22:00:30] WhatsApp Automation completed
```

## Command-Line Flags

- `-config <path>`: Specify config file path (default: `config.yaml`)
- `-dry-run`: Test run without sending messages

## Limitations

- Requires Chrome/Chromium browser
- Requires active WhatsApp Web session
- Subject to WhatsApp's rate limits and Terms of Service
- May break if WhatsApp Web UI changes significantly
- Cannot send media files (text only)

## Future Enhancements

Potential improvements:
- Support for media messages (images, documents)
- Support for WhatsApp groups
- Message scheduling
- Progress tracking with resume capability
- Support for message variables from CSV columns

## License

MIT License - Feel free to use and modify as needed.

## Disclaimer

This tool is for educational and authorized use only. Users are responsible for:
- Complying with WhatsApp's Terms of Service
- Obtaining consent from message recipients
- Using the tool ethically and legally
- Any consequences of misuse

The authors are not responsible for any misuse of this tool or violations of WhatsApp's policies.

## Support

For issues or questions:
- Check the Troubleshooting section above
- Review logs with `debug` level enabled
- Ensure all prerequisites are met
- Verify phone number formats in your CSV
