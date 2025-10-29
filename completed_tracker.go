package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

type CompletedContact struct {
	Name        string
	PhoneNumber string
	Hash        string
	Timestamp   string
}

type CompletedTracker struct {
	filePath        string
	completed       map[string]CompletedContact // key: hash
	messageTemplate string                      // Store template for hash generation
}

func NewCompletedTracker(filePath string, messageTemplate string) (*CompletedTracker, error) {
	tracker := &CompletedTracker{
		filePath:        filePath,
		completed:       make(map[string]CompletedContact),
		messageTemplate: messageTemplate,
	}

	// Load existing completed contacts if file exists
	if err := tracker.load(); err != nil {
		// If file doesn't exist, that's okay - we'll create it
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load completed contacts: %w", err)
		}
	}

	return tracker, nil
}

// generateHash creates a unique hash for a contact based on phone, name, fields, and message template
func (ct *CompletedTracker) generateHash(contact Contact) string {
	// Start with phone, name, and message template
	data := fmt.Sprintf("%s|%s|%s", contact.PhoneNumber, contact.Name, ct.messageTemplate)

	// Add all additional fields in sorted order for consistency
	// This ensures the hash is the same regardless of field order
	for key, value := range contact.Fields {
		data += fmt.Sprintf("|%s:%s", key, value)
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (ct *CompletedTracker) load() error {
	file, err := os.Open(ct.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read completed CSV: %w", err)
	}

	if len(records) == 0 {
		return nil // Empty file
	}

	// Parse header
	header := records[0]
	nameIdx := -1
	phoneIdx := -1
	hashIdx := -1
	timestampIdx := -1

	for i, col := range header {
		col = strings.TrimSpace(strings.ToLower(col))
		if col == "name" {
			nameIdx = i
		} else if col == "phone_number" || col == "phone" {
			phoneIdx = i
		} else if col == "hash" {
			hashIdx = i
		} else if col == "timestamp" || col == "date" {
			timestampIdx = i
		}
	}

	if nameIdx == -1 || phoneIdx == -1 || hashIdx == -1 {
		return fmt.Errorf("completed CSV must contain 'name', 'phone_number', and 'hash' columns")
	}

	// Parse completed contacts
	for i := 1; i < len(records); i++ {
		row := records[i]

		// Skip empty rows
		if len(row) == 0 || (len(row) > hashIdx && strings.TrimSpace(row[hashIdx]) == "") {
			continue
		}

		if len(row) <= hashIdx {
			continue // Skip malformed rows
		}

		hash := strings.TrimSpace(row[hashIdx])
		contact := CompletedContact{
			Hash: hash,
		}

		if len(row) > phoneIdx {
			contact.PhoneNumber = strings.TrimSpace(row[phoneIdx])
		}

		if len(row) > nameIdx {
			contact.Name = strings.TrimSpace(row[nameIdx])
		}

		if timestampIdx != -1 && len(row) > timestampIdx {
			contact.Timestamp = strings.TrimSpace(row[timestampIdx])
		}

		ct.completed[hash] = contact
	}

	Log("info", fmt.Sprintf("Loaded %d completed contacts from %s", len(ct.completed), ct.filePath))

	return nil
}

func (ct *CompletedTracker) IsCompleted(contact Contact) bool {
	hash := ct.generateHash(contact)
	_, exists := ct.completed[hash]
	return exists
}

func (ct *CompletedTracker) MarkCompleted(contact Contact) error {
	hash := ct.generateHash(contact)

	// Add to in-memory map
	completedContact := CompletedContact{
		Name:        contact.Name,
		PhoneNumber: contact.PhoneNumber,
		Hash:        hash,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}
	ct.completed[hash] = completedContact

	// Append to CSV file
	return ct.appendToFile(completedContact)
}

func (ct *CompletedTracker) appendToFile(contact CompletedContact) error {
	// Check if file exists to determine if we need to write header
	fileExists := true
	if _, err := os.Stat(ct.filePath); os.IsNotExist(err) {
		fileExists = false
	}

	// Open file in append mode
	file, err := os.OpenFile(ct.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open completed CSV: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header if new file
	if !fileExists {
		if err := writer.Write([]string{"name", "phone_number", "hash", "timestamp"}); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	// Write contact record
	record := []string{
		contact.Name,
		contact.PhoneNumber,
		contact.Hash,
		contact.Timestamp,
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write CSV record: %w", err)
	}

	return nil
}

func (ct *CompletedTracker) GetCompletedCount() int {
	return len(ct.completed)
}
