---
description: 'Compare the current project against the latest go-stack scaffold template
  and propose or apply updates (tasks, linters, dependencies, skills, provider patterns,
  security tooling) while preserving project-specific business logic and module identity.

  '
---

# go-stack Project Update Prompt

You are helping update a Go project that was originally scaffolded from the **go-stack** template. The canonical upstream template lives in the `go-stack` repository at:

- **Template source:** `https://github.com/emilioforrer/go-stack/tree/main/internal/scaffold/template`

## Goal

Compare the files in the current project against the latest version of the upstream template and apply any updates, improvements, or bug fixes that have been added to the template since this project was scaffolded.

## How to compare

1. **Fetch the latest template files** from the upstream repository. The key files and directories to check are:
   - `Taskfile.yml`
   - `.golangci.yml`
   - `.goreleaser.yml`
   - `.air.toml`
   - `.mise.toml`
   - `apm.yml`
   - `apm.lock.yaml`
   - `.apm/` — APM prompts, agents, and local configuration
   - `go.mod` (structure and tools)
   - `boot/provider/` — provider patterns and lifecycle code
   - `cmd/app/` — CLI entry points, commands, and tests
   - `devops/` — DevOps, security, and CI/CD configurations
   - `.github/` — GitHub skills and workflows
   - `.claude/` — Claude skills
   - `.opencode/` — OpenCode skills

2. **Diff file by file** against the current project. Pay attention to:
   - New tasks added to `Taskfile.yml`
   - New or changed linter rules in `.golangci.yml`
   - Dependency version bumps in `go.mod` / `go.sum`
   - New files or directories in `boot/provider/` or `cmd/app/`
   - Changes in `devops/security/` scanning tools or Docker Compose services
   - Skill updates under `.github/skills/`, `.claude/skills/`, and `.opencode/skills/`

## Update rules

- **Preserve the project's module name and custom code.** Do NOT overwrite:
  - The `module` line in `go.mod`
  - Project-specific business logic, handlers, or providers
  - Custom flags or commands added after scaffolding
  - Custom `apm.yml` dependencies added by the project team

- **Bring in template improvements.** DO apply:
  - New `task` commands in `Taskfile.yml`
  - Linter rule changes or new exclusions in `.golangci.yml`
  - Updates to `devops/security/docker-compose.yml` or scanning scripts
  - New or updated skill files under `.github/skills/`, `.claude/skills/`, `.opencode/skills/`
  - New or updated APM prompts, agents, skills, or configuration under `.apm/`
  - Changes to `boot/provider/server.go` or similar shared provider patterns
  - Fixes to CLI command files (`cmd/app/*.go`) that are part of the scaffold

- **Handle `go.mod` carefully.**
  - Update `tool` directives and shared dependency versions if the template has bumped them.
  - Keep the project's own `require` and `replace` blocks intact.
  - Run `go mod tidy` after making changes.

- **Handle `go.sum` automatically.** After any `go.mod` change, regenerate `go.sum` with `go mod tidy`.

## Workflow

1. List the template files and compare them to the current project tree.
2. For each file that differs, show a concise diff summary (additions, removals, modifications).
3. Ask the user to confirm which changes they want to apply, or apply safe changes automatically (infrastructure, linter configs, shared tasks) and flag code-level changes for review.
4. After applying updates, run the project's own test and lint tasks to verify nothing breaks:
   ```bash
   task test
   task linter
   ```

## Important reminders

- The upstream template is the **source of truth** for scaffolding patterns, but the current project is the **source of truth** for its own business logic and module identity.
- Never blindly overwrite `cmd/app/main.go` or any file containing project-specific initialization without confirming with the user.
- If a new file exists in the template but not in the current project, propose adding it and explain its purpose.
- If a file was removed from the template, consider whether the project still needs it; if it was a scaffold-only file, suggest removing it.