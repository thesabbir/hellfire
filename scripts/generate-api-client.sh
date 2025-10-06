#!/bin/bash
set -e

# Script to generate OpenAPI client for the frontend
# This should be run whenever the backend API changes

echo "🔨 Generating Swagger documentation from Go code..."
swag init -g cmd/hf/api.go -o docs --outputTypes go,json

echo "📋 Copying swagger.json to web directory..."
cp docs/swagger.json web/openapi.json

echo "🎨 Generating TypeScript API client..."
cd web
npm run generate-client

echo "✅ API client generation complete!"
echo ""
echo "The following files have been updated:"
echo "  - docs/swagger.json"
echo "  - docs/docs.go"
echo "  - web/openapi.json"
echo "  - web/src/lib/api/*.gen.ts"
