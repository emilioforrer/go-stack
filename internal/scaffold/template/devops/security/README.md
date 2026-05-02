# Security

## Run local SonarQube

```bash
docker-compose -f devops/security/docker-compose.yml --profile services up -d
```

Then go to `http://localhost:9000` and login with `admin/admin` and change the password.

After that, go to `http://127.0.0.1:9000/account/security` and generate a token and add to the `.env` file as `SONAR_TOKEN`.

### Dependency Check Plugin

The SonarQube setup includes the Dependency Check plugin which automatically scans for known vulnerabilities in project dependencies. The plugin is automatically downloaded and installed when you start the SonarQube service.

To verify the plugin is installed:
1. Go to `http://localhost:9000/admin/marketplace`
2. Look for "Dependency Check" in the installed plugins list

The plugin will automatically analyze dependencies during SonarQube scans and report vulnerabilities in the SonarQube dashboard.



## Run Dependency Check

Before running SonarQube scanner, you can generate a dependency check report:

```bash
./devops/security/run-dependency-check.sh
```

This will:
- Scan the project for known vulnerabilities in dependencies
- Generate reports in multiple formats (XML, HTML, JSON)
- Copy the XML report to the project root for SonarQube integration


## Run Trivy

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run trivy
```

This will scan the project for vulnerabilities, misconfigurations, and secrets, generating a SARIF report at `devops/security/trivy-reports/trivy-results.sarif`.

## Run Grype

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run grype
```

Grype is a vulnerability scanner for container images and filesystems. This will:
- Scan the entire project directory for known vulnerabilities
- Generate a SARIF format report at `devops/security/grype-reports/grype-results.sarif`
- Provide detailed vulnerability information including CVE details, severity levels, and fix recommendations

## Run Snyk

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run snyk
```

Snyk is a developer security platform that scans for vulnerabilities in dependencies and code. This will:
- Scan project dependencies for known vulnerabilities using `snyk test`
- Scan source code for security issues using `snyk code test`
- Generate both JSON and SARIF format reports in `devops/security/snyk-reports/`
- Provide detailed vulnerability information with fix recommendations

**Note:** You need to set the `SNYK_TOKEN` environment variable in your `.env` file. Get your token from [Snyk Account Settings](https://app.snyk.io/account).


## Run SonarQube Scanner

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run sonar-scanner
```

For a complete security scan including all security tools:

```bash
# 1. Run dependency check first
./devops/security/run-dependency-check.sh

# 2. Run all security scanners
docker-compose -f devops/security/docker-compose.yml --profile tools run trivy
docker-compose -f devops/security/docker-compose.yml --profile tools run grype
docker-compose -f devops/security/docker-compose.yml --profile tools run snyk

# 3. Finally run SonarQube scanner to aggregate all results
docker-compose -f devops/security/docker-compose.yml --profile tools run sonar-scanner
```

## Environment Variables

The following environment variables are used by the security tools and should be set in the `.env` file:

- `SONAR_TOKEN`: SonarQube authentication token
- `SNYK_TOKEN`: Snyk authentication token (get it from https://app.snyk.io/account)

## SonarQube CLI Tool

After read the README.md in `devops/security/cli/README.md`, you can run the CLI tool as follows:

```bash
./devops/security/cli/sonar-cli issues -o devops/security/sonar-reports $SONAR_HOST_URL $SONAR_TOKEN $SONAR_PROJECT_KEY
```

Or from docker-compose:

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run sonar-cli
```

## Agent CLI

The `cli-agents` service provides a confined terminal environment with a full set of CLI tools managed via [mise](https://mise.jdx.dev/). It runs as part of the `tools` profile and is built from `devops/security/cli-agents/`.

Inside the container you have access to:

- **AI coding agents**: [opencode](https://opencode.ai/), [Goose](https://block.github.io/goose/), [Crush](https://github.com/anthropics/crush), [Kilo Code](https://kilocode.ai/), [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [OpenAI Codex](https://platform.openai.com/docs/guides/codex)
- **Editors**: [Helix](https://helix-editor.com/), [Neovim](https://neovim.io/)
- **Terminal multiplexer**: [Zellij](https://zellij.dev/)

### Start the Agent CLI

```bash
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents bash
```

This drops you into an interactive bash shell inside the confined environment with your workspace mounted at `/workspace`.

### Run a specific tool directly

```bash
# Start opencode
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents opencode

# Start Goose
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents goose

# Start Crush
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents crush

# Start Kilo Code
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents kilo

# Start Claude Code
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents claude

# Start OpenAI Codex
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents codex

# Start Helix editor
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents hx

# Start Neovim editor
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents nvim

# Start Zellij session
docker-compose -f devops/security/docker-compose.yml --profile tools run cli-agents zellij
```

### Stop the Agent CLI

```bash
docker-compose -f devops/security/docker-compose.yml stop cli-agents
```