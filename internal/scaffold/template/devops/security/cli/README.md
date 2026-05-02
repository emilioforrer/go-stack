# SonarQube CLI Tool

A standalone command-line tool for querying SonarQube issues and generating comprehensive reports.

## Features

- **Complete Issue Retrieval**: Fetches all issues from a SonarQube project with automatic pagination
- **Multiple Report Formats**: Generates JSON, CSV, and summary text reports
- **Authentication Validation**: Tests SonarQube connection and authentication before querying
- **Project Verification**: Confirms the project exists before attempting to fetch issues
- **Comprehensive Reporting**: Provides detailed breakdowns by type, severity, status, and components
- **Error Handling**: Robust error handling with informative messages

## Installation

### Build from Source

```bash
cd devops/security/cli
go mod tidy
go build -o sonar-cli main.go
```

### Docker Build

```bash
cd devops/security/cli
./docker-build.sh
```

### Run Directly

```bash
cd devops/security/cli
go run main.go [command] [flags]
```

## Usage

### Basic Usage

```bash
./sonar-cli issues [sonar-url] [sonar-token] [project-key]
```

### Examples

#### Native Binary
```bash
# Query issues for a project
./sonar-cli issues https://sonar.example.com squ_abc123def456... my-project-key

# Specify custom output directory
./sonar-cli issues https://sonar.example.com squ_abc123def456... my-project-key --output-dir ./custom-reports

# Get help
./sonar-cli --help
./sonar-cli issues --help
```

#### Docker

The Docker container supports two modes of operation:

**1. Environment Variables Mode (Recommended)**
```bash
# Set environment variables in .env file or export them
export SONAR_HOST_URL=https://sonar.example.com
export SONAR_TOKEN=squ_abc123def456...
export SONAR_PROJECT_KEY=my-project-key

# Run with docker-compose (from devops/security directory)
docker-compose --profile tools run --rm sonar-cli

# Or run with Docker directly (using a named volume for reports)
docker run --rm \
  -e SONAR_HOST_URL=https://sonar.example.com \
  -e SONAR_TOKEN=squ_abc123def456... \
  -e SONAR_PROJECT_KEY=my-project-key \
  -v sonar-reports:/app/reports \
  sonar-cli:latest
```

**2. Command Arguments Mode**
```bash
# Run with explicit arguments using docker-compose
docker-compose --profile tools run --rm sonar-cli issues https://sonar.example.com squ_abc123def456... my-project-key

# Run with Docker directly (using a named volume for reports)
docker run --rm -v sonar-reports:/app/reports sonar-cli:latest issues https://sonar.example.com squ_abc123def456... my-project-key

# Get help with Docker
docker run --rm sonar-cli:latest --help
```

### Parameters

- **sonar-url**: The base URL of your SonarQube instance (e.g., `https://sonar.example.com`)
- **sonar-token**: Your SonarQube authentication token (starts with `squ_`)
- **project-key**: The key of the project in SonarQube

### Environment Variables (Docker)

When running in Docker without command arguments, the following environment variables are required:

- `SONAR_HOST_URL`: The base URL of your SonarQube instance (e.g., `https://sonar.example.com`)
- `SONAR_TOKEN`: Your SonarQube authentication token (starts with `squ_`)
- `SONAR_PROJECT_KEY`: The key of the project in SonarQube

### Flags

- `--output-dir, -o`: Output directory for reports (default: `./sonar-reports`)

## Output

The tool generates three types of reports:

### 1. JSON Report (`sonar-issues-{project}-{timestamp}.json`)
Complete raw data from SonarQube API including all issue details, metadata, and text ranges.

### 2. CSV Report (`sonar-issues-{project}-{timestamp}.csv`)
Structured data suitable for spreadsheet analysis with columns:
- key, type, severity, status, component, line, message, author, creationDate, updateDate, rule, project

### 3. Summary Report (`sonar-summary-{project}-{timestamp}.txt`)
Human-readable summary including:
- Total issue count
- Issues grouped by type (BUG, VULNERABILITY, CODE_SMELL)
- Issues grouped by severity (BLOCKER, CRITICAL, MAJOR, MINOR, INFO)
- Issues grouped by status (OPEN, CONFIRMED, REOPENED, RESOLVED, CLOSED)
- Top 10 components with most issues

## Authentication

The tool uses SonarQube's token-based authentication. You need to:

1. Generate a token in SonarQube (User > My Account > Security > Generate Tokens)
2. Use the token as the second parameter when running the CLI

## Error Handling

The tool provides detailed error messages for common issues:
- Invalid SonarQube URL or connection problems
- Authentication failures
- Project not found
- API errors and rate limiting

## Requirements

- Go 1.25.0 or later
- Network access to SonarQube instance
- Valid SonarQube authentication token
- Read permissions on the target project

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- Standard Go libraries for HTTP, JSON, CSV, and file operations
