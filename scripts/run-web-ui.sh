#!/bin/bash

# AI Infrastructure Agent Web UI Launch Script

echo "🚀 Starting AI Infrastructure Agent Web UI..."

# Set default values
PORT=${PORT:-8080}
HOST=${HOST:-"0.0.0.0"}

# Check if config.yaml exists
if [ ! -f config.yaml ]; then
    echo "📝 config.yaml not found. Please run the installation script first."
    exit 1
fi

# Start the web UI with Gunicorn
echo "🌐 Starting AI Infrastructure Agent Web UI on port ${PORT}..."
echo "🔗 Open: http://localhost:${PORT}"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

gunicorn --bind "${HOST}:${PORT}" api.app:app
