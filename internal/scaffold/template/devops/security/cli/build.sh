#!/bin/bash

# Build script for SonarQube CLI tool
set -e

echo "Building SonarQube CLI tool..."

# Ensure dependencies are up to date
echo "Updating dependencies..."
go mod tidy

# Build the executable
echo "Building executable..."
go build -o sonar-cli main.go

echo "Build completed successfully!"
echo "Executable: ./sonar-cli"
echo ""
echo "Usage examples:"
echo "  ./sonar-cli issues https://sonar.example.com squ_token... project-key"
echo "  ./sonar-cli issues --help"
