# Git Commit Instructions

## Commit Message Format

All commit messages MUST follow the [Conventional Commits](https://www.conventionalcommits.org) specification.

### Structure

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

| Type       | Description                                                                  |
|------------|------------------------------------------------------------------------------|
| `feat`     | A new feature                                                                |
| `fix`      | A bug fix                                                                    |
| `docs`     | Documentation only changes                                                   |
| `style`    | Changes that do not affect the meaning of the code (formatting, semicolons) |
| `refactor` | A code change that neither fixes a bug nor adds a feature                  |
| `perf`     | A code change that improves performance                                      |
| `test`     | Adding missing tests or correcting existing tests                            |
| `chore`    | Changes to the build process, auxiliary tools, or dependencies             |
| `ci`       | Changes to CI configuration files and scripts                                |
| `build`    | Changes that affect the build system or external dependencies                |
| `revert`   | Reverts a previous commit                                                    |

### Rules

1. **Type is required** — Choose the most appropriate type from the list above.
2. **Scope is optional** — Use a scope to indicate the area of the codebase affected (e.g., `feat(api):`, `fix(auth):`).
3. **Description is required** — Use the imperative, present tense (e.g., "change" not "changed" or "changes").
4. **No period at the end** of the description line.
5. **Body is optional** — Use it to explain *what* and *why*, not *how*.
6. **Footer is optional** — Use for referencing issues, breaking changes, or co-authors.
7. **Breaking changes** — Append `!` after the type/scope or include `BREAKING CHANGE:` in the footer.

### Examples

```
feat: add user authentication
```

```
feat(api)!: remove deprecated endpoints

BREAKING CHANGE: The /v1/users endpoint has been removed. Use /v2/users instead.
```

```
fix(parser): handle empty input strings

Previously, passing an empty string caused a null pointer exception.
Now it returns an empty list as expected.

Closes #123
```

```
docs(readme): update installation instructions
```

```
chore(deps): bump lodash from 4.17.20 to 4.17.21
```

## Enforcement

- Always suggest commit messages in Conventional Commits format.
- If the user provides a non-conforming message, suggest a corrected version.
- When summarizing changes, categorize them by type (feat, fix, docs, etc.).