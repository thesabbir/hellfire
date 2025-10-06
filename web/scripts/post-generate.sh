#!/bin/bash
# Post-generation script to add custom client export
# Run this after npm run generate-client

API_INDEX_FILE="src/lib/api/index.ts"

# Check if the custom export already exists
if grep -q "export { client } from '../api-client'" "$API_INDEX_FILE"; then
  echo "✅ Custom client export already exists"
  exit 0
fi

# Add the custom export
cat >> "$API_INDEX_FILE" << 'EOF'

// Re-export the custom client with auth interceptor
// Note: This line needs to be added back after regenerating the API client
export { client } from '../api-client';
EOF

echo "✅ Added custom client export to $API_INDEX_FILE"
