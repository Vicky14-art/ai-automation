#!/bin/bash
set -e

echo "Starting Deployment Process..."

# Pastikan berada di root direktori project
cd "$(dirname "$0")/.."

echo "Building and restarting Docker containers..."
docker-compose up -d --build

echo "Deployment completed successfully!"
