# LLM Control Plane

**Enterprise-grade governance middleware for Large Language Model operations**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](backend/docker-compose.yml)

---

## 📋 Overview

The **LLM Control Plane** is a stateless, cloud-native middleware platform that sits between your applications and multiple LLM providers (OpenAI, Anthropic, Azure OpenAI). It enforces enterprise governance policies, provides intelligent routing, and ensures comprehensive observability for all LLM operations.

### Purpose

Act as a deterministic control plane that:
- **Authenticates** requests using JWT/OIDC
- **Validates** prompts for PII, secrets, and injection attempts
- **Evaluates** policies (rate limits, cost caps, quotas, model restrictions)
- **Routes** requests to optimal providers with fallback support
- **Audits** all operations with structured logging and metrics
- **Observes** system behavior through Prometheus metrics and distributed tracing

### What This Is NOT

- ❌ Not an AI application or chatbot
- ❌ Not a RAG (Retrieval-Augmented Generation) system
- ❌ Not a UI/frontend (separate concern)
- ❌ Not provider-specific (unified interface for all LLMs)

---

## 🏗️ Architecture

### Request Pipeline

```
Client Request
    ↓
┌─────────────────────────────────────────────────┐
│  1. Authentication (JWT Validation)             │
│     ↓                                           │
│  2. Prompt Validation (PII, Secrets, Injection) │
│     ↓                                           │
│  3. Policy Evaluation (Rate Limits, Cost Caps)  │
│     ↓                                           │
│  4. Model Routing (Provider Selection)          │
│     ↓                                           │
│  5. LLM Provider Call (OpenAI/Anthropic/Azure)  │
│     ↓                                           │
│  6. Audit & Metrics (Logging, Prometheus)       │
└─────────────────────────────────────────────────┘
    ↓
Response to Client
```

### Key Tenets

- **Cloud-Native:** Stateless services following 12-factor app principles
- **Deterministic:** Predictable pipeline orchestration with explicit error handling
- **Observable:** Structured logging (zap), Prometheus metrics, distributed tracing
- **Secure:** Zero-trust approach to LLM outputs; validate, govern, audit everything
- **Scalable:** Horizontal scaling with PostgreSQL for persistence

---

## 📁 Repository Structure

```
llm-control-plane/
├── backend/
│   ├── cmd/
│   │   └── api-gateway/          # Main application entrypoint
│   ├── docker-compose.yml       # Local development infrastructure
│   └── internal/                 # Private application code
│       ├── auth/                 # Authentication & authorization (JWT, RBAC)
│       ├── policy/               # Policy evaluation engine
│       ├── routing/              # Intelligent model routing
│       ├── providers/            # LLM provider adapters (OpenAI, Anthropic, Azure)
│       ├── observability/        # Logging, metrics, tracing
│       ├── runtimeconfig/        # Dynamic configuration management
│       ├── rag/                  # RAG hooks (Phase 2+)
│       ├── audit/                # Audit logging
│       ├── prompt/               # Prompt validation (PII, injection detection)
│       ├── storage/              # Data persistence (PostgreSQL)
│       └── shared/               # Shared utilities
│
├── docs/
│   ├── architecture/             # C4 diagrams, sequence diagrams, decisions
│   ├── conventions/              # Coding standards (GO_CONVENTIONS.md)
│   ├── setup/                    # Setup guides and status tracking
│   └── security/                 # OWASP LLM Top 10, threat models
│
├── frontend/                     # React admin UI (separate)
├── go.mod                        # Go module definition
├── Makefile                      # Build automation
└── .golangci.yml                 # Linter configuration
```

## 🚀 Quick Start

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/doc/install) | 1.24+ | Backend runtime |
| [Docker Desktop](https://www.docker.com/products/docker-desktop) | Latest | Local infrastructure |
| [OpenSSL](https://www.openssl.org/) | Latest | HTTPS certificate generation |
| [AWS CLI](https://aws.amazon.com/cli/) | Latest | Cognito user management |
| Make (or PowerShell) | Any | Build automation |

### Two Deployment Options

#### Option 1: Automated Cloud Deployment (Recommended)

Deploy full infrastructure (Cognito, Aurora, Lambda, API Gateway, CloudFront) using GitHub Actions:

```bash
# 1. Set up GitHub secrets (AWS_ROLE_ARN)
# See docs/setup/COGNITO_DEPLOYMENT.md for OIDC setup

# 2. Push to main branch (triggers sandbox deployment)
git push origin main

# 3. Monitor workflow at:
# https://github.com/your-org/llm-control-plane/actions

# 4. Get Cognito credentials from workflow outputs
# Copy to backend/.env for local development
```

#### Option 2: Local Development Only

Set up local development environment with existing Cognito credentials:

```bash
# 1. Clone the repository
git clone https://github.com/upb/llm-control-plane.git
cd llm-control-plane

# 2. Complete initial setup (deps, infra, migrations, certs)
make setup

# 3. Configure Cognito credentials
copy backend\.env.example backend\.env
# Edit backend/.env with your Cognito credentials

# 4. Start development servers with HTTPS
make dev
```

The servers will start:
- **Backend (HTTPS):** https://localhost:8443
- **Frontend:** https://localhost:5173
- **Auth Login:** https://localhost:8443/auth/login

**Note:** Accept self-signed certificate warnings in your browser.

### Verify Installation

```bash
# Health check (HTTPS)
curl -k https://localhost:8443/healthz
# Expected: {"status":"ok"}

# Readiness check (validates database connection)
curl -k https://localhost:8443/readyz
# Expected: {"status":"ready","checks":{...}}

# Check infrastructure
docker compose ps
# Expected: database (healthy)

# Test OAuth2 flow
# Open in browser: https://localhost:8443/auth/login
# Should redirect to Cognito hosted UI

# Run tests
make test
```

For detailed setup instructions:
- **[Cognito Deployment Guide](docs/setup/COGNITO_DEPLOYMENT.md)** - Automated deployment & Cognito setup
- **[Local Development Guide](docs/setup/LOCAL_DEVELOPMENT.md)** - Detailed local environment setup

---

## 🛠️ Development

### Common Commands

```bash
# Development
make dev              # Run development server
make build            # Build production binary
make build-lambda     # Build AWS Lambda package

# Testing
make test             # Run all tests
make test-coverage    # Generate coverage report
make test-integration # Run integration tests

# Code Quality
make fmt              # Format code
make lint             # Run linter
make check            # Run all quality checks

# Infrastructure
make infra-up         # Start PostgreSQL
make infra-down       # Stop infrastructure
make infra-reset      # Reset all data (destructive!)
make infra-logs       # View logs

# Database
make db-shell         # Open PostgreSQL shell
make db-migrate       # Run migrations (TODO)
make db-seed          # Seed test data (TODO)
```

Run `make help` for a complete list of commands.

### Configuration

Configuration is managed through environment variables defined in `backend/.env` file.

**Setup:**

1. Copy the example environment file:
   ```bash
   cd backend
   cp .env.example .env
   ```

2. Update `.env` with your credentials (get from GitHub Actions workflow outputs or AWS Console):
   ```bash
   # Server Configuration (HTTPS for OAuth2)
   PORT=8443
   TLS_ENABLED=true
   TLS_CERT_FILE=certs/cert.pem
   TLS_KEY_FILE=certs/key.pem
   
   # AWS Cognito Configuration
   COGNITO_USER_POOL_ID=us-east-1_xxxxx     # From deployment
   COGNITO_CLIENT_ID=xxxxx                   # From deployment
   COGNITO_CLIENT_SECRET=xxxxx               # From AWS Console/Secrets
   COGNITO_DOMAIN=https://auth-sandbox-llm-cp.auth.us-east-1.amazoncognito.com
   COGNITO_REDIRECT_URI=https://localhost:8443/auth/callback
   AWS_REGION=us-east-1
   
   # Frontend URL
   FRONT_END_URL=https://localhost:5173
   
   # LLM Provider Keys
   OPENAI_API_KEY=sk-...                    # Your OpenAI API key
   ```

**Key Configuration Variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8443` | HTTPS server port (changed from 8080) |
| `TLS_ENABLED` | `true` | Enable HTTPS (required for Cognito) |
| `TLS_CERT_FILE` | `certs/cert.pem` | Path to TLS certificate |
| `TLS_KEY_FILE` | `certs/key.pem` | Path to TLS private key |
| `COGNITO_USER_POOL_ID` | `` | AWS Cognito User Pool ID (required) |
| `COGNITO_CLIENT_ID` | `` | AWS Cognito Client ID (required) |
| `COGNITO_CLIENT_SECRET` | `` | AWS Cognito Client Secret (required) |
| `COGNITO_DOMAIN` | `` | Cognito hosted UI domain |
| `COGNITO_REDIRECT_URI` | `https://localhost:8443/auth/callback` | OAuth2 callback URL |
| `FRONT_END_URL` | `https://localhost:5173` | Frontend URL for redirects |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `OPENAI_API_KEY` | `` | OpenAI API key (required) |

See `backend/.env.example` for all available options.

**Getting Cognito Credentials:**

```powershell
# Option 1: From GitHub Actions workflow outputs
# Navigate to: Actions → Latest workflow run → Deploy Infrastructure

# Option 2: Using helper script
.\scripts\get-cognito-info.ps1 -UserPoolId us-east-1_xxxxx

# Option 3: AWS Console
# Navigate to: Cognito → User Pools → Your Pool → App Integration
```

**For production:** Use AWS Secrets Manager, AWS Systems Manager Parameter Store, or Kubernetes secrets instead of `.env` files.

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| **[COGNITO_DEPLOYMENT.md](docs/setup/COGNITO_DEPLOYMENT.md)** | **NEW:** Automated AWS deployment & Cognito setup guide |
| **[LOCAL_DEVELOPMENT.md](docs/setup/LOCAL_DEVELOPMENT.md)** | Comprehensive local development setup |
| **[QUICKSTART.md](QUICKSTART.md)** | Quick reference for common tasks |
| **[GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)** | Coding standards and best practices (664 lines) |
| **[GROWTH_GUIDELINES.md](docs/conventions/GROWTH_GUIDELINES.md)** | Scaling and extension guidelines |
| **[ARCHITECTURE.md](docs/approach/ARCHITECTURE.md)** | Detailed system architecture |
| **[BOOTSTRAP_STATUS.md](docs/setup/BOOTSTRAP_STATUS.md)** | Project setup status and progress |
| **[SETUP_COMPLETE.md](SETUP_COMPLETE.md)** | Verification checklist |

### Helper Scripts

| Script | Description |
|--------|-------------|
| `scripts/generate-certs.ps1` | Generate self-signed HTTPS certificates |
| `scripts/create-test-user.ps1` | Create Cognito test users with custom attributes |
| `scripts/get-cognito-info.ps1` | Fetch Cognito credentials from AWS |
| `scripts/list-test-users.ps1` | List all users in Cognito User Pool |

---

## 🧪 Testing

The project includes comprehensive unit and integration tests.

### Running Tests

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage

# Run integration tests only
make test-integration

# Run tests in verbose mode
go test -v ./...

# Run specific test
go test -v ./backend/services/policy -run TestPolicyService

# Skip integration tests (faster)
go test -short ./...
```

### Test Categories

| Category | Location | Description |
|----------|----------|-------------|
| **Unit Tests** | `*_test.go` files | Test individual functions and methods |
| **Integration Tests** | `backend/tests/integration/` | Test full pipeline with real dependencies |
| **Repository Tests** | `backend/repositories/postgres/*_test.go` | Test database operations |
| **Service Tests** | `backend/services/*_test.go` | Test business logic |
| **Handler Tests** | `backend/handlers/*_test.go` | Test HTTP handlers |

### Integration Test Requirements

Integration tests use **testcontainers** to spin up real PostgreSQL instances:

- Docker must be running
- Tests automatically start and stop containers
- No manual setup required
- Use `-short` flag to skip: `go test -short ./...`

### Coverage Goals

- **Core Pipeline:** 80%+ coverage
- **Business Logic:** 75%+ coverage
- **HTTP Handlers:** 70%+ coverage
- **Overall:** 70%+ coverage

```bash
# Generate HTML coverage report
make test-coverage
open coverage.html
```

---

## 🔧 Troubleshooting

### Common Issues

**Problem:** `connection refused` when starting the server

**Solution:**
```bash
# Ensure infrastructure is running
make infra-up

# Check container status
docker compose ps

# View logs
make infra-logs
```

---

**Problem:** Database migration errors

**Solution:**
```bash
# Reset database (WARNING: destroys all data)
make infra-reset

# Or manually recreate
make infra-down
make infra-up
make db-migrate
```

---

**Problem:** Database connection errors

**Solution:**
```bash
# Test PostgreSQL connection
docker exec -it llm-cp-postgres pg_isready -U dev -d audit
# Expected: audit:5432 - accepting connections
```

---

**Problem:** `go.mod` dependency issues

**Solution:**
```bash
# Clean and reinstall dependencies
go clean -modcache
go mod download
go mod tidy
```

---

**Problem:** Port 8443 already in use

**Solution:**
```bash
# Find process using port 8443 (PowerShell)
Get-NetTCPConnection -LocalPort 8443

# Or change port in .env
PORT=8444
```

---

**Problem:** HTTPS certificate warnings in browser

**Solution:**
This is expected with self-signed certificates. To proceed:
1. In browser, click "Advanced" or "Show Details"
2. Click "Proceed to localhost (unsafe)" or similar
3. For Chrome: type `thisisunsafe` on the warning page

To generate new certificates:
```powershell
.\scripts\generate-certs.ps1
```

---

**Problem:** AWS Cognito authentication failing

**Solution:**
1. Verify credentials in `backend/.env`:
   ```env
   COGNITO_USER_POOL_ID=us-east-1_xxxxx
   COGNITO_CLIENT_ID=xxxxx
   COGNITO_CLIENT_SECRET=xxxxx
   COGNITO_DOMAIN=https://auth-sandbox-llm-cp.auth.us-east-1.amazoncognito.com
   COGNITO_REDIRECT_URI=https://localhost:8443/auth/callback
   AWS_REGION=us-east-1
   ```

2. Test AWS credentials:
   ```powershell
   aws cognito-idp describe-user-pool --user-pool-id <POOL_ID>
   ```

3. Check redirect URI matches exactly in Cognito console

4. See **[COGNITO_DEPLOYMENT.md](docs/setup/COGNITO_DEPLOYMENT.md)** for detailed troubleshooting

---

**Problem:** "Invalid state" error during OAuth callback

**Solution:**
This is CSRF protection. Common causes:
1. Cookies disabled in browser → Enable cookies
2. Different domain/port → Ensure using https://localhost:8443
3. Stale state → Clear browser cookies and try again

---

**Problem:** Integration tests failing

**Solution:**
```bash
# Ensure Docker is running
docker ps

# Run integration tests with verbose output
go test -v ./backend/tests/integration/

# Skip integration tests
go test -short ./...
```

---

**Problem:** High memory usage

**Solution:**
```bash
# Check database connection pool settings in .env
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Check database pool settings
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNS=2
```

---

For more troubleshooting, see **[Local Development Guide](docs/setup/LOCAL_DEVELOPMENT.md)**.

---

## 🔒 Security & Governance

### Core Principles

- **Zero-Trust LLMs:** Treat all LLM outputs as untrusted; validate, sanitize, and audit
- **Defense in Depth:** Multiple validation layers (auth → prompt → policy → routing)
- **PII Protection:** Detect and block personally identifiable information in prompts
- **Secrets Detection:** Prevent API keys, passwords, and credentials in requests
- **Injection Prevention:** Guard against prompt injection attacks
- **RBAC Enforcement:** Role-Based Access Control before any provider calls
- **Audit Everything:** Comprehensive logging without exposing secrets or PII

### Compliance

- **OWASP LLM Top 10:** See [docs/security/owasp-llm.md](docs/security/owasp-llm.md)
- **Threat Model:** See [docs/security/threat-model.md](docs/security/threat-model.md)
- **Dependency Security:** All dependencies pinned to specific versions

---

## 📊 Project Status

### ✅ Phase 1: Core Infrastructure (Complete)

| Component | Status | Description |
|-----------|--------|-------------|
| Repository Structure | ✅ | Domain-driven design with clean architecture |
| Build System | ✅ | Makefile with 30+ automation targets |
| Database Layer | ✅ | PostgreSQL with repository pattern |
| Cache Layer | 🔜 | Planned for future (in-memory for now) |
| Configuration | ✅ | Environment-based config with validation |
| Logging | ✅ | Structured logging with zap |
| Testing Framework | ✅ | Unit and integration test infrastructure |

### ✅ Phase 2: Domain Implementation (Complete)

| Component | Status | Description |
|-----------|--------|-------------|
| Models | ✅ | Organization, Application, User, Policy, AuditLog, InferenceRequest |
| Repositories | ✅ | PostgreSQL implementations with transactions |
| Services | ✅ | Audit, Budget, RateLimit, Policy, Prompt, Routing, Inference |
| Handlers | ✅ | HTTP handlers with validation and error handling |
| Middleware | ✅ | Auth, Policy, Context propagation |
| Provider Integration | ✅ | OpenAI adapter with mock support |

### ✅ Phase 3: Pipeline & Testing (Complete)

| Component | Status | Description |
|-----------|--------|-------------|
| Request Pipeline | ✅ | Auth → Validation → Policy → Routing → Provider → Audit |
| Prompt Validation | ✅ | PII detection, secrets detection, injection guard |
| Policy Engine | ✅ | Rate limiting, cost limits, model restrictions |
| Integration Tests | ✅ | Full pipeline tests with testcontainers |
| API Tests | ✅ | HTTP endpoint tests with mocked dependencies |
| Documentation | ✅ | README, Local Development Guide, API docs |

### ✅ Phase 4: Deployment & Authentication (Complete)

| Component | Status | Description |
|-----------|--------|-------------|
| GitHub Actions Workflows | ✅ | Automated deployment (infra.yml, on-push.yml, on-tags-rc.yml) |
| AWS Cognito Integration | ✅ | OAuth2 authentication with hosted UI |
| HTTPS Support | ✅ | TLS certificates for local development |
| OAuth2 Handlers | ✅ | Login, callback, logout flows |
| Auth Middleware | ✅ | JWT validation with cookie/header support |
| Helper Scripts | ✅ | User management, credential fetching, cert generation |
| Deployment Docs | ✅ | Comprehensive Cognito deployment guide |

**Deliverables:**
- ✅ Repository structure with domain-driven design
- ✅ Go 1.24+ module with 40+ dependencies
- ✅ Local development infrastructure (Docker Compose)
- ✅ Enterprise-grade build automation (Makefile)
- ✅ Comprehensive linting (golangci-lint with 20+ rules)
- ✅ Coding conventions (664-line standards document)
- ✅ 7 domain packages with complete implementations
- ✅ PostgreSQL schema with migrations
- ✅ PostgreSQL-based rate limiting
- ✅ OpenAI provider adapter
- ✅ Full request pipeline with validation
- ✅ 100+ unit tests
- ✅ 18+ integration test cases
- ✅ Comprehensive documentation

### 🚧 Phase 4: Advanced Features (Next)

- Additional provider adapters (Anthropic, AWS Bedrock)
- Advanced routing strategies (cost optimization, A/B testing)
- Prometheus metrics and Grafana dashboards
- Distributed tracing with OpenTelemetry
- AWS Lambda deployment automation

### 📋 Roadmap

**Phase 1: Core Pipeline (Current)**
- Implement authentication middleware (JWT validation)
- Build policy engine with rate limiting
- Add prompt validation (PII, secrets, injection detection)
- Create provider adapters with fallback support
- Implement audit logging to PostgreSQL

**Phase 2: Advanced Features**
- RAG integration (vector database)
- Cost-optimized routing
- A/B testing framework
- Multi-tenancy (database-per-tenant)
- Prompt template library

**Phase 3: Production Readiness**
- AWS Lambda deployment
- CloudWatch dashboards and alarms
- Datadog APM integration
- Load testing (1000+ req/sec)
- Production deployment pipeline

---

## 🤝 Contributing

### Development Workflow

1. **Fork and clone** the repository
2. **Create a feature branch:** `git checkout -b feature/your-feature`
3. **Follow conventions:** See [GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)
4. **Write tests:** Aim for 80%+ coverage on critical paths
5. **Run quality checks:** `make check`
6. **Commit with clear messages:** Follow conventional commits
7. **Push and create PR:** Include description and test plan

### Code Standards

- **Interface-first design:** Define interfaces before implementations
- **Explicit error handling:** No panics in production code
- **Context propagation:** All I/O operations accept `context.Context`
- **Structured logging:** Use zap with contextual fields
- **Table-driven tests:** Prefer table-driven tests for multiple cases

See [GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md) for complete standards.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- **Architecture patterns** inspired by [12-Factor App](https://12factor.net/) and cloud-native principles
- **Security guidelines** based on [OWASP LLM Top 10](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- **Go conventions** adapted from [Uber Go Style Guide](https://github.com/uber-go/guide) and [Effective Go](https://go.dev/doc/effective_go)

---

## 📞 Support

- **Documentation:** See [docs/](docs/) directory
- **Issues:** Open an issue on GitHub
- **Questions:** Check [QUICKSTART.md](QUICKSTART.md) first

---

**Built with ❤️ for enterprise LLM governance** 

