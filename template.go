package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type MessageTemplate struct {
	tmpl    *template.Template
	Content string // Raw template content for hashing
}

func LoadTemplate(filePath string) (*MessageTemplate, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	tmpl, err := template.New("message").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &MessageTemplate{
		tmpl:    tmpl,
		Content: string(content),
	}, nil
}

func (mt *MessageTemplate) Render(contact Contact) (string, error) {
	// Create a map that includes both standard fields and dynamic fields
	data := make(map[string]interface{})
	data["Name"] = contact.Name
	data["PhoneNumber"] = contact.PhoneNumber

	// Add all dynamic fields from the CSV
	for key, value := range contact.Fields {
		data[key] = value
	}

	var buf bytes.Buffer
	if err := mt.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}
