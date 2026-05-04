# Go Testing Guidelines: API-First TDD with Composable Internals

## Philosophy

> Start by designing the public API. Test that API first. Keep test structure consistent. Extract internal behavior into composed interfaces only when the public function grows too large. Test extracted components separately only when they contain meaningful logic.

Prefer a small public surface: **1 to 3 public methods**.

Do **not** start by testing private helpers or creating many internal abstractions. Start with the behavior expected from the public API.

---

## Standard Test Case Structure

All service-level tests must follow the same shape. Only the types change per feature.

```go
type <Feature>TestCase struct {
	name    string
	input   <InputType>
	wantErr bool
	verify  func(t *testing.T, input <InputType>, output <OutputType>)
	setup   func(t *testing.T, input <InputType>)
}
```

Example for a service returning `viewmodel.Response`:

```go
type SignupTestCase struct {
	name    string
	input   in.SignupInput
	wantErr bool
	verify  func(t *testing.T, input in.SignupInput, output viewmodel.Response[out.SignupViewModel])
	setup   func(t *testing.T, input in.SignupInput)
}
```

### Field Guidelines

| Field | Purpose |
|---|---|
| `name` | Scenario description in `snake_case`. Examples: `empty_input`, `invalid_email`, `weak_password`, `duplicate_email`, `success`, `repository_error`. Avoid `test_1`, `bad_case`, `works`. |
| `input` | Public API input. Clear and explicit; avoid hidden defaults. |
| `wantErr` | Whether the public API is expected to fail. Check with: `if (err != nil) != tt.wantErr { ... }`. |
| `verify` | Additional assertions **only after a successful call**. Use for returned data, generated IDs, normalized values, side effects, saved records, emitted events, notifications. Do **not** place the basic success/error assertion here. |
| `setup` | Preconditions only: creating existing data, preparing duplicate records, inserting reference data, configuring fake dependencies, preparing state. Do **not** perform assertions about the main test result here. |

---

## Standard Service Test Template

```go
func TestSignup(t *testing.T) {
	t.Parallel()

	svc := do.MustInvoke[*service.SignupService](injector)

	tests := []SignupTestCase{
		{
			name: "empty_input",
			input: in.SignupInput{Email: "", Password: ""},
			wantErr: true,
		},
		{
			name: "weak_password",
			input: in.SignupInput{Email: "user@example.com", Password: "123"},
			wantErr: true,
		},
		{
			name: "success",
			input: in.SignupInput{Email: "user_success@example.com", Password: "strong-password"},
			wantErr: false,
			verify: func(t *testing.T, input in.SignupInput, output viewmodel.Response[out.SignupViewModel]) {
				if output.Data.Email != input.Email {
					t.Errorf("got email %v, want %v", output.Data.Email, input.Email)
				}
				if output.Data.ID == "" {
					t.Error("expected user ID to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.setup != nil {
				tt.setup(t, tt.input)
			}

			output, err := svc.Signup(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Signup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.verify != nil {
				tt.verify(t, tt.input, output)
			}
		})
	}
}
```

Reuse this format for every feature. Adapt the types per feature.

---

## Recommended TDD Flow

### Phase 1: Define the public API

Before implementation, define the method the outside world should call.

```go
func (s *SignupService) Signup(input in.SignupInput) (viewmodel.Response[out.SignupViewModel], error)
```

Ask:
- What input does this feature need?
- What output does this feature return?
- What errors can happen?
- What does success look like?

### Phase 2: Write the first service-level tests

Start with **3 core test cases**:

1. `empty_input`
2. `one_important_business_rule_failure`
3. `success`

Do not overbuild the test suite at the beginning.

### Phase 3: Implement the simplest working service

The first implementation can be direct. Avoid unnecessary interfaces. Do not extract abstractions before there is enough complexity.

### Phase 4: Add more behavior tests

As rules are added, add more public API test cases. Examples: `invalid_email`, `duplicate_email`, `blocked_domain`, `repository_error`, `notification_error`.

### Phase 5: Extract components when the function grows

A public method is getting too large when it starts doing many things: validates input, checks duplicates, hashes passwords, creates domain models, saves data, sends notifications, publishes events, writes audit logs, handles multiple external errors.

When this happens, split using composition:

```go
type SignupService struct {
	validator      SignupValidator
	passwordHasher PasswordHasher
	userStore      UserStore
	notifier       Notifier
}
```

Then the public method becomes orchestration. The service is not responsible for every detail; it coordinates the flow.

### Phase 6: Test extracted components separately (only when meaningful)

**Even after extraction, keep the service-level tests.** They prove the feature works through the public API. Do not delete service tests just because internal components now have their own tests.

Only test extracted internals separately when they contain meaningful logic:

- **Test separately:** validators, password policies, pricing rules, permission rules, domain factories, mappers with non-trivial transformations, retry policies, notification formatters, business decision logic, etc.
- **Avoid testing separately:** tiny private helpers, one-line wrappers, simple pass-through methods, interfaces themselves, constructors with no logic, or something that is not a meaningful unit of behavior.

Use the same test case structure for extracted components. Only the output type changes.

### Phase 7: Add integration tests only where needed

Use integration tests for real external boundaries: database repositories, email providers, message brokers, file storage, HTTP clients.

Do **not** use integration tests as the first testing layer. Start with service behavior, then component behavior, then integration behavior.

Use **testcontainers** when possible to spin up real dependencies (databases, caches, message brokers) in isolated environments. This ensures tests run against actual implementations without requiring shared infrastructure or manual setup.

---

## Shared Test Infrastructure

The **Shared Test Infrastructure** section is good and practical. I would only make it more explicit about **file names and package scope**, because Go has conventions but not strict rules.

Common naming in Go:

| Purpose                                        | Common file name                                                | Notes                                                                                       |
| ---------------------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `TestMain` for one-time package setup/teardown | `main_test.go`                                                  | Very common when the package has a central test lifecycle.                                  |
| Test helpers                                   | `helper_test.go` or `helpers_test.go`                           | Both are common. I prefer `helpers_test.go` because it clearly means many helpers.          |
| Test fakes/stubs                               | `fake_test.go`, `fakes_test.go`, or `<dependency>_fake_test.go` | Example: `user_store_fake_test.go`.                                                         |
| Test builders/fixtures                         | `fixtures_test.go`, `builders_test.go`, or `testdata_test.go`   | Use when creating reusable inputs, entities, or expected outputs.                           |
| Package integration setup                      | `integration_test.go` or `testcontainers_test.go`               | Useful when the setup is specific to DB/testcontainers.                                     |
| Static test files                              | `testdata/` directory                                           | Go tooling recognizes `testdata` as a conventional folder ignored by normal package builds. |

For your document, I would slightly adjust the section like this:

````md
## Shared Test Infrastructure

When multiple test files in the same package need shared setup, such as database connections, testcontainers, dependency wiring, common fakes, builders, or reusable assertions, extract that setup into test-only files.

Common file names:

| Purpose | Recommended file name |
|---|---|
| One-time package test lifecycle using `TestMain` | `main_test.go` |
| General reusable helpers | `helpers_test.go` |
| Common fakes/stubs | `fakes_test.go` or `<dependency>_fake_test.go` |
| Test data builders / fixtures | `builders_test.go` or `fixtures_test.go` |
| Testcontainers setup | `testcontainers_test.go` |
| Static files used by tests | `testdata/` |

Use `main_test.go` only when the package needs a single `TestMain(m *testing.M)` for one-time setup and teardown.

Example:

```go
func TestMain(m *testing.M) {
	// start shared testcontainer
	// initialize shared pool / wire injector

	code := m.Run()

	// cleanup resources
	os.Exit(code)
}
````

Use `helpers_test.go` for reusable utilities that tests call explicitly:

```go
func newSignupServiceForTest(t *testing.T) *service.SignupService {
	t.Helper()

	// build fakes / dependencies / service
	return service.NewSignupService(...)
}
```

Guidelines:

* Keep `TestMain` focused on infrastructure lifecycle only.
* Do not put business logic in `TestMain`.
* Avoid global mutable state when possible.
* Prefer helper functions that each test calls explicitly.
* Use `t.Helper()` inside helper functions.
* Keep helpers in the same package when they are package-specific.
* Use `testdata/` for files required by tests.

Avoid global mutable state for business/test data. Shared package-level infrastructure is acceptable when it is initialized once in `TestMain`, treated as read-only by tests, and cleaned up after `m.Run()`.

Examples of acceptable shared package-level infrastructure:

- Database connection pool
- Testcontainer instance
- Redis/NATS/Postgres test dependency
- Dependency injector factory
- Logger/test configuration

Avoid sharing mutable business state:

- Reusing the same database rows across tests
- Reusing the same fake repository instance that accumulates data across tests
- Reusing mutable input/output structs between tests
- Letting tests depend on execution order

---

## Rules for AI Agents

1. **Start from the public API.** Do not create many internal abstractions immediately. First identify the public method.
2. **Use the standard test case struct.** Follow the `name`, `input`, `wantErr`, `verify`, `setup` shape consistently.
3. **Start with 3 test cases.** `empty_input`, one important business rule failure, `success`.
4. **Do not test private helpers first.** Only test them directly if they become complex, reused, bug-prone, or important business logic.
5. **Extract interfaces only when complexity appears.** Do not create interfaces just because they might be useful later.
6. **Keep service tests after refactoring.** Refactoring internals must not remove public behavior tests. The service-level test is the contract.
7. **Use `setup` for preconditions only.** Use `verify` for assertions after the method succeeds. Do not call the method again inside `verify`.

---

## Summary

```text
1. Design the public API.
2. Write service-level tests using the standard test case structure.
3. Implement the simplest version.
4. Add more public behavior tests as rules appear.
5. If the function grows too large, extract collaborators.
6. Test extracted collaborators only when they contain meaningful logic.
7. Keep the public API tests as the main behavior contract.
```

> Test behavior first. Refactor structure second. Test internals only when they become meaningful units of behavior.
