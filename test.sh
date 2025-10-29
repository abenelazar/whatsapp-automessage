#!/bin/bash

# Test script for WhatsApp Automation
# This demonstrates the dry-run functionality

echo "Creating temporary test config..."

# Create a temporary config with dummy credentials for dry-run testing
cat > config.test.yaml <<EOF
whatsapp:
  api_url: "https://graph.facebook.com/v18.0"
  phone_number_id: "TEST_PHONE_NUMBER_ID"
  access_token: "TEST_ACCESS_TOKEN"

files:
  csv_path: "contacts.example.csv"
  template_path: "template.txt"

retry:
  max_retries: 3
  initial_delay_seconds: 2
  max_delay_seconds: 30
  backoff_multiplier: 2

rate_limiting:
  messages_per_second: 1
  enabled: true

logging:
  level: "info"
  output_file: "test.log"
EOF

echo "Running dry-run test..."
./whatsapp-automation -config config.test.yaml -dry-run

# Clean up
rm -f config.test.yaml test.log

echo ""
echo "Test completed!"
