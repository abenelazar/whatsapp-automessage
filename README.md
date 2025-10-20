# WhatsApp Auto Message

This project automates sending personalised WhatsApp messages from a CSV contact list using Chrome browser automation via [chromedp](https://github.com/chromedp/chromedp). It supports macOS and Windows and keeps the WhatsApp Web session active between runs so you do not need to scan the QR code every time.

## Project Structure

```
.
├── completed.csv        # Created automatically to track which contacts were messaged
├── contacts.csv         # Your contact list (not committed)
├── go.mod               # Go module definition
├── main.go              # Program entry point
├── message.txt          # Message template (Go template syntax)
└── README.md            # Project documentation
```

## Prerequisites

* [Go 1.21+](https://go.dev/dl/)
* Google Chrome installed (the system Chrome executable is reused by chromedp)

> **Note:** On macOS the first run will launch a visible Chrome window. On Windows a console window opens alongside Chrome. Do not close Chrome while the automation is running.

## Setup

1. Clone the repository and navigate into the project directory.
2. Install dependencies:

   ```bash
   go mod tidy
   ```

3. Prepare your data files:
   * **contacts.csv** – The first row must contain headers (for example: `name,number,coupon_code,support_email`). Every subsequent row holds the values for a contact. Phone numbers should include the country code without `+` or leading zeros.
   * **message.txt** – Uses Go template syntax. Placeholders are referenced by the header names from `contacts.csv`, for example `Hello {{.name}}`.

   A sample template is provided in this repository (`message.txt`).

## Running the Automation

Run the tool with the default settings:

```bash
go run .
```

The program accepts several optional flags:

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `-contacts` | Path to the contacts CSV file | `contacts.csv` |
| `-message` | Path to the message template | `message.txt` |
| `-completed` | Path to the completion tracker CSV | `completed.csv` |
| `-session` | Directory used for Chrome's persistent user data | `whatsapp-session` |
| `-min-delay` | Minimum delay between messages | `3s` |
| `-max-delay` | Maximum delay between messages | `7s` |
| `-headless` | Run Chrome in headless mode | `false` |

Example with custom files and delays:

```bash
go run . -contacts ./data/contacts.csv -message ./templates/offer.txt -min-delay 5s -max-delay 12s
```

## How It Works

1. Contacts are loaded from the CSV file, using the header row as field names.
2. Completed contacts are read from `completed.csv` (created automatically) so that repeat runs skip already processed numbers.
3. The message template is rendered for each contact using Go's `text/template` package.
4. Chrome is launched with a persistent user data directory (`whatsapp-session`), which stores your WhatsApp Web session.
5. For each contact the program navigates to `https://web.whatsapp.com/send?phone=<number>` and waits for the chat to load or for an “invalid number” dialog.
6. Messages are typed directly into the chat input and sent with the Enter key. After each successful send the contact is appended to `completed.csv`.
7. Randomised delays between each message mimic human behaviour.

If WhatsApp reports that a number is invalid, the program dismisses the pop-up, logs the issue, and continues with the next contact.

## Troubleshooting

* Ensure Chrome is not already running with the same user data directory when you start the program.
* If the automation appears to stall, press `Ctrl+C` (Windows/Linux) or `Cmd+C` (macOS) in the terminal to stop it. You can safely restart; completed contacts remain logged.
* For very large contact lists consider increasing the delays to avoid triggering WhatsApp rate limits.

## Disclaimer

Use this tool responsibly and comply with WhatsApp's terms of service. Excessive automated messaging may lead to account restrictions.
