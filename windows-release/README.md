# WhatsApp Automation for Windows

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

- Windows 10 or higher
- Google Chrome installed (download from https://www.google.com/chrome/)
- WhatsApp account with phone number
- Active internet connection

## Quick Start Guide

### 1. Install Go

1. Download Go for Windows from: https://go.dev/dl/
2. Run the installer (e.g., `go1.25.3.windows-amd64.msi`)
3. Follow the installation wizard
4. Verify installation by opening Command Prompt and typing:
   ```cmd
   go version
   ```

### 2. Extract and Setup

1. Extract this ZIP file to a folder (e.g., `C:\whatsapp-automation`)
2. Open Command Prompt (cmd.exe)
3. Navigate to the folder:
   ```cmd
   cd C:\whatsapp-automation
   ```

### 3. Run the Setup Script

```cmd
setup.bat
```

This will:
- Install Go dependencies
- Build the application
- Create example configuration files

### 4. Configure Your Settings

1. Copy `config.example.yaml` to `config.yaml`:
   ```cmd
   copy config.example.yaml config.yaml
   ```

2. Edit `config.yaml` with Notepad if needed:
   ```cmd
   notepad config.yaml
   ```

### 5. Prepare Your Contacts

1. Copy `contacts.example.csv` to `contacts.csv`:
   ```cmd
   copy contacts.example.csv contacts.csv
   ```

2. Edit `contacts.csv` with your actual contacts:
   ```cmd
   notepad contacts.csv
   ```

   Format:
   ```csv
   name,phone_number
   John Doe,+1234567890
   Jane Smith,+1987654321
   ```

   **Important**: Phone numbers MUST include country code with + prefix (e.g., +1 for US)

### 6. Customize Your Message Template

Edit `template.txt` to customize your message:
```cmd
notepad template.txt
```

You can use these variables:
- `{{.Name}}` - Contact's name from CSV
- `{{.PhoneNumber}}` - Contact's phone number

Example:
```
Hello {{.Name}}!

This is a personalized message for you.

Best regards,
The Team
```

### 7. Run the Application

**First Run - QR Code Scan:**
```cmd
run.bat
```

This will:
1. Open Google Chrome with WhatsApp Web
2. Show a QR code
3. Scan the QR code with your phone:
   - Open WhatsApp on your phone
   - Tap the three dots (⋮) or Settings
   - Go to "Linked Devices"
   - Tap "Link a Device"
   - Scan the QR code in the browser

After the first successful login, your session is saved and you won't need to scan again.

**Test Run (Dry Run):**

To test without sending messages:
```cmd
run-dryrun.bat
```

## Windows-Specific Scripts

This package includes several batch files for Windows:

- **setup.bat** - First-time setup (install dependencies and build)
- **run.bat** - Run the application normally
- **run-dryrun.bat** - Test run without sending messages
- **build.bat** - Rebuild the application
- **clean.bat** - Remove session data and logs

## Configuration Reference

### config.yaml

```yaml
browser:
  headless: false              # Set to true to hide browser window
  user_data_dir: "./chrome-data"  # Session storage folder
  qr_timeout_seconds: 60       # Time to wait for QR scan
  page_load_timeout: 30        # Page load timeout

files:
  csv_path: "contacts.csv"
  template_path: "template.txt"
  completed_csv_path: "completed.csv"

retry:
  max_retries: 3                # Retry attempts per message
  initial_delay_seconds: 2      # Initial retry delay
  max_delay_seconds: 30         # Max retry delay
  backoff_multiplier: 2         # Exponential backoff

rate_limiting:
  messages_per_second: 1        # Messages per second (recommended: 1)
  enabled: true                 # Enable rate limiting

logging:
  level: "info"                 # debug, info, warn, error
  output_file: "automation.log" # Log file path
```

## Finding Chrome Path on Windows

If the application can't find Chrome automatically, you can specify the path:

Common Chrome locations:
- `C:\Program Files\Google\Chrome\Application\chrome.exe`
- `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`
- `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`

## Troubleshooting Windows Issues

### Chrome Not Found

If you get "Chrome not found" error:
1. Make sure Chrome is installed
2. Try installing from: https://www.google.com/chrome/
3. Restart Command Prompt after installing

### Permission Errors

If you get permission errors:
1. Run Command Prompt as Administrator:
   - Right-click on Command Prompt
   - Select "Run as administrator"
2. Navigate to the folder and run setup again

### Antivirus Blocking

If your antivirus blocks the application:
1. The .exe file is safe (it's compiled from the Go source code)
2. Add an exception in your antivirus for the folder
3. Windows Defender may show a warning - click "More info" then "Run anyway"

### Browser Window Not Showing

If headless mode is enabled:
1. Open `config.yaml`
2. Set `headless: false`
3. Save and run again

### QR Code Timeout

If you can't scan the QR code in time:
1. Open `config.yaml`
2. Increase `qr_timeout_seconds: 120` (2 minutes)
3. Save and try again

### Session Expired

If you need to scan QR code again:
```cmd
clean.bat
run.bat
```

## Running in Background (Headless Mode)

After your first successful QR scan:

1. Edit `config.yaml`:
   ```yaml
   browser:
     headless: true
   ```

2. Run normally:
   ```cmd
   run.bat
   ```

The browser will run in the background.

## Safety & Best Practices

1. **Start Small**: Test with 2-3 contacts first
2. **Rate Limiting**: Keep `messages_per_second: 1` or slower
3. **Session Security**: Don't share the `chrome-data` folder
4. **No Spam**: Only send messages to people who expect them
5. **WhatsApp Terms**: Comply with WhatsApp's Terms of Service

## Command-Line Options

Run the application with custom options:

```cmd
whatsapp-automation.exe -config myconfig.yaml
whatsapp-automation.exe -dry-run
```

Available flags:
- `-config <path>` - Use custom config file
- `-dry-run` - Test without sending

## Logs and Tracking

- **automation.log** - Detailed activity log
- **completed.csv** - Tracks sent messages (prevents duplicates)

To view logs:
```cmd
notepad automation.log
```

To clear logs:
```cmd
clean.bat
```

## Folder Structure

```
whatsapp-automation/
├── whatsapp-automation.exe   (Built application)
├── setup.bat                 (Setup script)
├── run.bat                   (Run script)
├── run-dryrun.bat           (Test script)
├── build.bat                 (Build script)
├── clean.bat                 (Clean script)
├── config.yaml              (Your configuration)
├── config.example.yaml      (Example configuration)
├── contacts.csv             (Your contacts)
├── contacts.example.csv     (Example contacts)
├── template.txt             (Message template)
├── completed.csv            (Tracking file - auto-generated)
├── automation.log           (Log file - auto-generated)
├── chrome-data/             (Session data - auto-generated)
├── README-WINDOWS.md        (This file)
└── Source files (.go)       (Source code)
```

## Example Session

```
C:\whatsapp-automation> run.bat
Loading configuration from config.yaml
WhatsApp Automation started
Loading contacts from contacts.csv
Loaded 5 contacts
Loading message template from template.txt
Initializing browser automation...
Opening WhatsApp Web...
Waiting for WhatsApp Web to load...
Please scan the QR code...
WhatsApp Web loaded successfully!
Processing contact 1/5: John Doe (+1234567890)
Successfully sent message to John Doe
Processing contact 2/5: Jane Smith (+1987654321)
Successfully sent message to Jane Smith
...
=== Automation Summary ===
Total contacts: 5
Successful: 5
Failed: 0
Duration: 45s
WhatsApp Automation completed
```

## Security Notes

1. **Session Data**: The `chrome-data` folder contains your WhatsApp login session
   - Keep this folder private
   - Don't share or upload it anywhere
   - Delete it if you want to log out

2. **CSV Files**: Don't commit `contacts.csv` to version control if it has real phone numbers

3. **Firewall**: Windows Firewall may ask for permission - click "Allow access"

## Updating

To update to a new version:
1. Extract the new ZIP to a new folder
2. Copy your `config.yaml`, `contacts.csv`, and `template.txt` to the new folder
3. Optionally copy `chrome-data` folder to avoid re-scanning QR code
4. Run `setup.bat` in the new folder

## Limitations

- Text messages only (no images/documents)
- Requires Chrome browser
- Requires WhatsApp Web session
- Subject to WhatsApp's rate limits
- May break if WhatsApp Web UI changes

## Getting Help

1. Check this README thoroughly
2. Review `automation.log` for error details
3. Try running with `-dry-run` to test
4. Enable debug logging in `config.yaml`:
   ```yaml
   logging:
     level: "debug"
   ```

## License

MIT License - Free to use and modify

## Disclaimer

This tool is for educational and authorized use only. You are responsible for:
- Complying with WhatsApp's Terms of Service
- Obtaining consent from recipients
- Using the tool ethically and legally
- Any consequences of misuse

The authors are not responsible for misuse or policy violations.

## Windows Tips

- **Path Spaces**: If your folder path has spaces, use quotes:
  ```cmd
  cd "C:\My Documents\whatsapp-automation"
  ```

- **Multiple Windows**: You can run multiple instances from different folders with different configs

- **Task Scheduler**: You can schedule runs using Windows Task Scheduler

- **Network Drives**: Don't run from network drives - use local folders

## Support

For technical issues:
1. Check Windows Event Viewer for system errors
2. Ensure Chrome is up to date
3. Verify phone number format (+countrycode)
4. Try running as Administrator
5. Check antivirus isn't blocking

Happy Automating! 🚀
