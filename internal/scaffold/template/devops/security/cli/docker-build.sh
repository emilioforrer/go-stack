#!/bin/bash

# Docker build script for SonarQube CLI tool
set -e

echo "Building SonarQube CLI Docker image..."

# Build the Docker image
docker build -t sonar-cli:latest .

echo "Docker image built successfully!"
echo "Image: sonar-cli:latest"
echo ""
echo "Usage examples:"
echo "  # Run with docker directly"
echo "  docker run --rm -v \$(pwd)/reports:/app/reports sonar-cli:latest issues https://sonar.example.com squ_token... project-key"
echo ""
echo "  # Run with docker-compose"
echo "  cd ../.. && docker-compose --profile tools run --rm sonar-cli issues https://sonar.example.com squ_token... project-key"
