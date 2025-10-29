package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type MessageResult struct {
	Contact Contact
	Success bool
	Error   error
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without sending messages")
	flag.Parse()

	// Load configuration
	Log("info", fmt.Sprintf("Loading configuration from %s", *configPath))
	config, err := LoadConfig(*configPath)
	if err != nil {
		Log("error", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}

	// Initialize logger
	if err := InitLogger(config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer CloseLogger()

	Log("info", "WhatsApp Automation started")

	// Load contacts from CSV
	Log("info", fmt.Sprintf("Loading contacts from %s", config.Files.CSVPath))
	contacts, err := ParseCSV(config.Files.CSVPath)
	if err != nil {
		Log("error", fmt.Sprintf("Failed to parse CSV: %v", err))
		os.Exit(1)
	}
	Log("info", fmt.Sprintf("Loaded %d contacts", len(contacts)))

	// Load message template
	Log("info", fmt.Sprintf("Loading message template from %s", config.Files.TemplatePath))
	msgTemplate, err := LoadTemplate(config.Files.TemplatePath)
	if err != nil {
		Log("error", fmt.Sprintf("Failed to load template: %v", err))
		os.Exit(1)
	}

	// Initialize completed contacts tracker
	Log("info", fmt.Sprintf("Loading completed contacts from %s", config.Files.CompletedCSVPath))
	tracker, err := NewCompletedTracker(config.Files.CompletedCSVPath, msgTemplate.Content)
	if err != nil {
		Log("error", fmt.Sprintf("Failed to initialize completed tracker: %v", err))
		os.Exit(1)
	}

	// Initialize WhatsApp client
	whatsappClient := NewWhatsAppClient(config)

	// Initialize browser automation (skip for dry-run)
	if !*dryRun {
		if err := whatsappClient.Initialize(); err != nil {
			Log("error", fmt.Sprintf("Failed to initialize WhatsApp client: %v", err))
			os.Exit(1)
		}
		defer whatsappClient.Close()
	}

	// Process contacts
	results := make([]MessageResult, 0, len(contacts))
	successCount := 0
	failureCount := 0
	skippedCount := 0

	startTime := time.Now()

	for i, contact := range contacts {
		Log("info", fmt.Sprintf("Processing contact %d/%d: %s (%s)",
			i+1, len(contacts), contact.Name, contact.PhoneNumber))

		// Check if already completed
		if tracker.IsCompleted(contact) {
			Log("info", fmt.Sprintf("Skipping %s - already sent message previously", contact.PhoneNumber))
			skippedCount++
			continue
		}

		// Render message for this contact
		message, err := msgTemplate.Render(contact)
		if err != nil {
			Log("error", fmt.Sprintf("Failed to render template for %s: %v",
				contact.Name, err))
			results = append(results, MessageResult{
				Contact: contact,
				Success: false,
				Error:   err,
			})
			failureCount++
			continue
		}

		if *dryRun {
			Log("info", fmt.Sprintf("[DRY RUN] Would send message to %s:\n%s",
				contact.PhoneNumber, message))
			results = append(results, MessageResult{
				Contact: contact,
				Success: true,
				Error:   nil,
			})
			successCount++
			continue
		}

		// Send message
		err = whatsappClient.SendMessage(contact.PhoneNumber, message)
		if err != nil {
			Log("error", fmt.Sprintf("Failed to send message to %s: %v",
				contact.Name, err))
			results = append(results, MessageResult{
				Contact: contact,
				Success: false,
				Error:   err,
			})
			failureCount++
		} else {
			Log("info", fmt.Sprintf("Successfully sent message to %s", contact.Name))

			// Mark as completed
			if err := tracker.MarkCompleted(contact); err != nil {
				Log("warn", fmt.Sprintf("Failed to mark %s as completed: %v", contact.PhoneNumber, err))
			}

			results = append(results, MessageResult{
				Contact: contact,
				Success: true,
				Error:   nil,
			})
			successCount++
		}
	}

	duration := time.Since(startTime)

	// Print summary
	Log("info", "=== Automation Summary ===")
	Log("info", fmt.Sprintf("Total contacts: %d", len(contacts)))
	Log("info", fmt.Sprintf("Successful: %d", successCount))
	Log("info", fmt.Sprintf("Failed: %d", failureCount))
	Log("info", fmt.Sprintf("Skipped (already sent): %d", skippedCount))
	Log("info", fmt.Sprintf("Duration: %v", duration))

	if failureCount > 0 {
		Log("warn", "\nFailed contacts:")
		for _, result := range results {
			if !result.Success {
				Log("warn", fmt.Sprintf("  - %s (%s): %v",
					result.Contact.Name, result.Contact.PhoneNumber, result.Error))
			}
		}
	}

	Log("info", "WhatsApp Automation completed")

	if failureCount > 0 {
		os.Exit(1)
	}
}
