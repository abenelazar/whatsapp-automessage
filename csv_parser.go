package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type Contact struct {
	Name        string
	PhoneNumber string
	Fields      map[string]string // Dynamic fields from CSV
}

func ParseCSV(filePath string) ([]Contact, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// Parse header
	header := records[0]
	nameIdx := -1
	phoneIdx := -1

	// Normalize headers and track all column indices
	normalizedHeaders := make([]string, len(header))
	for i, col := range header {
		normalizedHeaders[i] = strings.TrimSpace(col)
		colLower := strings.ToLower(normalizedHeaders[i])
		if colLower == "name" {
			nameIdx = i
		} else if colLower == "phone_number" || colLower == "phone" {
			phoneIdx = i
		}
	}

	if nameIdx == -1 || phoneIdx == -1 {
		return nil, fmt.Errorf("CSV must contain 'name' and 'phone_number' columns")
	}

	// Parse contacts
	contacts := make([]Contact, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		row := records[i]

		// Skip empty rows
		if len(row) == 0 || (len(row) > nameIdx && strings.TrimSpace(row[nameIdx]) == "") {
			continue
		}

		if len(row) <= nameIdx || len(row) <= phoneIdx {
			return nil, fmt.Errorf("row %d has insufficient columns", i+1)
		}

		contact := Contact{
			Name:        strings.TrimSpace(row[nameIdx]),
			PhoneNumber: strings.TrimSpace(row[phoneIdx]),
			Fields:      make(map[string]string),
		}

		// Validate phone number format (basic validation)
		if contact.PhoneNumber == "" {
			return nil, fmt.Errorf("row %d has empty phone number", i+1)
		}

		// Parse all additional fields (excluding name and phone)
		for j, value := range row {
			if j != nameIdx && j != phoneIdx {
				// Capitalize first letter of field name for template compatibility
				fieldName := normalizedHeaders[j]
				if len(fieldName) > 0 {
					// Capitalize first letter: "value" -> "Value"
					fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
				}
				contact.Fields[fieldName] = strings.TrimSpace(value)
			}
		}

		contacts = append(contacts, contact)
	}

	return contacts, nil
}
