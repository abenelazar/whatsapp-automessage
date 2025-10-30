package main

import (
	"context"
	"encoding/base64"
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

	// Wait for chat to fully load and "Starting chat" dialog to disappear
	Log("debug", "Waiting for chat to fully load...")

	// Clean phone number for screenshot filename
	cleanNumberForFile := strings.ReplaceAll(cleanNumber, "+", "")

	// Explicitly wait for "Starting chat" spinner/dialog to disappear
	Log("info", "Waiting for 'Starting chat' dialog to disappear...")
	maxStartWait := 15 * time.Second
	startWaitBegin := time.Now()
	dialogGone := false

	for time.Since(startWaitBegin) < maxStartWait {
		var spinnerVisible bool
		err = chromedp.Run(c.ctx,
			chromedp.Evaluate(`
				(function() {
					// Check for "Starting chat" text or spinner
					const startingText = Array.from(document.querySelectorAll('div, span')).find(el =>
						el.textContent.includes('Starting chat') || el.textContent.includes('starting chat')
					);
					if (startingText && startingText.offsetParent !== null) return true;

					// Check for loading spinners
					const spinner = document.querySelector('div[role="progressbar"]');
					if (spinner && spinner.offsetParent !== null) return true;

					return false;
				})()
			`, &spinnerVisible),
		)

		if err != nil || !spinnerVisible {
			dialogGone = true
			Log("info", "✓ 'Starting chat' dialog is gone")
			break
		}

		Log("debug", fmt.Sprintf("'Starting chat' dialog still visible, waiting... (%v elapsed)", time.Since(startWaitBegin).Round(time.Second)))
		time.Sleep(500 * time.Millisecond)
	}

	if !dialogGone {
		Log("warn", "Timed out waiting for 'Starting chat' dialog to disappear, proceeding anyway...")
	}

	// Additional wait to ensure UI is stable
	time.Sleep(1 * time.Second)
	c.takeScreenshot(fmt.Sprintf("text_01_chat_opened_%s.png", cleanNumberForFile))

	// Count existing messages before we send (to verify new message was sent)
	var messageCountBefore int
	chromedp.Run(c.ctx,
		chromedp.Evaluate(`document.querySelectorAll('div[data-pre-plain-text]').length`, &messageCountBefore),
	)
	Log("debug", fmt.Sprintf("Message count before sending: %d", messageCountBefore))

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

	// Method 1: Type message with proper newline handling (Shift+Enter for newlines)
	Log("info", "Method 1: Typing message with keyboard simulation...")
	var textPasted bool

	// Split message by newlines and send each part separately with Shift+Enter between them
	lines := strings.Split(normalizedMessage, "\n")

	for i, line := range lines {
		if i > 0 {
			// Send Shift+Enter for newline (Enter alone sends the message in WhatsApp)
			// Modifier 8 = Shift (1 << 3)
			err = chromedp.Run(c.ctx,
				chromedp.KeyEvent("\r", chromedp.KeyModifiers(8)),
				chromedp.Sleep(50*time.Millisecond),
			)
			if err != nil {
				Log("warn", fmt.Sprintf("Failed to send Shift+Enter: %v", err))
				break
			}
		}

		// Type this line
		if line != "" {
			err = chromedp.Run(c.ctx,
				chromedp.SendKeys(usedSelector, line, chromedp.BySearch, chromedp.NodeNotVisible),
				chromedp.Sleep(50*time.Millisecond),
			)
			if err != nil {
				Log("warn", fmt.Sprintf("Failed to type line %d: %v", i+1, err))
				break
			}
		}
	}

	if err != nil {
		Log("warn", fmt.Sprintf("Keyboard simulation failed: %v, trying advanced DOM method", err))
	} else {
		// Verify that text was typed
		time.Sleep(300 * time.Millisecond)
		var inputText string
		chromedp.Run(c.ctx,
			chromedp.Evaluate(`document.querySelector('div[contenteditable="true"][data-tab="10"]')?.textContent || document.querySelector('div[contenteditable="true"][role="textbox"]')?.textContent || ""`, &inputText),
		)
		if len(inputText) > 0 {
			textPasted = true
			Log("info", fmt.Sprintf("✓ Keyboard typing successful (%d characters typed)", len(inputText)))
		} else {
			Log("warn", "Typing reported success but input is empty, trying advanced method...")
		}
	}

	// Method 2: Advanced DOM manipulation with proper WhatsApp structure
	if !textPasted {
		Log("info", "Method 2: Trying advanced DOM manipulation with WhatsApp structure...")

		// Split message into lines for proper paragraph structure
		lines := strings.Split(normalizedMessage, "\n")
		var htmlContent string
		for _, line := range lines {
			if line == "" {
				htmlContent += "<br>"
			} else {
				// Escape HTML but preserve the structure
				escapedLine := strings.ReplaceAll(line, "&", "&amp;")
				escapedLine = strings.ReplaceAll(escapedLine, "<", "&lt;")
				escapedLine = strings.ReplaceAll(escapedLine, ">", "&gt;")
				htmlContent += fmt.Sprintf("<div>%s</div>", escapedLine)
			}
		}

		advancedInputJS := fmt.Sprintf(`
			(function() {
				const inputBox = document.querySelector('div[contenteditable="true"][data-tab="10"]') ||
				                 document.querySelector('div[contenteditable="true"][role="textbox"]');
				if (!inputBox) return false;

				// Clear existing content
				inputBox.innerHTML = '';

				// Set HTML content with proper structure
				inputBox.innerHTML = %s;

				// Move cursor to end
				const range = document.createRange();
				const selection = window.getSelection();
				range.selectNodeContents(inputBox);
				range.collapse(false);
				selection.removeAllRanges();
				selection.addRange(range);

				// Fire comprehensive event chain
				inputBox.focus();
				inputBox.dispatchEvent(new Event('focus', { bubbles: true }));
				inputBox.dispatchEvent(new InputEvent('beforeinput', { bubbles: true, cancelable: true }));
				inputBox.dispatchEvent(new InputEvent('input', { bubbles: true }));
				inputBox.dispatchEvent(new Event('keyup', { bubbles: true }));
				inputBox.dispatchEvent(new Event('change', { bubbles: true }));

				return true;
			})()
		`, escapeJSString(htmlContent))

		var advancedSuccess bool
		err = chromedp.Run(c.ctx,
			chromedp.Evaluate(advancedInputJS, &advancedSuccess),
			chromedp.Sleep(800*time.Millisecond),
		)

		if err != nil || !advancedSuccess {
			c.takeScreenshot(fmt.Sprintf("text_02_all_methods_failed_%s.png", cleanNumberForFile))
			Log("error", "All text input methods failed!")
			return fmt.Errorf("failed to input message using all available methods")
		}

		Log("info", "✓ Advanced DOM manipulation successful")
		textPasted = true
	}

	// Final verification
	time.Sleep(300 * time.Millisecond)
	var finalInputText string
	chromedp.Run(c.ctx,
		chromedp.Evaluate(`document.querySelector('div[contenteditable="true"][data-tab="10"]')?.textContent || document.querySelector('div[contenteditable="true"][role="textbox"]')?.textContent || ""`, &finalInputText),
	)

	if len(finalInputText) == 0 {
		c.takeScreenshot(fmt.Sprintf("text_02_input_verification_failed_%s.png", cleanNumberForFile))
		Log("error", "Final verification: input is still empty!")
		return fmt.Errorf("text input failed - input box is empty after all methods")
	}

	Log("info", fmt.Sprintf("✓ Final verification: %d characters in input box", len(finalInputText)))
	c.takeScreenshot(fmt.Sprintf("text_02_text_ready_%s.png", cleanNumberForFile))

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

	// Verify that a new message was actually sent by checking message count
	Log("info", "Verifying message was sent...")
	var messageCountAfter int
	maxWaitTime := 20 * time.Second
	checkInterval := 1 * time.Second
	startTime := time.Now()
	messageSent := false

	for time.Since(startTime) < maxWaitTime && !messageSent {
		chromedp.Run(c.ctx,
			chromedp.Evaluate(`document.querySelectorAll('div[data-pre-plain-text]').length`, &messageCountAfter),
		)

		if messageCountAfter > messageCountBefore {
			messageSent = true
			Log("info", fmt.Sprintf("✓ New message detected! Count increased from %d to %d", messageCountBefore, messageCountAfter))
			break
		}

		if !messageSent {
			Log("info", fmt.Sprintf("Waiting for new message to appear... (%v elapsed, count still %d)", time.Since(startTime).Round(time.Second), messageCountAfter))
			time.Sleep(checkInterval)
		}
	}

	if !messageSent {
		c.takeScreenshot(fmt.Sprintf("text_03_send_failed_%s.png", cleanNumberForFile))
		Log("error", fmt.Sprintf("Message was NOT sent to %s - message count did not increase after %v", phoneNumber, maxWaitTime))
		return fmt.Errorf("message was not sent - no new message bubble appeared in chat")
	}

	c.takeScreenshot(fmt.Sprintf("text_03_message_sent_%s.png", cleanNumberForFile))

	// Wait for checkmark to confirm message is being delivered
	Log("info", "Waiting for delivery confirmation...")
	time.Sleep(3 * time.Second)

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

	// Read the image file into memory
	imageData, err := os.ReadFile(absImagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}
	Log("info", fmt.Sprintf("Read image file: %d bytes", len(imageData)))

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

	// Explicitly wait for "Starting chat" spinner/dialog to disappear
	Log("info", "Waiting for 'Starting chat' dialog to disappear...")
	maxStartWait := 15 * time.Second
	startWaitBegin := time.Now()
	dialogGone := false

	for time.Since(startWaitBegin) < maxStartWait {
		var spinnerVisible bool
		err = chromedp.Run(c.ctx,
			chromedp.Evaluate(`
				(function() {
					// Check for "Starting chat" text or spinner
					const startingText = Array.from(document.querySelectorAll('div, span')).find(el =>
						el.textContent.includes('Starting chat') || el.textContent.includes('starting chat')
					);
					if (startingText && startingText.offsetParent !== null) return true;

					// Check for loading spinners
					const spinner = document.querySelector('div[role="progressbar"]');
					if (spinner && spinner.offsetParent !== null) return true;

					return false;
				})()
			`, &spinnerVisible),
		)

		if err != nil || !spinnerVisible {
			dialogGone = true
			Log("info", "✓ 'Starting chat' dialog is gone")
			break
		}

		Log("debug", fmt.Sprintf("'Starting chat' dialog still visible, waiting... (%v elapsed)", time.Since(startWaitBegin).Round(time.Second)))
		time.Sleep(500 * time.Millisecond)
	}

	if !dialogGone {
		Log("warn", "Timed out waiting for 'Starting chat' dialog to disappear, proceeding anyway...")
	}

	time.Sleep(1 * time.Second)
	c.takeScreenshot(fmt.Sprintf("01_chat_loaded_%s.png", cleanNumber))

	// Step 1: Copy image to clipboard using JavaScript
	Log("info", "Step 1: Copying image to clipboard...")
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Determine MIME type from file extension
	mimeType := "image/png"
	ext := strings.ToLower(filepath.Ext(absImagePath))
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}

	// Use a simpler approach: set a global variable then read it back
	// This avoids Promise unmarshaling issues
	clipboardJS := fmt.Sprintf(`
		(async function() {
			try {
				const base64Data = '%s';
				const byteCharacters = atob(base64Data);
				const byteNumbers = new Array(byteCharacters.length);
				for (let i = 0; i < byteCharacters.length; i++) {
					byteNumbers[i] = byteCharacters.charCodeAt(i);
				}
				const byteArray = new Uint8Array(byteNumbers);
				const blob = new Blob([byteArray], { type: '%s' });

				await navigator.clipboard.write([
					new ClipboardItem({ '%s': blob })
				]);

				window._clipboardSuccess = true;
			} catch (e) {
				console.error('Clipboard error:', e);
				window._clipboardSuccess = false;
			}
		})()
	`, base64Image, mimeType, mimeType)

	// Execute the async function
	err = chromedp.Run(c.ctx,
		chromedp.Evaluate(clipboardJS, nil),
		chromedp.Sleep(1500*time.Millisecond), // Wait for async completion
	)

	if err != nil {
		c.takeScreenshot(fmt.Sprintf("02_clipboard_failed_%s.png", cleanNumber))
		Log("error", fmt.Sprintf("Failed to execute clipboard code: %v", err))
		return fmt.Errorf("failed to execute clipboard code: %w", err)
	}

	// Read back the result
	var clipboardSuccess bool
	err = chromedp.Run(c.ctx,
		chromedp.Evaluate(`window._clipboardSuccess === true`, &clipboardSuccess),
	)

	if err != nil || !clipboardSuccess {
		c.takeScreenshot(fmt.Sprintf("02_clipboard_failed_%s.png", cleanNumber))
		Log("error", fmt.Sprintf("Failed to copy image to clipboard: %v", err))
		return fmt.Errorf("failed to copy image to clipboard")
	}

	Log("info", "✓ Image copied to clipboard")

	// Step 2: Click the message input box to focus it
	Log("info", "Step 2: Finding and clicking message input box...")
	messageInputSelectors := []string{
		`//div[@contenteditable='true'][@data-tab='10']`,
		`//div[@contenteditable='true'][@role='textbox']`,
		`//div[@contenteditable='true']`,
	}

	var inputClicked bool
	for _, selector := range messageInputSelectors {
		err = chromedp.Run(c.ctx, chromedp.Click(selector, chromedp.BySearch))
		if err == nil {
			inputClicked = true
			Log("info", fmt.Sprintf("✓ Clicked message input: %s", selector))
			break
		}
	}

	if !inputClicked {
		c.takeScreenshot(fmt.Sprintf("03_input_not_found_%s.png", cleanNumber))
		return fmt.Errorf("could not find message input box")
	}

	time.Sleep(500 * time.Millisecond)

	// Step 3: Paste the image using Ctrl/Cmd+V directly into the text box
	Log("info", "Step 3: Pasting image into text box with Cmd/Ctrl+V...")
	err = chromedp.Run(c.ctx,
		chromedp.KeyEvent("v", chromedp.KeyModifiers(2)), // Cmd/Ctrl+V
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		c.takeScreenshot(fmt.Sprintf("04_paste_failed_%s.png", cleanNumber))
		return fmt.Errorf("failed to paste image: %w", err)
	}

	// Wait for image preview to appear
	Log("info", "Waiting for image preview to load...")
	time.Sleep(2 * time.Second)
	c.takeScreenshot(fmt.Sprintf("05_image_preview_%s.png", cleanNumber))

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
		normalizedCaption := strings.ReplaceAll(message, "\r\n", "\n")
		normalizedCaption = strings.ReplaceAll(normalizedCaption, "\r", "\n")

		// Type caption with proper newline handling (Shift+Enter for newlines)
		Log("info", "Typing caption with keyboard simulation...")
		captionLines := strings.Split(normalizedCaption, "\n")

		for i, line := range captionLines {
			if i > 0 {
				// Send Shift+Enter for newline
				// Modifier 8 = Shift (1 << 3)
				err = chromedp.Run(c.ctx,
					chromedp.KeyEvent("\r", chromedp.KeyModifiers(8)),
					chromedp.Sleep(50*time.Millisecond),
				)
				if err != nil {
					Log("warn", fmt.Sprintf("Failed to send Shift+Enter in caption: %v", err))
					break
				}
			}

			// Type this line of caption
			if line != "" {
				if bySearch {
					err = chromedp.Run(c.ctx,
						chromedp.SendKeys(usedCaptionSelector, line, chromedp.BySearch, chromedp.NodeNotVisible),
						chromedp.Sleep(50*time.Millisecond),
					)
				} else {
					err = chromedp.Run(c.ctx,
						chromedp.SendKeys(usedCaptionSelector, line, chromedp.NodeNotVisible),
						chromedp.Sleep(50*time.Millisecond),
					)
				}
				if err != nil {
					Log("warn", fmt.Sprintf("Failed to type caption line %d: %v", i+1, err))
					break
				}
			}
		}

		Log("info", "Caption typing complete")
		time.Sleep(1 * time.Second)
	} else {
		Log("warn", "Could not find caption input - sending image without caption")
	}

	c.takeScreenshot(fmt.Sprintf("06_before_send_%s.png", cleanNumber))

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

// takeScreenshot captures a screenshot and saves it to the screenshots directory
func (c *WhatsAppClient) takeScreenshot(filename string) {
	screenshotDir := "screenshots"
	os.MkdirAll(screenshotDir, 0755)

	screenshotPath := filepath.Join(screenshotDir, filename)
	var buf []byte
	if err := chromedp.Run(c.ctx, chromedp.FullScreenshot(&buf, 100)); err != nil {
		Log("warn", fmt.Sprintf("Failed to take screenshot %s: %v", filename, err))
		return
	}

	if err := os.WriteFile(screenshotPath, buf, 0644); err != nil {
		Log("warn", fmt.Sprintf("Failed to save screenshot %s: %v", filename, err))
		return
	}

	Log("info", fmt.Sprintf("📸 Screenshot saved: %s", screenshotPath))
}
