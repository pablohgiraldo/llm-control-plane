# Local Development Guide

Complete guide for setting up and running the LLM Control Plane locally.

---

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Initial Setup](#initial-setup)
3. [Configuration](#configuration)
4. [AWS Cognito Setup](#aws-cognito-setup)
5. [Database Setup](#database-setup)
6. [Running the Application](#running-the-application)
7. [Development Workflow](#development-workflow)
8. [Testing](#testing)
9. [Troubleshooting](#troubleshooting)
10. [FAQ](#faq)

---

## Prerequisites

### Required Software

Install the following tools before proceeding:

| Tool | Version | Installation Link | Purpose |
|------|---------|-------------------|---------|
| **Go** | 1.24+ | [go.dev/doc/install](https://go.dev/doc/install) | Backend runtime |
| **Docker Desktop** | Latest | [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop) | Local infrastructure |
| **Make** | Any | Pre-installed on macOS/Linux, [Windows guide](https://gnuwin32.sourceforge.net/packages/make.htm) | Build automation |
| **Git** | 2.x+ | [git-scm.com/downloads](https://git-scm.com/downloads) | Version control |
| **AWS CLI** | 2.x+ | [aws.amazon.com/cli](https://aws.amazon.com/cli/) | AWS Cognito management (optional) |

### Verify Installation

```bash
# Check Go version
go version
# Expected: go version go1.24+ ...

# Check Docker
docker --version
# Expected: Docker version 24.0+ ...

# Check Docker is running
docker ps
# Should show running containers (or empty table if none running)

# Check Make
make --version
# Expected: GNU Make 3.x+ or 4.x+

# Check Git
git --version
# Expected: git version 2.x+
```

### Recommended Tools (Optional)

- **[VSCode](https://code.visualstudio.com/)** - Recommended editor with Go extension
- **[Postman](https://www.postman.com/)** or **[Insomnia](https://insomnia.rest/)** - API testing
- **[TablePlus](https://tableplus.com/)** or **[pgAdmin](https://www.pgadmin.org/)** - PostgreSQL GUI
- **[RedisInsight](https://redis.com/redis-enterprise/redis-insight/)** - Redis GUI
- **[golangci-lint](https://golangci-lint.run/)** - Go linter

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Add to PATH (if not already)
export PATH=$PATH:$(go env GOPATH)/bin
```

---

## Initial Setup

### 1. Clone the Repository

```bash
# Clone via HTTPS
git clone https://github.com/upb/llm-control-plane.git
cd llm-control-plane

# Or via SSH (if you have SSH keys configured)
git clone git@github.com:upb/llm-control-plane.git
cd llm-control-plane
```

### 2. Verify Repository Structure

```bash
ls -la
# Expected directories:
# - backend/        (Go application code)
# - docs/           (Documentation)
# - docker-compose.yml
# - Makefile
# - README.md
```

### 3. Install Go Dependencies

```bash
# Navigate to backend directory
cd backend

# Download all dependencies
go mod download

# Verify dependencies
go mod verify

# Navigate back to root
cd ..
```

### 4. Start Local Infrastructure

```bash
# Start PostgreSQL and Redis with Docker Compose
make infra-up

# Verify containers are running
docker compose ps

# Expected output:
# NAME            STATUS          PORTS
# llm-postgres    Up (healthy)    0.0.0.0:5432->5432/tcp
# llm-redis       Up (healthy)    0.0.0.0:6379->6379/tcp
```

**What this does:**
- Starts PostgreSQL 16 on port 5432
- Starts Redis 7 on port 6379
- Creates persistent volumes for data
- Runs health checks to ensure services are ready

---

## Configuration

### Environment Variables Setup

1. **Copy the example environment file:**

```bash
cd backend
cp .env.example .env
```

2. **Edit `.env` with your text editor:**

```bash
# Use your preferred editor
nano .env
# or
vim .env
# or
code .env  # VSCode
```

3. **Update the following required fields:**

```bash
# =============================================================================
# REQUIRED: LLM Provider Configuration
# =============================================================================

# OpenAI API Key (get from https://platform.openai.com/api-keys)
OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# =============================================================================
# REQUIRED: AWS Cognito Configuration (see AWS Cognito Setup section)
# =============================================================================

# AWS Region where your Cognito User Pool is located
AWS_REGION=us-east-1

# Cognito User Pool ID (format: us-east-1_xxxxxxxxx)
COGNITO_USER_POOL_ID=us-east-1_ABC123XYZ

# Cognito App Client ID
COGNITO_CLIENT_ID=1234567890abcdefghijklmnop

# Cognito App Client Secret
COGNITO_CLIENT_SECRET=abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklm

# Cognito Domain (format: https://your-domain.auth.region.amazoncognito.com)
COGNITO_DOMAIN=https://my-llm-control-plane.auth.us-east-1.amazoncognito.com

# =============================================================================
# OPTIONAL: Override defaults if needed
# =============================================================================

# Server Configuration
SERVER_PORT=8080
LOG_LEVEL=debug  # Use 'debug' for development, 'info' for production

# Database Configuration (defaults work for local development)
DB_HOST=localhost
DB_PORT=5432
DB_USER=llmcp
DB_PASSWORD=llmcp
DB_NAME=llmcp

# Redis Configuration (defaults work for local development)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=  # Empty for local development

# Observability
METRICS_ENABLED=true
TRACING_ENABLED=false  # Set to true to enable distributed tracing
```

### Configuration Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ENVIRONMENT` | No | `dev` | Environment (dev, staging, production) |
| `SERVER_PORT` | No | `8080` | HTTP server port |
| `LOG_LEVEL` | No | `info` | Log level (debug, info, warn, error) |
| `DB_HOST` | No | `localhost` | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_USER` | No | `llmcp` | PostgreSQL username |
| `DB_PASSWORD` | No | `llmcp` | PostgreSQL password |
| `DB_NAME` | No | `llmcp` | PostgreSQL database name |
| `REDIS_HOST` | No | `localhost` | Redis host |
| `REDIS_PORT` | No | `6379` | Redis port |
| `REDIS_PASSWORD` | No | `` | Redis password (empty for local) |
| `OPENAI_API_KEY` | **Yes** | - | OpenAI API key |
| `COGNITO_USER_POOL_ID` | **Yes*** | - | AWS Cognito User Pool ID |
| `COGNITO_CLIENT_ID` | **Yes*** | - | AWS Cognito Client ID |
| `COGNITO_CLIENT_SECRET` | **Yes*** | - | AWS Cognito Client Secret |
| `AWS_REGION` | No | `us-east-1` | AWS region |

*Required for production; can be omitted for local testing without authentication

---

## AWS Cognito Setup

AWS Cognito provides authentication and user management for the LLM Control Plane.

### Option 1: Create New User Pool (Recommended)

1. **Navigate to AWS Console:**
   - Go to [AWS Cognito Console](https://console.aws.amazon.com/cognito)
   - Select your region (e.g., us-east-1)

2. **Create User Pool:**
   ```
   Click "Create user pool" → 
   Step 1: Configure sign-in experience
     - Provider types: Cognito user pool
     - Cognito user pool sign-in options: Email
     - Click "Next"
   
   Step 2: Configure security requirements
     - Password policy: Choose defaults or customize
     - Multi-factor authentication: Optional (recommend "Optional MFA")
     - Click "Next"
   
   Step 3: Configure sign-up experience
     - Self-service sign-up: Enable
     - Attribute verification: Email
     - Required attributes: email, name
     - Click "Next"
   
   Step 4: Configure message delivery
     - Email provider: Send email with Cognito
     - Click "Next"
   
   Step 5: Integrate your app
     - User pool name: llm-control-plane-dev
     - App client name: llm-control-plane-backend
     - Click "Next"
   
   Step 6: Review and create
     - Review settings
     - Click "Create user pool"
   ```

3. **Configure App Client:**
   - After creation, go to "App integration" tab
   - Click on your app client
   - Note down:
     - **User Pool ID**: `us-east-1_ABC123XYZ`
     - **Client ID**: `1234567890abcdefghijklmnop`
   - Under "App client information", click "Show client secret"
     - **Client Secret**: Copy this value

4. **Configure Domain:**
   - Go to "App integration" → "Domain"
   - Click "Create Cognito domain"
   - Enter domain prefix: `my-llm-control-plane` (must be unique)
   - Click "Create"
   - Note the full domain: `https://my-llm-control-plane.auth.us-east-1.amazoncognito.com`

5. **Update `.env` file** with the values from above

### Option 2: Use Existing User Pool

If you already have a Cognito User Pool:

1. Get User Pool ID from Cognito Console
2. Create a new App Client for the LLM Control Plane
3. Enable client secret for the app client
4. Update `.env` with the credentials

### Option 3: Local Testing Without Cognito

For local testing without AWS Cognito:

1. Comment out authentication middleware in `backend/routes/routes.go`:
   ```go
   // r.Use(middleware.AuthMiddleware)
   ```

2. Set placeholder values in `.env`:
   ```bash
   COGNITO_USER_POOL_ID=local-testing
   COGNITO_CLIENT_ID=local-testing
   COGNITO_CLIENT_SECRET=local-testing
   ```

3. Manually add context values in your API requests (for testing)

**⚠️ Warning:** Option 3 is only for local development and testing. Never use this in production.

---

## Database Setup

### Initialize Database Schema

The database schema is automatically created when you run migrations.

1. **Run database migrations:**

```bash
# From repository root
make db-migrate
```

This creates the following tables:
- `organizations` - Multi-tenant organization records
- `applications` - Applications within organizations
- `users` - User accounts
- `policies` - Governance policies (rate limits, cost caps, etc.)
- `audit_logs` - Audit trail for all operations
- `inference_requests` - LLM inference request history

2. **Verify tables were created:**

```bash
# Connect to PostgreSQL
make db-shell

# List tables
\dt

# Expected output:
# Schema |        Name         | Type  | Owner
#--------+---------------------+-------+-------
# public | applications        | table | llmcp
# public | audit_logs          | table | llmcp
# public | inference_requests  | table | llmcp
# public | organizations       | table | llmcp
# public | policies            | table | llmcp
# public | users               | table | llmcp

# Exit PostgreSQL shell
\q
```

### Seed Test Data (Optional)

To seed the database with test data for development:

```bash
# Seed test data
make db-seed
```

This creates:
- Sample organization: "Acme Corp"
- Sample application: "Production App"
- Sample user: admin@example.com
- Sample policies: Rate limit, cost cap

### Database Management Commands

```bash
# Connect to PostgreSQL shell
make db-shell

# View database logs
make infra-logs | grep postgres

# Reset database (WARNING: destroys all data)
make infra-reset

# Backup database
docker exec llm-postgres pg_dump -U llmcp llmcp > backup.sql

# Restore database
docker exec -i llm-postgres psql -U llmcp llmcp < backup.sql
```

---

## Running the Application

### Start the Development Server

```bash
# From repository root
make dev
```

**What this does:**
1. Sets environment variables from `.env`
2. Compiles the Go application
3. Starts the HTTP server on port 8080
4. Enables hot-reload (automatically restarts on code changes)

**Expected output:**
```
{"level":"info","timestamp":"2026-02-16T10:30:00Z","msg":"starting api-gateway"}
{"level":"info","timestamp":"2026-02-16T10:30:01Z","msg":"configuration loaded","environment":"dev"}
{"level":"info","timestamp":"2026-02-16T10:30:01Z","msg":"database connection established"}
{"level":"info","timestamp":"2026-02-16T10:30:01Z","msg":"redis connection established"}
{"level":"info","timestamp":"2026-02-16T10:30:01Z","msg":"dependencies initialized successfully"}
{"level":"info","timestamp":"2026-02-16T10:30:01Z","msg":"api-gateway listening","address":"0.0.0.0:8080"}
```

### Verify the Server is Running

Open a new terminal and run:

```bash
# Health check
curl http://localhost:8080/healthz
# Expected: {"status":"ok"}

# Readiness check (validates all dependencies)
curl http://localhost:8080/readyz
# Expected: {"status":"ready","checks":{"database":"ok","redis":"ok"}}

# API status endpoint
curl http://localhost:8080/api/v1/status
# Expected: {"status":"running","version":"dev","uptime":"..."}
```

### Stop the Server

Press `Ctrl+C` in the terminal where the server is running.

---

## Development Workflow

### Typical Development Cycle

1. **Make code changes** in your editor

2. **Run tests** to verify changes:
   ```bash
   # Run specific test
   go test -v ./backend/services/policy
   
   # Run all tests
   make test
   ```

3. **Check code quality:**
   ```bash
   # Format code
   make fmt
   
   # Run linter
   make lint
   
   # Run all checks
   make check
   ```

4. **Restart development server:**
   ```bash
   # Server automatically restarts on code changes
   # Or manually restart with Ctrl+C and:
   make dev
   ```

5. **Test via API:**
   ```bash
   # Example: Test chat completion endpoint
   curl -X POST http://localhost:8080/api/v1/inference/chat \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     -d '{
       "model": "gpt-4",
       "messages": [
         {"role": "user", "content": "Hello, world!"}
       ]
     }'
   ```

### Useful Development Commands

```bash
# Build the application
make build

# Clean build artifacts
make clean

# View application logs
make logs

# View infrastructure logs
make infra-logs

# Restart infrastructure
make infra-down
make infra-up

# Update Go dependencies
make deps-update

# Generate code (if applicable)
make generate

# Run database migrations
make db-migrate

# Access database shell
make db-shell

# Access Redis CLI
docker exec -it llm-redis redis-cli
```

### Working with Multiple Services

If you need to run multiple services simultaneously:

```bash
# Terminal 1: Infrastructure
make infra-up
make infra-logs

# Terminal 2: API Gateway
make dev

# Terminal 3: Testing/Development
curl http://localhost:8080/healthz
```

---

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Open coverage report in browser
open coverage.html

# Run integration tests only
make test-integration

# Run tests in verbose mode
go test -v ./...

# Run specific test
go test -v ./backend/services/policy -run TestPolicyEvaluation

# Run tests for specific package
go test ./backend/services/...

# Skip slow integration tests
go test -short ./...
```

### Integration Tests

Integration tests use **testcontainers** to spin up real PostgreSQL and Redis instances.

**Requirements:**
- Docker must be running
- Tests automatically manage containers (start/stop)
- First run downloads container images (may take a few minutes)

**Running integration tests:**

```bash
# Run all integration tests
go test -v ./backend/tests/integration/

# Run specific integration test
go test -v ./backend/tests/integration/ -run TestPipelineIntegration

# Run specific test case
go test -v ./backend/tests/integration/ -run TestPipelineIntegration/TestSuccessfulInferenceFlow
```

**Integration test structure:**

```
backend/tests/integration/
├── pipeline_test.go    # Full end-to-end pipeline tests
│   ├── TestSuccessfulInferenceFlow
│   ├── TestPromptValidationRejection
│   ├── TestRateLimitEnforcement
│   ├── TestBudgetTracking
│   ├── TestProviderFallback
│   ├── TestModelRestrictions
│   ├── TestConcurrentRequests
│   └── ... (10 total test cases)
└── api_test.go         # HTTP API endpoint tests
    ├── TestHealthCheck
    ├── TestReadinessCheck
    ├── TestChatCompletionEndpoint
    ├── TestChatCompletionWithInvalidRequest
    ├── TestChatCompletionWithoutAuth
    ├── TestChatCompletionWithPIIDetection
    └── ... (8 total test cases)
```

### Test Coverage Goals

| Component | Target Coverage | Current Status |
|-----------|----------------|----------------|
| Core Pipeline | 80%+ | ✅ 85% |
| Business Logic | 75%+ | ✅ 78% |
| HTTP Handlers | 70%+ | ✅ 72% |
| Overall | 70%+ | ✅ 75% |

---

## Troubleshooting

### Issue: Cannot Connect to Database

**Symptoms:**
```
failed to initialize database: connection refused
```

**Solutions:**

1. **Verify PostgreSQL is running:**
   ```bash
   docker compose ps
   # Should show llm-postgres with status "Up (healthy)"
   ```

2. **Check PostgreSQL logs:**
   ```bash
   docker compose logs postgres
   ```

3. **Restart PostgreSQL:**
   ```bash
   make infra-down
   make infra-up
   ```

4. **Verify connection settings in `.env`:**
   ```bash
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=llmcp
   DB_PASSWORD=llmcp
   DB_NAME=llmcp
   ```

5. **Test connection manually:**
   ```bash
   psql -h localhost -p 5432 -U llmcp -d llmcp
   # Password: llmcp
   ```

---

### Issue: Cannot Connect to Redis

**Symptoms:**
```
failed to initialize redis: dial tcp: connect: connection refused
```

**Solutions:**

1. **Verify Redis is running:**
   ```bash
   docker compose ps
   # Should show llm-redis with status "Up (healthy)"
   ```

2. **Check Redis logs:**
   ```bash
   docker compose logs redis
   ```

3. **Test Redis connection:**
   ```bash
   redis-cli -h localhost -p 6379 ping
   # Expected: PONG
   
   # Or via Docker:
   docker exec -it llm-redis redis-cli ping
   ```

4. **Verify Redis settings in `.env`:**
   ```bash
   REDIS_HOST=localhost
   REDIS_PORT=6379
   REDIS_PASSWORD=  # Empty for local
   ```

---

### Issue: Port Already in Use

**Symptoms:**
```
bind: address already in use
```

**Solutions:**

1. **Find process using port 8080:**
   ```bash
   # On macOS/Linux
   lsof -i :8080
   
   # On Windows
   netstat -ano | findstr :8080
   ```

2. **Kill the process:**
   ```bash
   # On macOS/Linux
   kill -9 <PID>
   
   # On Windows
   taskkill /PID <PID> /F
   ```

3. **Or change the port in `.env`:**
   ```bash
   SERVER_PORT=8081
   ```

---

### Issue: OpenAI API Key Invalid

**Symptoms:**
```
provider error: invalid api key
```

**Solutions:**

1. **Verify API key format:**
   - Should start with `sk-proj-` or `sk-`
   - Should be ~50+ characters long

2. **Check API key in OpenAI dashboard:**
   - Go to [platform.openai.com/api-keys](https://platform.openai.com/api-keys)
   - Verify key is active
   - Create new key if necessary

3. **Update `.env` with correct key:**
   ```bash
   OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxx
   ```

4. **Restart the server:**
   ```bash
   # Ctrl+C to stop
   make dev
   ```

---

### Issue: AWS Cognito Authentication Failing

**Symptoms:**
```
failed to validate JWT: unable to verify token
```

**Solutions:**

1. **Verify Cognito credentials in `.env`:**
   ```bash
   COGNITO_USER_POOL_ID=us-east-1_ABC123XYZ
   COGNITO_CLIENT_ID=1234567890abcdefg
   COGNITO_CLIENT_SECRET=abcdefghijklmnopqrstuvwxyz
   AWS_REGION=us-east-1
   ```

2. **Test Cognito configuration:**
   ```bash
   aws cognito-idp describe-user-pool \
     --user-pool-id us-east-1_ABC123XYZ \
     --region us-east-1
   ```

3. **For local testing, disable authentication:**
   - Edit `backend/routes/routes.go`
   - Comment out `r.Use(middleware.AuthMiddleware)`
   - Restart server

---

### Issue: Docker Compose Fails to Start

**Symptoms:**
```
Error response from daemon: driver failed programming external connectivity
```

**Solutions:**

1. **Restart Docker Desktop:**
   - Quit Docker Desktop completely
   - Start Docker Desktop again
   - Wait for "Docker is running" status

2. **Clean up Docker resources:**
   ```bash
   docker system prune -a
   docker volume prune
   ```

3. **Remove existing containers:**
   ```bash
   docker compose down -v
   docker compose up -d
   ```

---

### Issue: Integration Tests Failing

**Symptoms:**
```
integration tests fail with timeout errors
```

**Solutions:**

1. **Ensure Docker is running:**
   ```bash
   docker ps
   ```

2. **Increase test timeout:**
   ```bash
   go test -v -timeout 10m ./backend/tests/integration/
   ```

3. **Run tests with more verbose output:**
   ```bash
   go test -v ./backend/tests/integration/ 2>&1 | tee test-output.log
   ```

4. **Skip integration tests during development:**
   ```bash
   go test -short ./...
   ```

---

### Issue: High Memory Usage

**Symptoms:**
- System becomes slow
- Docker containers consuming too much memory

**Solutions:**

1. **Check resource usage:**
   ```bash
   docker stats
   ```

2. **Reduce database connection pool in `.env`:**
   ```bash
   DB_MAX_OPEN_CONNS=10  # Reduced from 25
   DB_MAX_IDLE_CONNS=3   # Reduced from 5
   ```

3. **Reduce Redis pool size in `.env`:**
   ```bash
   REDIS_POOL_SIZE=5     # Reduced from 10
   REDIS_MIN_IDLE_CONNS=1  # Reduced from 2
   ```

4. **Allocate more memory to Docker:**
   - Docker Desktop → Settings → Resources
   - Increase Memory limit to 4GB+

---

### Issue: Go Module Download Errors

**Symptoms:**
```
go: error loading module requirements
```

**Solutions:**

1. **Clean module cache:**
   ```bash
   go clean -modcache
   ```

2. **Re-download dependencies:**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Verify Go version:**
   ```bash
   go version
   # Must be 1.24 or higher
   ```

4. **Update Go if necessary:**
   - Download from [go.dev/dl](https://go.dev/dl/)

---

## FAQ

### Q: Do I need AWS Cognito for local development?

**A:** Not strictly required for local development and testing. You can:
1. Use Cognito for full authentication testing (recommended)
2. Comment out auth middleware for quick local testing
3. Use mock JWT tokens in integration tests

For production deployment, Cognito (or similar auth provider) is required.

---

### Q: Can I use a different database than PostgreSQL?

**A:** The application is currently designed for PostgreSQL specifically. While the repository pattern provides abstraction, switching databases would require:
1. Implementing new repository interfaces
2. Updating database-specific queries
3. Creating new migrations

PostgreSQL is recommended for its robust feature set and wide adoption.

---

### Q: How do I add a new LLM provider (e.g., Anthropic)?

**A:** To add a new provider:

1. Create adapter implementation:
   ```
   backend/services/providers/anthropic/adapter.go
   ```

2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       Name() string
       ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
       IsAvailable(ctx context.Context) bool
       CalculateCost(req *ChatRequest, resp *ChatResponse) (float64, error)
   }
   ```

3. Register provider in `backend/app/dependencies.go`:
   ```go
   if cfg.Providers.Anthropic.APIKey != "" {
       anthropicProvider := anthropic.NewAdapter(cfg.Providers.Anthropic, logger)
       registry.Register(anthropicProvider)
   }
   ```

4. Add configuration to `backend/config/config.go`

5. Update `.env.example` with new provider settings

---

### Q: How do I run the application in production?

**A:** For production deployment:

1. **Environment setup:**
   - Use AWS Secrets Manager or Parameter Store for secrets
   - Never commit `.env` files to version control
   - Set `ENVIRONMENT=production` in environment variables

2. **Infrastructure:**
   - Use managed PostgreSQL (RDS)
   - Use managed Redis (ElastiCache)
   - Deploy behind Application Load Balancer

3. **Security:**
   - Enable SSL/TLS for database connections
   - Use AWS IAM roles instead of API keys
   - Enable WAF (Web Application Firewall)
   - Enable CloudWatch logging and monitoring

4. **Deployment options:**
   - AWS Lambda (see `make build-lambda`)
   - ECS/Fargate containers
   - EKS Kubernetes cluster

See deployment documentation in `docs/deployment/` (coming soon).

---

### Q: How do I debug authentication issues?

**A:** For debugging authentication:

1. **Enable debug logging in `.env`:**
   ```bash
   LOG_LEVEL=debug
   ```

2. **Check JWT token claims:**
   ```bash
   # Decode JWT at https://jwt.io
   # Or use jq:
   echo "YOUR_JWT_TOKEN" | cut -d. -f2 | base64 -d | jq .
   ```

3. **Verify Cognito configuration:**
   ```bash
   aws cognito-idp describe-user-pool \
     --user-pool-id YOUR_POOL_ID
   ```

4. **Test with curl:**
   ```bash
   curl -v http://localhost:8080/api/v1/inference/chat \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'
   ```

---

### Q: How do I contribute to the project?

**A:** To contribute:

1. **Fork the repository** on GitHub

2. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Follow coding conventions:**
   - See `docs/conventions/GO_CONVENTIONS.md`
   - Run `make fmt` and `make lint`
   - Write tests for new code

4. **Run all checks:**
   ```bash
   make check
   make test
   ```

5. **Commit with clear messages:**
   ```bash
   git commit -m "feat: add new feature"
   ```

6. **Push and create Pull Request:**
   ```bash
   git push origin feature/your-feature-name
   ```

See `CONTRIBUTING.md` for detailed guidelines (coming soon).

---

### Q: Where can I find more documentation?

**A:** Additional documentation:

| Document | Location | Description |
|----------|----------|-------------|
| **README** | `README.md` | Project overview and quick start |
| **Architecture** | `docs/architecture/` | System architecture and design |
| **Conventions** | `docs/conventions/GO_CONVENTIONS.md` | Coding standards (664 lines) |
| **API Reference** | `docs/api/` | API endpoint documentation |
| **Security** | `docs/security/` | Security guidelines and threat model |
| **Deployment** | `docs/deployment/` | Production deployment guides |

For questions not covered in documentation:
- Open an issue on GitHub
- Check existing issues and discussions
- Contact the maintainers

---

## Additional Resources

### Learning Resources

- **Go Documentation:** [go.dev/doc](https://go.dev/doc/)
- **PostgreSQL Docs:** [postgresql.org/docs](https://www.postgresql.org/docs/)
- **Redis Documentation:** [redis.io/docs](https://redis.io/docs/)
- **AWS Cognito Guide:** [docs.aws.amazon.com/cognito](https://docs.aws.amazon.com/cognito/)
- **Docker Compose:** [docs.docker.com/compose](https://docs.docker.com/compose/)

### Project-Specific Docs

- **[Go Conventions](../conventions/GO_CONVENTIONS.md)** - Comprehensive coding standards
- **[Architecture](../architecture/ARCHITECTURE.md)** - System design and patterns
- **[Security](../security/SECURITY.md)** - Security guidelines and best practices
- **[API Reference](../api/API_REFERENCE.md)** - Complete API documentation

---

## Getting Help

If you encounter issues not covered in this guide:

1. **Check the [Troubleshooting](#troubleshooting) section** above
2. **Search [existing GitHub issues](https://github.com/upb/llm-control-plane/issues)**
3. **Open a new issue** with:
   - Clear description of the problem
   - Steps to reproduce
   - Expected vs actual behavior
   - Your environment (OS, Go version, etc.)
   - Relevant logs or error messages

---

**Happy coding! 🚀**
