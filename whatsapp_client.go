package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type WhatsAppClient struct {
	config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	rateLimiter <-chan time.Time
}

func NewWhatsAppClient(config *Config) *WhatsAppClient {
	client := &WhatsAppClient{
		config: config,
	}

	// Setup rate limiter if enabled
	if config.RateLimiting.Enabled && config.RateLimiting.MessagesPerSecond > 0 {
		interval := time.Second / time.Duration(config.RateLimiting.MessagesPerSecond)
		client.rateLimiter = time.Tick(interval)
	}

	return client
}

func (c *WhatsAppClient) Initialize() error {
	Log("info", "Initializing browser automation...")

	// Ensure user data directory exists with proper permissions
	if err := ensureUserDataDir(c.config.Browser.UserDataDir); err != nil {
		return fmt.Errorf("failed to create user data directory: %w", err)
	}

	// Setup Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", c.config.Browser.Headless),
		chromedp.UserDataDir(c.config.Browser.UserDataDir),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("disable-extensions", false),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.WindowSize(1200, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	c.allocCancel = allocCancel
	c.ctx, c.cancel = chromedp.NewContext(allocCtx)

	// Navigate to WhatsApp Web
	Log("info", "Opening WhatsApp Web...")
	err := chromedp.Run(c.ctx,
		chromedp.Navigate("https://web.whatsapp.com"),
	)
	if err != nil {
		return fmt.Errorf("failed to navigate to WhatsApp Web: %w", err)
	}

	// Wait for login (either QR code scan or existing session)
	Log("info", "Waiting for WhatsApp Web to load...")

	// Check if already logged in or wait for QR scan
	timeoutCtx, timeoutCancel := context.WithTimeout(c.ctx, time.Duration(c.config.Browser.QRTimeoutSeconds)*time.Second)
	defer timeoutCancel()

	err = chromedp.Run(timeoutCtx,
		chromedp.WaitVisible(`//div[@id='side']`, chromedp.BySearch),
	)

	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("timeout waiting for WhatsApp Web login. Please scan the QR code within %d seconds", c.config.Browser.QRTimeoutSeconds)
		}
		return fmt.Errorf("failed to load WhatsApp Web: %w", err)
	}

	Log("info", "WhatsApp Web loaded successfully!")

	// Wait a bit for the page to fully stabilize
	time.Sleep(3 * time.Second)

	return nil
}

func (c *WhatsAppClient) Close() {
	if c.cancel != nil {
		Log("info", "Closing browser...")
		c.cancel()
	}
	if c.allocCancel != nil {
		c.allocCancel()
	}
}

func (c *WhatsAppClient) SendMessage(phoneNumber, message string) error {
	// Apply rate limiting
	if c.rateLimiter != nil {
		<-c.rateLimiter
	}

	var lastErr error
	retryDelay := time.Duration(c.config.Retry.InitialDelaySeconds) * time.Second
	maxDelay := time.Duration(c.config.Retry.MaxDelaySeconds) * time.Second

	for attempt := 0; attempt <= c.config.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			Log("info", fmt.Sprintf("Retry attempt %d/%d for %s after %v",
				attempt, c.config.Retry.MaxRetries, phoneNumber, retryDelay))
			time.Sleep(retryDelay)

			// Exponential backoff
			retryDelay = time.Duration(float64(retryDelay) * c.config.Retry.BackoffMultiplier)
			if retryDelay > maxDelay {
				retryDelay = maxDelay
			}
		}

		err := c.sendMessageAttempt(phoneNumber, message)
		if err == nil {
			return nil // Success
		}

		lastErr = err
		Log("warn", fmt.Sprintf("Failed to send message to %s: %v", phoneNumber, err))
	}

	return fmt.Errorf("failed after %d retries: %w", c.config.Retry.MaxRetries, lastErr)
}

func (c *WhatsAppClient) sendMessageAttempt(phoneNumber, message string) error {
	// Clean phone number (remove + and spaces)
	cleanNumber := strings.ReplaceAll(strings.ReplaceAll(phoneNumber, "+", ""), " ", "")

	// Use WhatsApp Web direct URL to open chat
	chatURL := fmt.Sprintf("https://web.whatsapp.com/send?phone=%s", cleanNumber)

	Log("debug", fmt.Sprintf("Opening chat for %s", phoneNumber))

	// Disable beforeunload event to prevent "Leave site?" dialog
	err := chromedp.Run(c.ctx,
		chromedp.Evaluate(`window.onbeforeunload = null;`, nil),
	)
	if err != nil {
		Log("warn", fmt.Sprintf("Failed to disable beforeunload: %v", err))
	}

	// Navigate to chat URL
	err = chromedp.Run(c.ctx,
		chromedp.Navigate(chatURL),
		chromedp.Sleep(3*time.Second), // Wait for navigation
	)
	if err != nil {
		return fmt.Errorf("failed to navigate to chat: %w", err)
	}

	// Wait for the message input box to be visible
	Log("debug", "Waiting for message input box...")

	// Try different possible selectors for the message input box
	inputSelectors := []string{
		`//div[@contenteditable='true'][@data-tab='10']`,
		`//div[@contenteditable='true'][@role='textbox'][@title='Type a message']`,
		`//div[@contenteditable='true'][@data-lexical-editor='true']`,
		`//div[contains(@class, 'copyable-text')]//div[@contenteditable='true']`,
	}

	var inputFound bool
	var usedSelector string

	for _, selector := range inputSelectors {
		err = chromedp.Run(c.ctx,
			chromedp.WaitVisible(selector, chromedp.BySearch),
		)
		if err == nil {
			inputFound = true
			usedSelector = selector
			Log("debug", fmt.Sprintf("Found message input using selector: %s", selector))
			break
		}
	}

	if !inputFound {
		// Check if number is invalid
		var invalidText string
		chromedp.Run(c.ctx,
			chromedp.Text(`//div[contains(text(), 'Phone number')]`, &invalidText, chromedp.BySearch),
		)
		if invalidText != "" {
			return fmt.Errorf("invalid phone number: %s", phoneNumber)
		}

		return fmt.Errorf("could not find message input box (chat may not have loaded)")
	}

	Log("debug", "Typing message...")

	// Click the input box to focus it
	err = chromedp.Run(c.ctx,
		chromedp.Click(usedSelector, chromedp.BySearch),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("failed to click message input: %w", err)
	}

	// Clear any existing text by selecting all and deleting
	// Use Cmd+A on Mac, Ctrl+A on other systems
	err = chromedp.Run(c.ctx,
		chromedp.KeyEvent("a", chromedp.KeyModifiers(2)), // 2 = Cmd/Ctrl modifier
		chromedp.Sleep(100*time.Millisecond),
		chromedp.KeyEvent("\b"),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		Log("warn", fmt.Sprintf("Failed to clear existing text: %v", err))
	}

	// Type the message with proper newline handling
	// In WhatsApp Web, Enter sends the message, so we need to use Shift+Enter for newlines
	lines := strings.Split(message, "\n")

	for i, line := range lines {
		if i > 0 {
			// Send Shift+Enter for newline (not just Enter which would send the message)
			err = chromedp.Run(c.ctx,
				chromedp.KeyEvent("\n", chromedp.KeyModifiers(1)), // 1 = Shift modifier
				chromedp.Sleep(50*time.Millisecond),
			)
			if err != nil {
				return fmt.Errorf("failed to insert newline: %w", err)
			}
		}

		if line != "" {
			err = chromedp.Run(c.ctx,
				chromedp.SendKeys(usedSelector, line, chromedp.BySearch),
				chromedp.Sleep(100*time.Millisecond),
			)
			if err != nil {
				return fmt.Errorf("failed to type message line: %w", err)
			}
		}
	}

	// Wait to ensure message is fully typed
	time.Sleep(1 * time.Second)

	// Send the message by pressing Enter (without Shift modifier)
	Log("debug", "Sending message with Enter key...")
	err = chromedp.Run(c.ctx,
		chromedp.KeyEvent("\r"), // Enter key to send
	)
	if err != nil {
		return fmt.Errorf("failed to send message with Enter key: %w", err)
	}

	// Wait a bit for the message to start sending
	time.Sleep(2 * time.Second)

	// Wait for message to be sent by looking for the sent indicator (checkmark)
	Log("debug", "Waiting for message to be sent...")

	// WhatsApp shows a checkmark icon when message is sent
	// Look for the most recent message bubble with a checkmark
	sendCheckSelectors := []string{
		`(//span[@data-icon='msg-check'])[last()]`,     // Single checkmark (sent) - last one
		`(//span[@data-icon='msg-dblcheck'])[last()]`,  // Double checkmark (delivered) - last one
		`(//span[@data-icon='msg-dblcheck-ack'])[last()]`, // Blue double checkmark (read) - last one
	}

	messageSent := false
	maxWaitTime := 15 * time.Second
	checkInterval := 1 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime && !messageSent {
		for _, selector := range sendCheckSelectors {
			// Try to find the checkmark - use a very short timeout
			checkCtx, checkCancel := context.WithTimeout(c.ctx, 100*time.Millisecond)
			err = chromedp.Run(checkCtx,
				chromedp.WaitVisible(selector, chromedp.BySearch),
			)
			checkCancel()

			if err == nil {
				messageSent = true
				Log("debug", fmt.Sprintf("Message sent confirmation found with selector: %s", selector))
				break
			}
		}
		if !messageSent {
			Log("debug", fmt.Sprintf("Checking for checkmark... (%v elapsed)", time.Since(startTime).Round(time.Second)))
			time.Sleep(checkInterval)
		}
	}

	if !messageSent {
		Log("warn", fmt.Sprintf("Could not confirm message was sent to %s (no checkmark found after %v)", phoneNumber, maxWaitTime))
		return fmt.Errorf("message send confirmation timeout")
	}

	// Wait an additional moment to ensure message is fully sent before navigating away
	time.Sleep(2 * time.Second)

	Log("info", fmt.Sprintf("Message sent successfully to %s", phoneNumber))
	return nil
}

// ensureUserDataDir creates the user data directory if it doesn't exist
// and ensures it has proper permissions for Chrome to access
func ensureUserDataDir(dirPath string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, create it
			Log("info", fmt.Sprintf("Creating user data directory: %s", absPath))
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			Log("info", "User data directory created successfully")
			return nil
		}
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", absPath)
	}

	// Directory exists and is valid
	Log("info", fmt.Sprintf("Using existing user data directory: %s", absPath))
	return nil
}
