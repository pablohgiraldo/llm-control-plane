# LLM Control Plane

**Enterprise-grade governance middleware for Large Language Model operations**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](docker-compose.yml)

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
- **Scalable:** Horizontal scaling with Redis for rate limiting and PostgreSQL for persistence

---

## 📁 Repository Structure

```
llm-control-plane/
├── backend/
│   ├── cmd/
│   │   └── api-gateway/          # Main application entrypoint
│   │
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
│       ├── storage/              # Data persistence (PostgreSQL, Redis)
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
├── docker-compose.yml            # Local development infrastructure
└── .golangci.yml                 # Linter configuration
```

## 🚀 Quick Start

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/doc/install) | 1.24+ | Backend runtime |
| [Docker Desktop](https://www.docker.com/products/docker-desktop) | Latest | Local infrastructure |
| Make | Any | Build automation |

### Installation

```bash
# 1. Clone the repository
git clone https://github.com/upb/llm-control-plane.git
cd llm-control-plane

# 2. Download dependencies
go mod download

# 3. Start local infrastructure (PostgreSQL + Redis)
docker compose up -d

# 4. Run development server
make dev
```

The API gateway will start on `http://localhost:8080`

### Verify Installation

```bash
# Health check
curl http://localhost:8080/healthz
# Expected: "ok"

# Check infrastructure
docker compose ps
# Expected: postgres (healthy), redis (healthy)

# Run tests
make test
```

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
make infra-up         # Start PostgreSQL + Redis
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

Configuration is managed through environment variables. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | HTTP server address |
| `DATABASE_URL` | `postgresql://dev:dev@localhost:5432/...` | PostgreSQL connection |
| `REDIS_URL` | `localhost:6379` | Redis connection |
| `REDIS_PASSWORD` | `dev` | Redis password |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |

See `backend/internal/runtimeconfig/types.go` for all available options.

**For production:** Use AWS Secrets Manager or Kubernetes secrets instead of environment variables.

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| **[QUICKSTART.md](QUICKSTART.md)** | Quick reference for common tasks |
| **[GO_CONVENTIONS.md](docs/conventions/GO_CONVENTIONS.md)** | Coding standards and best practices (664 lines) |
| **[GROWTH_GUIDELINES.md](docs/conventions/GROWTH_GUIDELINES.md)** | Scaling and extension guidelines |
| **[ARCHITECTURE.md](docs/approach/ARCHITECTURE.md)** | Detailed system architecture |
| **[BOOTSTRAP_STATUS.md](docs/setup/BOOTSTRAP_STATUS.md)** | Project setup status and progress |
| **[SETUP_COMPLETE.md](SETUP_COMPLETE.md)** | Verification checklist |

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

### ✅ Completed (HU-03: Enterprise Repository Bootstrap)

| Subtask | Status | Description |
|---------|--------|-------------|
| #1 | ✅ | Base repository structure |
| #2 | ✅ | Go modules and conventions configured |
| #3 | ✅ | Domain package placeholders created |
| #4 | 🚧 | README documentation (this file) |

**Deliverables:**
- ✅ Repository structure with domain-driven design
- ✅ Go 1.24+ module with 40+ dependencies
- ✅ Local development infrastructure (Docker Compose)
- ✅ Enterprise-grade build automation (Makefile)
- ✅ Comprehensive linting (golangci-lint with 20+ rules)
- ✅ Coding conventions (664-line standards document)
- ✅ 7 domain packages with interface definitions

### 🚧 In Progress

- Core pipeline implementation
- Database migrations and schema
- Provider adapters (OpenAI, Anthropic, Azure)

### 📋 Roadmap

**Phase 1: Core Pipeline (Current)**
- Implement authentication middleware (JWT validation)
- Build policy engine with Redis rate limiting
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

