#!/bin/bash

# Script to run OWASP Dependency Check and generate reports for SonarQube
# This script should be run before the SonarQube scanner
#
# The script caches the vulnerability database to avoid downloading it every time.
# Use --force-update to force a fresh download of the vulnerability database.
#
# Usage:
#   ./run-dependency-check.sh              # Use cached database (faster)
#   ./run-dependency-check.sh --force-update # Force fresh database download

set -e

# Check if we should force a fresh update
FORCE_UPDATE=false
if [[ "$1" == "--force-update" ]]; then
    FORCE_UPDATE=true
    echo "Starting OWASP Dependency Check scan with forced database update..."
else
    echo "Starting OWASP Dependency Check scan (will use cached database if available)..."
fi

# Create reports directory if it doesn't exist
mkdir -p devops/security/dependency-check-reports



# Create data directory for persistent vulnerability database cache
mkdir -p devops/security/dependency-check-data

# Clear cached data if force update is requested
if [[ "$FORCE_UPDATE" == "true" ]]; then
    echo "Clearing cached vulnerability database..."
    rm -rf devops/security/dependency-check-data/*
fi

# Run dependency check using Docker with persistent data volume
docker run --rm \
    -v "$(pwd):/src" \
    -v "$(pwd)/devops/security/dependency-check-reports:/reports" \
    -v "$(pwd)/devops/security/dependency-check-data:/usr/share/dependency-check/data" \
    owasp/dependency-check:latest \
    --disableOssIndex true \
    --scan /src \
    --format "ALL" \
    --project "sa-intranet" \
    --out /reports \
    --exclude "**/node_modules/**" \
    --exclude "**/vendor/**" \
    --exclude "**/tests/**" \
    --exclude "**/tmp/**" \
    --exclude "**/dist/**" \
    --exclude "**/public/**" \
    --exclude "**/bootstrap/cache/**" \
    --enableRetired

echo "Dependency Check scan completed!"
echo "Reports generated in dependency-check-reports directory:"
echo "- dependency-check-report.xml (for SonarQube)"
echo "- dependency-check-report.html (for viewing)"
echo "- dependency-check-report.json (for automation)"


