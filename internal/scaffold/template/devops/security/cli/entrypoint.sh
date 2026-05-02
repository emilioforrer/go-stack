#!/bin/sh

# SonarQube CLI Docker Entrypoint Script
# This script allows the container to be run with environment variables
# or with explicit command line arguments

# Ensure the reports directory exists
mkdir -p /app/reports

if [ $# -eq 0 ]; then
    # No arguments provided - use environment variables for default behavior
    if [ -z "$SONAR_HOST_URL" ] || [ -z "$SONAR_TOKEN" ] || [ -z "$SONAR_PROJECT_KEY" ]; then
        echo "Error: When no arguments are provided, the following environment variables must be set:"
        echo "  SONAR_HOST_URL - SonarQube server URL"
        echo "  SONAR_TOKEN - SonarQube authentication token"
        echo "  SONAR_PROJECT_KEY - Project key to analyze"
        echo ""
        echo "Alternatively, you can provide arguments directly:"
        echo "  docker run ... sonar-cli issues <sonar-url> <sonar-token> <project-key>"
        echo ""
        echo "Available commands:"
        exec ./sonar-cli --help
        exit 1
    fi

    # Use environment variables with default output directory
    echo "Using environment variables:"
    echo "  SONAR_HOST_URL: $SONAR_HOST_URL"
    echo "  SONAR_PROJECT_KEY: $SONAR_PROJECT_KEY"
    echo "  Output directory: /app/reports"
    echo ""

    exec ./sonar-cli issues "$SONAR_HOST_URL" "$SONAR_TOKEN" "$SONAR_PROJECT_KEY" --output-dir /app/reports
else
    # Arguments provided - pass them through to the CLI
    exec ./sonar-cli "$@"
fi
