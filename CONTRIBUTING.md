# Contributing to LLM Control Plane

Thank you for your interest in contributing to the LLM Control Plane! This document provides guidelines and standards for contributing to this project.

---

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Workflow](#development-workflow)
4. [Coding Standards](#coding-standards)
5. [Testing Requirements](#testing-requirements)
6. [Commit Guidelines](#commit-guidelines)
7. [Pull Request Process](#pull-request-process)
8. [Documentation](#documentation)

---

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors. We expect:

- **Respectful communication** in all interactions
- **Constructive feedback** focused on code, not people
- **Collaborative problem-solving** over individual ego
- **Recognition** that everyone is learning

### Unacceptable Behavior

- Harassment, discrimination, or personal attacks
- Trolling, insulting comments, or inflammatory language
- Publishing others' private information
- Any conduct that would be inappropriate in a professional setting

---

## Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.24+** installed ([installation guide](https://go.dev/doc/install))
- **Docker Desktop** running ([download](https://www.docker.com/products/docker-desktop))
- **Git** configured with your name and email
- **golangci-lint** installed (optional but recommended)

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/llm-control-plane.git
cd llm-control-plane
```

3. Add upstream remote:

```bash
git remote add upstream https://github.com/upb/llm-control-plane.git
```

4. Set up the development environment:

```bash
make setup
```

---

## Development Workflow

### 1. Create a Feature Branch

Always create a new branch for your work:

```bash
# Update main branch
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name
```

**Branch naming conventions:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test additions or modifications
- `chore/` - Maintenance tasks

### 2. Make Your Changes

Follow these principles:

- **Small, focused commits** - Each commit should do one thing
- **Test as you go** - Write tests alongside code
- **Follow conventions** - See [GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)
- **Keep it simple** - Prefer clarity over cleverness

### 3. Run Quality Checks

Before committing, ensure your code passes all checks:

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Run all checks
make check
```

### 4. Commit Your Changes

Write clear, descriptive commit messages:

```bash
git add .
git commit -m "feat: add JWT validation middleware"
```

See [Commit Guidelines](#commit-guidelines) below.

### 5. Push and Create PR

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create pull request on GitHub
```

---

## Coding Standards

### Go Conventions

**Required reading:** [docs/conventions/GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)

Key principles:

#### 1. Interface-First Design

Define interfaces before implementations:

```go
// Good: Interface defines contract
type PolicyEngine interface {
    Evaluate(ctx context.Context, req *Request) (*Decision, error)
}

// Implementation comes later
type engine struct {
    repo Repository
}

func (e *engine) Evaluate(ctx context.Context, req *Request) (*Decision, error) {
    // Implementation
}
```

#### 2. Explicit Error Handling

Never ignore errors; always wrap with context:

```go
// Good: Explicit error handling with context
user, err := repo.GetUser(ctx, id)
if err != nil {
    return nil, fmt.Errorf("failed to get user %s: %w", id, err)
}

// Bad: Ignored error
user, _ := repo.GetUser(ctx, id)
```

#### 3. Context Propagation

All I/O operations must accept `context.Context`:

```go
// Good: Context-aware
func ProcessRequest(ctx context.Context, req *Request) error {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    return service.Process(ctx, req)
}

// Bad: No context
func ProcessRequest(req *Request) error {
    return service.Process(req)
}
```

#### 4. Structured Logging

Use zap for structured logging:

```go
// Good: Structured with context
logger.Info(ctx, "processing request",
    zap.String("request_id", requestID),
    zap.String("user_id", userID),
    zap.Duration("duration", duration),
)

// Bad: Unstructured
log.Printf("Processing request %s for user %s took %v", requestID, userID, duration)
```

---

## Testing Requirements

### Test Coverage

- **Aim for 80%+ coverage** on critical paths
- **100% coverage** for security-sensitive code (auth, validation, policy)
- Focus on business logic, not trivial getters/setters

### Test Structure

Use table-driven tests:

```go
func TestValidatePrompt(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {
            name:    "valid prompt",
            input:   "Hello, world!",
            wantErr: false,
        },
        {
            name:    "contains PII",
            input:   "My email is user@example.com",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePrompt(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePrompt() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

Tag integration tests that require infrastructure:

```go
//go:build integration

package policy_test

func TestPolicyEngine_Integration(t *testing.T) {
    // Requires PostgreSQL and Redis
}
```

Run with: `make test-integration`

---

## Commit Guidelines

### Conventional Commits

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `style:` - Code style changes (formatting, no logic change)
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks (dependencies, build)
- `perf:` - Performance improvements
- `ci:` - CI/CD changes

**Examples:**

```bash
# Feature
git commit -m "feat(auth): add JWT validation middleware"

# Bug fix
git commit -m "fix(policy): correct rate limit calculation"

# Documentation
git commit -m "docs: update installation instructions"

# Breaking change
git commit -m "feat(api)!: change request format

BREAKING CHANGE: API now requires version header"
```

### Commit Message Guidelines

- **Subject line:**
  - Use imperative mood ("add" not "added" or "adds")
  - Capitalize first letter
  - No period at the end
  - Maximum 50 characters

- **Body (optional):**
  - Explain *what* and *why*, not *how*
  - Wrap at 72 characters
  - Separate from subject with blank line

- **Footer (optional):**
  - Reference issues: `Closes #123`
  - Note breaking changes: `BREAKING CHANGE: ...`

---

## Pull Request Process

### Before Creating PR

1. ✅ All tests pass: `make test`
2. ✅ Linter passes: `make lint`
3. ✅ Code is formatted: `make fmt`
4. ✅ Documentation updated (if needed)
5. ✅ Commit messages follow conventions
6. ✅ Branch is up to date with main

### PR Description Template

```markdown
## Summary
Brief description of changes (1-2 sentences)

## Changes
- Change 1
- Change 2
- Change 3

## Test Plan
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing performed

## Checklist
- [ ] Code follows conventions
- [ ] Tests pass
- [ ] Documentation updated
- [ ] No breaking changes (or documented)

## Related Issues
Closes #123
```

### Review Process

1. **Automated checks** must pass (CI/CD)
2. **At least one approval** required from maintainers
3. **Address feedback** promptly and professionally
4. **Squash commits** if requested (keep history clean)

### After Merge

1. Delete your feature branch
2. Update your local main branch
3. Close related issues (if not auto-closed)

---

## Documentation

### When to Update Documentation

Update documentation when you:

- Add new features or APIs
- Change existing behavior
- Add configuration options
- Introduce breaking changes
- Fix significant bugs

### Documentation Locations

| Type | Location |
|------|----------|
| API documentation | Code comments (GoDoc) |
| Architecture | `docs/architecture/` |
| Setup guides | `docs/setup/` |
| Conventions | `docs/conventions/` |
| Security | `docs/security/` |
| User guide | `README.md` |

### GoDoc Standards

All exported symbols must have doc comments:

```go
// PolicyEngine evaluates governance policies against incoming requests.
// It checks rate limits, cost caps, quotas, and model restrictions
// before allowing requests to proceed to LLM providers.
type PolicyEngine interface {
    // Evaluate checks if the request complies with all policies.
    // It returns a Decision indicating whether the request is allowed
    // and any violations that were detected.
    Evaluate(ctx context.Context, req *EvaluationRequest) (*Decision, error)
}
```

---

## Questions?

- **Documentation:** Check [docs/](docs/) directory
- **Quick reference:** See [QUICKSTART.md](QUICKSTART.md)
- **Conventions:** Read [GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)
- **Issues:** Open an issue on GitHub
- **Discussions:** Use GitHub Discussions for questions

---

## Recognition

Contributors will be recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Project documentation (with permission)

Thank you for contributing to LLM Control Plane! 🚀
