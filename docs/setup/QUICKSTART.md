# Quick Start Guide

**LLM Control Plane - Enterprise Repository**

---

## 🚀 First Time Setup

### 1. Install Prerequisites

#### Go 1.24+
```bash
# Windows (using Chocolatey)
choco install golang

# Or download from: https://go.dev/doc/install

# Verify installation
go version
```

#### Docker Desktop
```bash
# Download from: https://www.docker.com/products/docker-desktop

# Verify installation
docker --version
docker compose version
```

#### golangci-lint (optional but recommended)
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

### 2. Initialize Project

```bash
# Navigate to project
cd llm-control-plane

# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Tidy up
go mod tidy

# Verify build
go build -v ./...
```

---

### 3. Start Local Infrastructure

```bash
# Start PostgreSQL and Redis
make infra-up

# Verify services are running
make infra-status

# Expected output:
# - llm-cp-postgres (healthy)
# - llm-cp-redis (healthy)
```

---

### 4. Run Development Server

```bash
# Start the API gateway
make dev

# Server will start on http://localhost:8080
# Test health endpoint:
curl http://localhost:8080/healthz
```

---

## 📋 Common Commands

### Development
```bash
make dev              # Run development server
make build            # Build production binary
make build-lambda     # Build AWS Lambda package
make test             # Run tests
make test-coverage    # Run tests with coverage report
make lint             # Run code quality checks
make check            # Run all quality checks (fmt, vet, lint, test)
```

### Infrastructure
```bash
make infra-up         # Start PostgreSQL + Redis
make infra-down       # Stop infrastructure
make infra-reset      # Reset all data (destructive!)
make infra-logs       # Show infrastructure logs
make infra-status     # Show service status
```

### Database
```bash
make db-migrate       # Run database migrations (TODO)
make db-seed          # Seed test data (TODO)
make db-reset         # Drop and recreate database
make db-shell         # Open PostgreSQL shell
```

### Utilities
```bash
make help             # Show all available commands
make version          # Show version information
make clean            # Remove build artifacts
make clean-all        # Clean + stop infrastructure
```

---

## 🏗️ Project Structure

```
llm-control-plane/
├── backend/
│   ├── cmd/
│   │   └── api-gateway/          # Main application entry
│   │
│   └── internal/                 # Private application code
│       ├── auth/                 # Authentication & authorization
│       ├── policy/               # Policy evaluation engine
│       ├── routing/              # Model routing logic
│       ├── providers/            # LLM provider adapters
│       ├── observability/        # Logging, metrics, tracing
│       ├── runtimeconfig/        # Configuration management
│       └── rag/                  # RAG hooks (Phase 2+)
│
├── docs/
│   ├── conventions/              # Coding standards
│   ├── architecture/             # Architecture docs
│   └── setup/                    # Setup guides
│
├── go.mod                        # Go module definition
├── Makefile                      # Build automation
└── docker-compose.yml            # Local infrastructure
```

---

## 📚 Key Documentation

- **[GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)** - Coding standards and best practices
- **[BOOTSTRAP_STATUS.md](docs/setup/BOOTSTRAP_STATUS.md)** - Current project status
- **[ARCHITECTURE.md](docs/approach/ARCHITECTURE.md)** - System architecture
- **[README.md](README.md)** - Project overview

---

## 🔧 Configuration

Configuration is managed through environment variables. For local development:

1. **Database:** `postgresql://dev:dev@localhost:5432/llm_control_plane_dev`
2. **Redis:** `localhost:6379` (password: `dev`)
3. **HTTP Server:** `:8080`

See `backend/internal/runtimeconfig/types.go` for all configuration options.

---

## 🧪 Testing

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Generate coverage report
make test-coverage
# Opens: coverage/coverage.html

# Run integration tests (requires infrastructure)
make test-integration
```

---

## 🔍 Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter
make lint

# Run all checks
make check
```

---

## 🐛 Troubleshooting

### "go: command not found"
- Install Go 1.24+ from https://go.dev/doc/install
- Ensure Go is in your PATH

### "docker: command not found"
- Install Docker Desktop from https://www.docker.com/products/docker-desktop
- Ensure Docker daemon is running

### Database connection errors
```bash
# Check if PostgreSQL is running
make infra-status

# View logs
make infra-logs

# Reset infrastructure
make infra-reset
```

### Port already in use
```bash
# Check what's using port 8080
netstat -ano | findstr :8080

# Or change port in environment
export HTTP_ADDR=:8081
make dev
```

---

## 🚢 Deployment

### Build for AWS Lambda
```bash
make build-lambda
# Output: bin/lambda.zip
```

### CI/CD
```bash
# Run CI test suite
make ci-test

# Build for deployment
make ci-build
```

---

## 📖 Learning Resources

### Project Documentation
- [GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md) - Coding standards (664 lines)
- [GROWTH_GUIDELINES.md](docs/conventions/GROWTH_GUIDELINES.md) - Scaling and extension patterns
- [ARCHITECTURE.md](docs/approach/ARCHITECTURE.md) - System architecture
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines

### Go
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)

### Architecture
- [12-Factor App](https://12factor.net/)
- [Cloud Native Patterns](https://www.oreilly.com/library/view/cloud-native-patterns/9781617294297/)
- [Domain-Driven Design](https://martinfowler.com/bliki/DomainDrivenDesign.html)

---

## 🆘 Getting Help

1. Check documentation in `docs/`
2. Run `make help` for available commands
3. Review `docs/conventions/GO_CONVENTIONS.md` for coding standards
4. Check `docs/setup/BOOTSTRAP_STATUS.md` for project status

---

## ✅ Next Steps

After completing setup:

1. ✅ Install prerequisites (Go, Docker)
2. ✅ Run `make setup`
3. ✅ Verify with `make test`
4. 🚧 Implement core pipeline (Subtask #2)
5. 🚧 Add database migrations
6. 🚧 Implement provider adapters
7. 🚧 Build policy engine

See `docs/setup/BOOTSTRAP_STATUS.md` for detailed status.
