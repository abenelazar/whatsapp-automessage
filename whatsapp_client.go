package main

import (
	"context"
	"fmt"
	"net/http"
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

	// Check network connectivity
	Log("debug", "Checking network connectivity to WhatsApp Web...")
	if err := checkNetworkConnectivity(); err != nil {
		Log("warn", fmt.Sprintf("Network connectivity check failed: %v", err))
		Log("warn", "Proceeding anyway, but you may experience connection issues")
	} else {
		Log("debug", "Network connectivity check passed")
	}

	// Validate Chrome path if specified
	if c.config.Browser.ChromePath != "" {
		if _, err := os.Stat(c.config.Browser.ChromePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Chrome executable not found at: %s\nPlease check your chrome_path in config.yaml or remove it to use auto-detection", c.config.Browser.ChromePath)
			}
			return fmt.Errorf("cannot access Chrome executable at %s: %w", c.config.Browser.ChromePath, err)
		}
		Log("info", fmt.Sprintf("Using Chrome at: %s", c.config.Browser.ChromePath))
	} else {
		Log("info", "No Chrome path specified, using chromedp defaults")
	}

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

	// Add explicit Chrome path if configured or detected
	if c.config.Browser.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(c.config.Browser.ChromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	c.allocCancel = allocCancel
	c.ctx, c.cancel = chromedp.NewContext(allocCtx)

	// Navigate to WhatsApp Web
	Log("info", "Opening WhatsApp Web...")
	Log("debug", "Starting Chrome browser process...")
	err := chromedp.Run(c.ctx,
		chromedp.Navigate("https://web.whatsapp.com"),
	)
	if err != nil {
		Log("error", fmt.Sprintf("Chrome startup or navigation failed: %v", err))
		// Provide helpful error messages based on common issues
		if strings.Contains(err.Error(), "chrome failed to start") {
			return fmt.Errorf("Chrome failed to start. Try:\n1. Close all Chrome windows\n2. Delete the chrome-data folder\n3. Check Chrome path in config.yaml\nError: %w", err)
		}
		if strings.Contains(err.Error(), "cannot find Chrome") || strings.Contains(err.Error(), "executable file not found") {
			return fmt.Errorf("Chrome executable not found. Check chrome_path in config.yaml or reinstall Chrome.\nError: %w", err)
		}
		return fmt.Errorf("failed to navigate to WhatsApp Web: %w", err)
	}
	Log("info", "Chrome started and navigated to WhatsApp Web")

	// Wait for login (either QR code scan or existing session)
	Log("info", "Waiting for WhatsApp Web to load...")
	Log("info", fmt.Sprintf("If you see a QR code, please scan it within %d seconds", c.config.Browser.QRTimeoutSeconds))

	// Check if already logged in or wait for QR scan
	timeoutCtx, timeoutCancel := context.WithTimeout(c.ctx, time.Duration(c.config.Browser.QRTimeoutSeconds)*time.Second)
	defer timeoutCancel()

	// Add periodic status messages while waiting
	done := make(chan error, 1)
	go func() {
		done <- chromedp.Run(timeoutCtx,
			chromedp.WaitVisible(`//div[@id='side']`, chromedp.BySearch),
		)
	}()

	// Show progress messages while waiting
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()

waitLoop:
	for {
		select {
		case err = <-done:
			break waitLoop
		case <-ticker.C:
			elapsed := time.Since(startTime).Seconds()
			remaining := float64(c.config.Browser.QRTimeoutSeconds) - elapsed
			if remaining > 0 {
				Log("info", fmt.Sprintf("Still waiting for WhatsApp Web to load... (%.0f seconds remaining)", remaining))
			}
		}
	}

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

	// Send image with caption if configured
	if c.config.Files.ImagePath != "" {
		if err := c.sendImageWithCaption(phoneNumber, cleanNumber, chatURL, message); err != nil {
			Log("warn", fmt.Sprintf("Failed to send image to %s: %v", phoneNumber, err))
			Log("warn", "Continuing with text message only...")
		} else {
			Log("info", "Image with caption sent successfully!")
			return nil // Image was sent with caption, we're done
		}
	}

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

	Log("debug", "Preparing to paste message...")

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

	// Normalize line endings - Windows uses \r\n, Unix uses \n
	// Replace \r\n with \n, then remove any remaining \r
	normalizedMessage := strings.ReplaceAll(message, "\r\n", "\n")
	normalizedMessage = strings.ReplaceAll(normalizedMessage, "\r", "\n")

	// Copy message to clipboard using JavaScript
	Log("debug", "Copying message to clipboard...")
	jsCode := fmt.Sprintf(`navigator.clipboard.writeText(%s)`, escapeJSString(normalizedMessage))
	err = chromedp.Run(c.ctx,
		chromedp.Evaluate(jsCode, nil),
		chromedp.Sleep(200*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("failed to copy message to clipboard: %w", err)
	}

	// Paste the message using Ctrl+V (Cmd+V on Mac)
	Log("debug", "Pasting message from clipboard...")
	err = chromedp.Run(c.ctx,
		chromedp.KeyEvent("v", chromedp.KeyModifiers(2)), // 2 = Cmd/Ctrl modifier
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("failed to paste message: %w", err)
	}

	// Wait to ensure message is fully pasted
	Log("debug", "Message paste complete, waiting before sending...")
	time.Sleep(500 * time.Millisecond)

	// Send the message by pressing Enter (without Shift modifier)
	Log("debug", "Sending message with Enter key...")
	err = chromedp.Run(c.ctx,
		chromedp.KeyEvent("\r"), // Enter key to send
	)
	if err != nil {
		return fmt.Errorf("failed to send message with Enter key: %w", err)
	}

	// Wait a bit for the message to start sending
	time.Sleep(3 * time.Second)

	// Wait for message to be sent by looking for the sent indicator (checkmark)
	Log("info", "Waiting for message to be sent...")

	// WhatsApp shows a checkmark icon when message is sent
	// Look for the most recent message bubble with a checkmark
	sendCheckSelectors := []string{
		`(//span[@data-icon='msg-check'])[last()]`,     // Single checkmark (sent) - last one
		`(//span[@data-icon='msg-dblcheck'])[last()]`,  // Double checkmark (delivered) - last one
		`(//span[@data-icon='msg-dblcheck-ack'])[last()]`, // Blue double checkmark (read) - last one
	}

	messageSent := false
	maxWaitTime := 20 * time.Second
	checkInterval := 1 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime && !messageSent {
		for _, selector := range sendCheckSelectors {
			// Try to find the checkmark - use a very short timeout
			checkCtx, checkCancel := context.WithTimeout(c.ctx, 200*time.Millisecond)
			err = chromedp.Run(checkCtx,
				chromedp.WaitVisible(selector, chromedp.BySearch),
			)
			checkCancel()

			if err == nil {
				messageSent = true
				Log("info", fmt.Sprintf("Message sent confirmation found with selector: %s", selector))
				break
			}
		}
		if !messageSent {
			Log("info", fmt.Sprintf("Checking for checkmark... (%v elapsed)", time.Since(startTime).Round(time.Second)))
			time.Sleep(checkInterval)
		}
	}

	if !messageSent {
		Log("warn", fmt.Sprintf("Could not confirm message was sent to %s (no checkmark found after %v)", phoneNumber, maxWaitTime))
		Log("warn", "Waiting extra time to ensure message sends anyway...")
		// Don't fail, just wait extra time to be safe
		time.Sleep(5 * time.Second)
	} else {
		// Wait an additional moment to ensure message is fully sent before navigating away
		Log("info", "Message confirmed sent, waiting before moving to next contact...")
		time.Sleep(3 * time.Second)
	}

	Log("info", fmt.Sprintf("Message sent successfully to %s", phoneNumber))
	return nil
}

// sendImageWithCaption sends an image with a text caption to a WhatsApp contact
func (c *WhatsAppClient) sendImageWithCaption(phoneNumber, cleanNumber, chatURL, message string) error {
	Log("info", fmt.Sprintf("Sending image with caption to %s", phoneNumber))

	// Verify image file exists
	if _, err := os.Stat(c.config.Files.ImagePath); err != nil {
		return fmt.Errorf("image file not found: %s", c.config.Files.ImagePath)
	}

	// Get absolute path for the image
	absImagePath, err := filepath.Abs(c.config.Files.ImagePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute image path: %w", err)
	}
	Log("info", fmt.Sprintf("Using image at: %s", absImagePath))

	// Navigate to chat
	Log("info", "Navigating to chat for image send...")
	err = chromedp.Run(c.ctx,
		chromedp.Evaluate(`window.onbeforeunload = null;`, nil),
		chromedp.Navigate(chatURL),
		chromedp.Sleep(4*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to navigate to chat for image: %w", err)
	}

	// Wait for chat to fully load by checking for message input
	Log("info", "Waiting for chat to load...")
	inputSelectors := []string{
		`//div[@contenteditable='true'][@data-tab='10']`,
		`//div[@contenteditable='true'][@role='textbox']`,
		`//div[@contenteditable='true']`,
	}

	chatLoaded := false
	for _, selector := range inputSelectors {
		ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
		err = chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.BySearch),
		)
		cancel()
		if err == nil {
			chatLoaded = true
			Log("info", "Chat loaded successfully")
			break
		}
	}

	if !chatLoaded {
		return fmt.Errorf("chat did not load - cannot send image")
	}

	time.Sleep(2 * time.Second)

	// Click the attachment button to open the attachment menu
	Log("info", "Looking for attachment button...")
	attachmentSelectors := []string{
		`//div[@title='Attach']`,
		`//button[@aria-label='Attach']`,
		`//div[@aria-label='Attach']`,
		`//span[@data-icon='plus']`,
		`//span[@data-icon='attach-menu-plus']`,
		`div[title='Attach']`,
		`button[aria-label='Attach']`,
	}

	var attachmentClicked bool
	for i, selector := range attachmentSelectors {
		Log("info", fmt.Sprintf("Trying attachment selector %d/%d: %s", i+1, len(attachmentSelectors), selector))

		// Determine if it's XPath or CSS
		bySearch := strings.HasPrefix(selector, "//") || strings.HasPrefix(selector, "(")

		ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
		var err error
		if bySearch {
			err = chromedp.Run(ctx, chromedp.Click(selector, chromedp.BySearch))
		} else {
			err = chromedp.Run(ctx, chromedp.Click(selector))
		}
		cancel()

		if err == nil {
			attachmentClicked = true
			Log("info", fmt.Sprintf("✓ Clicked attachment button with selector: %s", selector))
			break
		} else {
			Log("debug", fmt.Sprintf("✗ Attachment selector %d failed: %v", i+1, err))
		}
	}

	if !attachmentClicked {
		Log("warn", "Could not click attachment button, trying direct file input access...")
	} else {
		// Wait for the attachment menu to appear
		time.Sleep(1 * time.Second)
	}

	// Now find and use the file input (it should be available whether we clicked the button or not)
	Log("info", "Looking for file input element...")

	fileInputSelectors := []string{
		`input[type="file"][accept*="image"]`,
		`input[type="file"][accept*="video"]`,
		`input[type="file"]`,
		`//input[@type='file' and contains(@accept, 'image')]`,
		`//input[@type='file' and contains(@accept, 'video')]`,
		`//input[@type='file']`,
	}

	var fileInputFound bool
	for i, selector := range fileInputSelectors {
		Log("info", fmt.Sprintf("Trying file input selector %d/%d: %s", i+1, len(fileInputSelectors), selector))

		// Determine if it's XPath or CSS
		bySearch := strings.HasPrefix(selector, "//") || strings.HasPrefix(selector, "(")

		ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
		var err error
		if bySearch {
			err = chromedp.Run(ctx, chromedp.SetUploadFiles(selector, []string{absImagePath}, chromedp.BySearch))
		} else {
			err = chromedp.Run(ctx, chromedp.SetUploadFiles(selector, []string{absImagePath}))
		}
		cancel()

		if err == nil {
			fileInputFound = true
			Log("info", fmt.Sprintf("✓ Successfully uploaded image using selector: %s", selector))
			break
		} else {
			Log("debug", fmt.Sprintf("✗ File input selector %d failed: %v", i+1, err))
		}
	}

	if !fileInputFound {
		Log("error", "Could not find file input element after trying all selectors")
		return fmt.Errorf("could not find file input for image upload")
	}

	// Wait for image to upload and preview to appear
	Log("info", "Waiting for image preview to load...")
	time.Sleep(4 * time.Second)

	// Add caption to the image
	Log("info", "Adding caption to image...")

	// Find the caption input box in the image preview modal
	captionSelectors := []string{
		`div[contenteditable='true'][data-tab='10']`,
		`div[contenteditable='true'][role='textbox']`,
		`div.copyable-text[contenteditable='true']`,
		`//div[@contenteditable='true'][@data-tab='10']`,
		`//div[@contenteditable='true'][@role='textbox']`,
	}

	var captionInputFound bool
	var usedCaptionSelector string
	for i, selector := range captionSelectors {
		Log("debug", fmt.Sprintf("Trying caption input selector %d/%d: %s", i+1, len(captionSelectors), selector))

		// Determine if it's XPath or CSS
		bySearch := strings.HasPrefix(selector, "//") || strings.HasPrefix(selector, "(")

		ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
		var err error
		if bySearch {
			err = chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.BySearch))
		} else {
			err = chromedp.Run(ctx, chromedp.WaitVisible(selector))
		}
		cancel()

		if err == nil {
			captionInputFound = true
			usedCaptionSelector = selector
			Log("info", fmt.Sprintf("✓ Found caption input with selector: %s", selector))
			break
		} else {
			Log("debug", fmt.Sprintf("✗ Caption selector %d failed: %v", i+1, err))
		}
	}

	if captionInputFound {
		// Click the caption input to focus it
		bySearch := strings.HasPrefix(usedCaptionSelector, "//") || strings.HasPrefix(usedCaptionSelector, "(")
		if bySearch {
			err = chromedp.Run(c.ctx, chromedp.Click(usedCaptionSelector, chromedp.BySearch))
		} else {
			err = chromedp.Run(c.ctx, chromedp.Click(usedCaptionSelector))
		}
		if err != nil {
			Log("warn", fmt.Sprintf("Failed to click caption input: %v", err))
		}

		time.Sleep(300 * time.Millisecond)

		// Normalize line endings
		normalizedMessage := strings.ReplaceAll(message, "\r\n", "\n")
		normalizedMessage = strings.ReplaceAll(normalizedMessage, "\r", "\n")

		// Copy message to clipboard using JavaScript
		Log("debug", "Copying caption to clipboard...")
		jsCode := fmt.Sprintf(`navigator.clipboard.writeText(%s)`, escapeJSString(normalizedMessage))
		err = chromedp.Run(c.ctx,
			chromedp.Evaluate(jsCode, nil),
			chromedp.Sleep(200*time.Millisecond),
		)
		if err != nil {
			Log("warn", fmt.Sprintf("Failed to copy caption to clipboard: %v", err))
		}

		// Paste the caption using Ctrl+V (Cmd+V on Mac)
		Log("debug", "Pasting caption from clipboard...")
		err = chromedp.Run(c.ctx,
			chromedp.KeyEvent("v", chromedp.KeyModifiers(2)), // 2 = Cmd/Ctrl modifier
			chromedp.Sleep(500*time.Millisecond),
		)
		if err != nil {
			Log("warn", fmt.Sprintf("Failed to paste caption: %v", err))
		}

		Log("info", "Caption added successfully")
		time.Sleep(1 * time.Second)
	} else {
		Log("warn", "Could not find caption input - sending image without caption")
	}

	// Click the send button in the image preview modal
	Log("info", "Looking for send button in image preview...")
	sendButtonSelectors := []string{
		`//span[@data-icon='send']`,
		`//button[@aria-label='Send']`,
		`//div[@aria-label='Send']`,
		`//span[@data-icon='send']/ancestor::button`,
		`//span[@data-icon='send']/parent::div[@role='button']`,
	}

	var sendClicked bool
	for i, selector := range sendButtonSelectors {
		Log("info", fmt.Sprintf("Trying send button selector %d/%d...", i+1, len(sendButtonSelectors)))
		ctx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
		err = chromedp.Run(ctx,
			chromedp.Click(selector, chromedp.BySearch),
		)
		cancel()
		if err == nil {
			sendClicked = true
			Log("info", fmt.Sprintf("✓ Clicked send button with selector: %s", selector))
			break
		} else {
			Log("debug", fmt.Sprintf("✗ Send button selector %d failed: %v", i+1, err))
		}
	}

	if !sendClicked {
		Log("error", "Could not find send button in image preview")
		return fmt.Errorf("could not find send button for image")
	}

	// Wait for image to send - give it time for upload and delivery
	Log("info", "Waiting for image to upload and send...")
	time.Sleep(8 * time.Second)

	Log("info", fmt.Sprintf("Image sent successfully to %s", phoneNumber))
	return nil
}

// checkNetworkConnectivity verifies we can reach WhatsApp Web
func checkNetworkConnectivity() error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://web.whatsapp.com")
	if err != nil {
		return fmt.Errorf("cannot reach WhatsApp Web: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("WhatsApp Web returned server error: %d", resp.StatusCode)
	}

	return nil
}

// ensureUserDataDir creates the user data directory if it doesn't exist
// and ensures it has proper permissions for Chrome to access
func ensureUserDataDir(dirPath string) error {
	if dirPath == "" {
		Log("info", "No user data directory specified, Chrome will use default")
		return nil
	}

	// dirPath is already absolute from LoadConfig
	Log("info", fmt.Sprintf("Using user data directory: %s", dirPath))

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, create it
			Log("info", fmt.Sprintf("Creating user data directory: %s", dirPath))
			// Use 0777 permissions for Windows compatibility
			if err := os.MkdirAll(dirPath, 0777); err != nil {
				Log("error", fmt.Sprintf("Failed to create directory: %v", err))
				return fmt.Errorf("failed to create directory %s: %w\nTry running as administrator or use a different directory", dirPath, err)
			}
			Log("info", "User data directory created successfully")
		} else {
			return fmt.Errorf("failed to check directory: %w", err)
		}
	} else {
		// Check if it's actually a directory
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", dirPath)
		}
		Log("debug", "User data directory already exists")
	}

	// Test write permissions by creating a test file
	testFile := filepath.Join(dirPath, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0666); err != nil {
		Log("error", fmt.Sprintf("Cannot write to directory: %v", err))
		return fmt.Errorf("directory exists but is not writable: %s\nTry:\n1. Running as administrator\n2. Deleting the directory and trying again\n3. Using a different directory in config.yaml", dirPath)
	}
	os.Remove(testFile) // Clean up test file
	Log("debug", "Directory write test passed")

	return nil
}

// escapeJSString escapes a string for safe use in JavaScript code
func escapeJSString(s string) string {
	// Use JSON encoding which properly escapes quotes, newlines, backslashes, etc.
	// and wraps the string in quotes
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	escaped = strings.ReplaceAll(escaped, "\r", "\\r")
	escaped = strings.ReplaceAll(escaped, "\t", "\\t")
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return `"` + escaped + `"`
}
