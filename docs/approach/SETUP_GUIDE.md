# LLM Control Plane - Practical Setup Guide

**Version:** 1.0  
**Date:** February 6, 2026  
**Audience:** Developers and DevOps engineers

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development Setup](#local-development-setup)
3. [AWS Account Preparation](#aws-account-preparation)
4. [GitHub Repository Setup](#github-repository-setup)
5. [Deploying Sandbox Environment](#deploying-sandbox-environment)
6. [Testing the Deployment](#testing-the-deployment)
7. [Deploying Production](#deploying-production)
8. [Troubleshooting](#troubleshooting)

---

## 1. Prerequisites

### 1.1 Software Requirements

Install the following tools:

```powershell
# Go 1.24+
winget install GoLang.Go

# Node.js 20+
winget install OpenJS.NodeJS

# Docker Desktop
winget install Docker.DockerDesktop

# AWS CLI v2
winget install Amazon.AWSCLI

# Git
winget install Git.Git

# Make (via Chocolatey)
choco install make

# Optional: PostgreSQL client (psql)
choco install postgresql
```

**Verify installations:**
```powershell
go version           # Should be 1.24+
node --version       # Should be 20+
docker --version     # Should be 24+
aws --version        # Should be 2.x
git --version
make --version
```

### 1.2 AWS Account Requirements

- **AWS Account ID:** (note this down)
- **IAM User with permissions:**
  - Administrator access (for initial setup)
  - Or specific permissions: Lambda, API Gateway, Cognito, Aurora, ElastiCache, S3, CloudFront, Secrets Manager
- **AWS CLI configured:**
  ```powershell
  aws configure
  # Enter: Access Key ID, Secret Access Key, Region (us-east-1), Output (json)
  ```

### 1.3 GitHub Account

- **GitHub account** with repository access
- **GitHub CLI (optional):**
  ```powershell
  winget install GitHub.cli
  gh auth login
  ```

---

## 2. Local Development Setup

### 2.1 Clone Repository

```powershell
cd C:\Users\ASUS\Documents
git clone https://github.com/<your-org>/llm-control-plane.git
cd llm-control-plane
```

### 2.2 Start Local Infrastructure

**Start PostgreSQL and Redis:**
```powershell
cd backend
docker-compose up -d

# Verify services are running
docker ps

# Expected output:
# CONTAINER ID   IMAGE                 PORTS
# abc123         postgres:16-alpine    0.0.0.0:5432->5432/tcp
# def456         redis:7-alpine        0.0.0.0:6379->6379/tcp
```

**Check logs:**
```powershell
docker-compose logs -f postgres
docker-compose logs -f redis
```

### 2.3 Configure Environment Variables

**Create `backend/.env`:**
```powershell
cd backend
cp .env.example .env
```

**Edit `backend/.env`:**
```bash
# Environment
ENVIRONMENT=dev

# Database
DATABASE_URL=postgresql://dev:audit_password@localhost:5432/audit?sslmode=disable

# Redis
REDIS_URL=localhost:6379

# Cognito (will be set up later, leave blank for now)
COGNITO_USER_POOL_ID=
COGNITO_CLIENT_ID=
COGNITO_CLIENT_SECRET=
COGNITO_DOMAIN=
COGNITO_REDIRECT_URI=http://localhost:8080/oauth2/idpresponse

# Frontend URL
FRONT_END_URL=http://localhost:5173

# LLM Provider API Keys (use your own keys)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
AZURE_OPENAI_KEY=
AZURE_OPENAI_ENDPOINT=

# Observability (optional for local dev)
DATADOG_API_KEY=
DATADOG_SITE=us5.datadoghq.com
```

### 2.4 Install Dependencies

**Backend:**
```powershell
cd backend
go mod download
go mod tidy
```

**Frontend:**
```powershell
cd frontend
npm install
```

### 2.5 Run Database Migrations

```powershell
cd backend
make migrate-up

# Or manually:
go run cmd/migrate/main.go up
```

**Verify migrations:**
```powershell
# Connect to PostgreSQL
docker exec -it llm-cp-postgres psql -U dev -d audit

# List tables
\dt

# Expected output:
# Schema |       Name        | Type  | Owner
#--------+-------------------+-------+-------
# public | organizations     | table | dev
# public | applications      | table | dev
# public | users             | table | dev
# public | policies          | table | dev
# public | request_logs      | table | dev
# public | schema_migrations | table | dev

# Exit
\q
```

### 2.6 Seed Database (Optional)

```powershell
cd backend
go run cmd/seed/main.go

# This creates:
# - 1 organization (org-dev)
# - 1 application (app-dev)
# - 1 user (dev@example.com)
# - Sample policies
```

### 2.7 Run Local Development Servers

**Terminal 1 - Backend:**
```powershell
cd backend
make backend-dev

# Or:
go run main.go

# Expected output:
# [INFO] Starting LLM Control Plane Gateway
# [INFO] Environment: dev
# [INFO] Listening on :8080
```

**Terminal 2 - Frontend:**
```powershell
cd frontend
npm run dev

# Expected output:
# VITE v7.0.0  ready in 423 ms
# ➜  Local:   http://localhost:5173/
# ➜  Network: use --host to expose
```

### 2.8 Test Local API

**Health check:**
```powershell
curl http://localhost:8080/health

# Expected response:
# {"status":"ok"}
```

**Readiness check:**
```powershell
curl http://localhost:8080/health/ready

# Expected response:
# {
#   "status":"ok",
#   "checks":{
#     "database":"ok",
#     "redis":"ok"
#   }
# }
```

**Chat completion (without auth for local testing):**
```powershell
curl -X POST http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Note: Authentication middleware should be disabled for local testing
# Or use a mock JWT token
```

---

## 3. AWS Account Preparation

### 3.1 Create IAM OIDC Provider for GitHub Actions

**Step 1: Create OIDC Provider**
```powershell
aws iam create-open-id-connect-provider `
  --url https://token.actions.githubusercontent.com `
  --client-id-list sts.amazonaws.com `
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1

# Note the ARN (e.g., arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com)
```

**Step 2: Create IAM Role for GitHub Actions**

Create `github-actions-role.json`:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:<GITHUB_ORG>/<GITHUB_REPO>:*"
        }
      }
    }
  ]
}
```

**Create role:**
```powershell
aws iam create-role `
  --role-name GitHubActionsDeploymentRole `
  --assume-role-policy-document file://github-actions-role.json

# Attach policies (for sandbox, use PowerUserAccess)
aws iam attach-role-policy `
  --role-name GitHubActionsDeploymentRole `
  --policy-arn arn:aws:iam::aws:policy/PowerUserAccess

# For production, use more restrictive policies
```

**Note the Role ARN:**
```
arn:aws:iam::<ACCOUNT_ID>:role/GitHubActionsDeploymentRole
```

### 3.2 Create Secrets in AWS Secrets Manager

**LLM Provider Credentials:**
```powershell
# Create provider secrets
aws secretsmanager create-secret `
  --name /llm-cp/sandbox/providers `
  --secret-string '{
    "openai_api_key": "sk-...",
    "anthropic_api_key": "sk-ant-...",
    "azure_openai_key": "",
    "azure_openai_endpoint": ""
  }'

# Repeat for prod
aws secretsmanager create-secret `
  --name /llm-cp/prod/providers `
  --secret-string '{
    "openai_api_key": "sk-...",
    "anthropic_api_key": "sk-ant-...",
    "azure_openai_key": "",
    "azure_openai_endpoint": ""
  }'
```

**Datadog API Key:**
```powershell
aws secretsmanager create-secret `
  --name /llm-cp/datadog `
  --secret-string '{
    "api_key": "YOUR_DATADOG_API_KEY"
  }'
```

### 3.3 Verify AWS Setup

```powershell
# List secrets
aws secretsmanager list-secrets

# Get secret value (to verify)
aws secretsmanager get-secret-value `
  --secret-id /llm-cp/sandbox/providers
```

---

## 4. GitHub Repository Setup

### 4.1 Configure GitHub Secrets

Navigate to your GitHub repository: `https://github.com/<org>/<repo>/settings/secrets/actions`

**Add the following secrets:**

| Secret Name | Value | Description |
|-------------|-------|-------------|
| `AWS_ROLE_ARN` | `arn:aws:iam::<account>:role/GitHubActionsDeploymentRole` | Non-prod AWS role |
| `AWS_ROLE_ARN_PROD` | `arn:aws:iam::<prod-account>:role/GitHubActionsDeploymentRole` | Production AWS role |
| `DATADOG_API_KEY` | `<your-datadog-api-key>` | Datadog APM API key |
| `DATADOG_APPLICATION_ID` | `<your-datadog-app-id>` | Datadog RUM Application ID |
| `DATADOG_CLIENT_TOKEN` | `<your-datadog-client-token>` | Datadog RUM Client Token |

**Using GitHub CLI:**
```powershell
gh secret set AWS_ROLE_ARN --body "arn:aws:iam::123456789012:role/GitHubActionsDeploymentRole"
gh secret set DATADOG_API_KEY --body "YOUR_API_KEY"
gh secret set DATADOG_APPLICATION_ID --body "YOUR_APP_ID"
gh secret set DATADOG_CLIENT_TOKEN --body "YOUR_CLIENT_TOKEN"
```

### 4.2 Verify GitHub Actions Permissions

Ensure GitHub Actions has `id-token: write` permission:

In `.github/workflows/infra.yml`:
```yaml
permissions:
  id-token: write  # Required for OIDC
  contents: read
```

---

## 5. Deploying Sandbox Environment

### 5.1 Create Infrastructure Configuration

**Create `infra/sandbox-llm-cp/` directory:**
```powershell
mkdir infra\sandbox-llm-cp
mkdir infra\sandbox-llm-cp\assets
```

**Create `infra/sandbox-llm-cp/cors.json`:**
```json
{
  "CORSRules": [
    {
      "AllowedOrigins": ["https://sandbox.llm-cp.yourdomain.com", "http://localhost:5173"],
      "AllowedMethods": ["GET", "POST", "PUT", "DELETE"],
      "AllowedHeaders": ["*"],
      "ExposeHeaders": ["ETag"],
      "MaxAgeSeconds": 3600
    }
  ]
}
```

**Add branding assets:**
- Copy `logo.png` to `infra/sandbox-llm-cp/assets/logo.png`
- Copy `favicon.ico` to `infra/sandbox-llm-cp/assets/favicon.ico`
- Copy `background.png` to `infra/sandbox-llm-cp/assets/background.png`

### 5.2 Commit and Push

```powershell
git add infra/
git commit -m "Add sandbox infrastructure configuration"
git push origin main
```

### 5.3 Monitor Deployment

**Option 1: GitHub UI**
- Navigate to `https://github.com/<org>/<repo>/actions`
- Click on the running workflow
- Monitor each step

**Option 2: GitHub CLI**
```powershell
# Watch workflow runs
gh run watch

# View logs
gh run view <run-id> --log
```

**Option 3: Makefile (if implemented)**
```powershell
make wait-for-ci
make check-ci-logs
```

### 5.4 Verify Sandbox Deployment

**Step 1: Check Lambda Function**
```powershell
aws lambda list-functions --query 'Functions[?contains(FunctionName, `llm-cp-sandbox`)].FunctionName'

# Expected output:
# [
#     "llm-cp-sandbox-llm-cp"
# ]
```

**Step 2: Invoke Lambda (health check)**
```powershell
aws lambda invoke `
  --function-name llm-cp-sandbox-llm-cp `
  --payload '{"rawPath":"/health","requestContext":{"http":{"method":"GET"}}}' `
  response.json

cat response.json

# Expected output:
# {"statusCode":200,"body":"{\"status\":\"ok\"}"}
```

**Step 3: Check API Gateway**
```powershell
aws apigateway get-rest-apis --query 'items[?contains(name, `llm-cp-sandbox`)].{Name:name,ID:id}'

# Expected output:
# [
#     {
#         "Name": "llm-cp-sandbox",
#         "ID": "abc123xyz"
#     }
# ]
```

**Step 4: Test API Endpoint**
```powershell
curl https://api.sandbox.llm-cp.yourdomain.com/health

# Expected output:
# {"status":"ok"}
```

### 5.5 Check Aurora Database

```powershell
# List Aurora clusters
aws rds describe-db-clusters --query 'DBClusters[?contains(DBClusterIdentifier, `llm-cp-sandbox`)].{ID:DBClusterIdentifier,Status:Status}'

# Expected output:
# [
#     {
#         "ID": "llm-cp-sandbox",
#         "Status": "available"
#     }
# ]
```

**Connect to Aurora (if publicly accessible):**
```powershell
# Get database credentials from Secrets Manager
aws secretsmanager get-secret-value `
  --secret-id <AURORA_SECRET_ARN> `
  --query SecretString `
  --output text | ConvertFrom-Json

# Connect using psql
psql -h <aurora-endpoint> -U <username> -d llm_cp_sandboxdb

# List tables
\dt
```

### 5.6 Check ElastiCache Redis

```powershell
# List Redis clusters
aws elasticache describe-cache-clusters --query 'CacheClusters[?contains(CacheClusterId, `llm-cp-sandbox`)].{ID:CacheClusterId,Status:CacheClusterStatus}'

# Expected output:
# [
#     {
#         "ID": "llm-cp-sandbox",
#         "Status": "available"
#     }
# ]
```

**Test Redis connection (from Lambda or EC2 in VPC):**
```powershell
# Use redis-cli (must be in VPC)
redis-cli -h <redis-endpoint> -p 6379

# Test commands
PING  # Should return "PONG"
SET test "hello"
GET test  # Should return "hello"
```

---

## 6. Testing the Deployment

### 6.1 Test Cognito Authentication

**Step 1: Get Cognito User Pool details**
```powershell
aws cognito-idp list-user-pools --max-results 10 `
  --query 'UserPools[?contains(Name, `llm-cp-sandbox`)].{Name:Name,ID:Id}'

# Note the User Pool ID
```

**Step 2: Create test user**
```powershell
aws cognito-idp admin-create-user `
  --user-pool-id <USER_POOL_ID> `
  --username testuser@example.com `
  --user-attributes Name=email,Value=testuser@example.com Name=custom:orgId,Value=org-123 Name=custom:appId,Value=app-456 Name=custom:userRole,Value=developer `
  --temporary-password TempPass123!

# Set permanent password
aws cognito-idp admin-set-user-password `
  --user-pool-id <USER_POOL_ID> `
  --username testuser@example.com `
  --password MyPassword123! `
  --permanent
```

**Step 3: Test login flow**

Navigate to:
```
https://llm-cp-sandbox.auth.us-east-1.amazoncognito.com/login?
  client_id=<CLIENT_ID>&
  response_type=code&
  redirect_uri=https://api.sandbox.llm-cp.yourdomain.com/oauth2/idpresponse&
  scope=openid+email+profile
```

### 6.2 Test Inference API

**Step 1: Get JWT token (manually for testing)**

Use Postman or curl to exchange authorization code for JWT (from OAuth2 flow above).

**Step 2: Test chat completion**
```powershell
$jwt = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

curl -X POST https://api.sandbox.llm-cp.yourdomain.com/v1/chat/completions `
  -H "Authorization: Bearer $jwt" `
  -H "Content-Type: application/json" `
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'

# Expected response:
# {
#   "id": "req-xyz789",
#   "object": "chat.completion",
#   "created": 1707206400,
#   "model": "gpt-3.5-turbo",
#   "choices": [
#     {
#       "index": 0,
#       "message": {
#         "role": "assistant",
#         "content": "The capital of France is Paris."
#       },
#       "finish_reason": "stop"
#     }
#   ],
#   "usage": {
#     "prompt_tokens": 15,
#     "completion_tokens": 8,
#     "total_tokens": 23
#   },
#   "metadata": {
#     "provider": "openai",
#     "cost_usd": 0.00046,
#     "latency_ms": 342
#   }
# }
```

### 6.3 Test PII Detection

```powershell
curl -X POST https://api.sandbox.llm-cp.yourdomain.com/v1/chat/completions `
  -H "Authorization: Bearer $jwt" `
  -H "Content-Type: application/json" `
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "My email is john.doe@example.com and my SSN is 123-45-6789"}
    ]
  }'

# Expected response (PII blocked):
# {
#   "error": {
#     "code": "PII_DETECTED",
#     "message": "Prompt contains personally identifiable information",
#     "details": [
#       {"type": "email", "value": "john.doe@example.com"},
#       {"type": "ssn", "value": "123-45-6789"}
#     ]
#   }
# }
```

### 6.4 Test Rate Limiting

```powershell
# Send 101 requests rapidly (exceeds 100 req/min limit)
for ($i = 1; $i -le 101; $i++) {
    curl -X POST https://api.sandbox.llm-cp.yourdomain.com/v1/chat/completions `
      -H "Authorization: Bearer $jwt" `
      -H "Content-Type: application/json" `
      -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Test"}]}'
}

# Expected: 101st request returns 429 Too Many Requests
# {
#   "error": {
#     "code": "RATE_LIMIT_EXCEEDED",
#     "message": "You have exceeded the rate limit of 100 requests per minute"
#   }
# }
```

### 6.5 Test Admin API

**List organizations:**
```powershell
curl -X GET https://api.sandbox.llm-cp.yourdomain.com/v1/admin/organizations `
  -H "Authorization: Bearer $jwt"

# Expected response:
# [
#   {
#     "id": "org-123",
#     "name": "Test Organization",
#     "created_at": "2026-02-06T12:00:00Z"
#   }
# ]
```

**Create policy:**
```powershell
curl -X POST https://api.sandbox.llm-cp.yourdomain.com/v1/admin/policies `
  -H "Authorization: Bearer $jwt" `
  -H "Content-Type: application/json" `
  -d '{
    "org_id": "org-123",
    "policy_type": "rate_limit",
    "config": {
      "requests_per_minute": 50,
      "requests_per_day": 5000
    }
  }'

# Expected response:
# {
#   "id": "policy-xyz",
#   "org_id": "org-123",
#   "policy_type": "rate_limit",
#   "config": {...},
#   "created_at": "2026-02-06T12:30:00Z"
# }
```

### 6.6 Check Audit Logs

**Query PostgreSQL:**
```powershell
# Connect to Aurora
psql -h <aurora-endpoint> -U <username> -d llm_cp_sandboxdb

# Query recent requests
SELECT 
  timestamp,
  org_id,
  app_id,
  model,
  provider,
  tokens_input,
  tokens_output,
  cost_usd,
  latency_ms,
  status
FROM request_logs
ORDER BY timestamp DESC
LIMIT 10;
```

**Check S3 (long-term archive):**
```powershell
aws s3 ls s3://llm-cp-sandbox-audit-logs/ --recursive

# Expected: logs archived after 90 days
# 2026-02-06 12:00:00  1234 2026/02/06/req-xyz789.json
```

---

## 7. Deploying Production

### 7.1 Create Production Configuration

```powershell
mkdir infra\prod-llm-cp
mkdir infra\prod-llm-cp\assets

# Copy CORS config
cp infra\sandbox-llm-cp\cors.json infra\prod-llm-cp\cors.json

# Edit CORS to production domains
# Change: "https://sandbox.llm-cp.yourdomain.com" → "https://llm-cp.yourdomain.com"

# Copy assets
cp infra\sandbox-llm-cp\assets\* infra\prod-llm-cp\assets\
```

### 7.2 Commit Production Config

```powershell
git add infra/prod-llm-cp/
git commit -m "Add production infrastructure configuration"
git push origin main
```

### 7.3 Tag Release

```powershell
# Create version tag
git tag -a v1.0.0 -m "Release v1.0.0 - MVP"
git push origin v1.0.0

# This triggers production deployment workflow (.github/workflows/on-tags.yml)
```

### 7.4 Monitor Production Deployment

```powershell
# Watch GitHub Actions
gh run watch

# Check CloudWatch logs
aws logs tail /aws/lambda/llm-cp-prod-llm-cp --follow
```

### 7.5 Smoke Test Production

**Health check:**
```powershell
curl https://api.llm-cp.yourdomain.com/health

# Expected: {"status":"ok"}
```

**Readiness check:**
```powershell
curl https://api.llm-cp.yourdomain.com/health/ready

# Expected: {"status":"ok","checks":{...}}
```

**Inference test (with production JWT):**
```powershell
curl -X POST https://api.llm-cp.yourdomain.com/v1/chat/completions `
  -H "Authorization: Bearer $jwt" `
  -H "Content-Type: application/json" `
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello production!"}]
  }'
```

---

## 8. Troubleshooting

### 8.1 Lambda Function Errors

**Symptom:** 500 Internal Server Error from API Gateway

**Diagnosis:**
```powershell
# Check Lambda logs
aws logs tail /aws/lambda/llm-cp-sandbox-llm-cp --follow

# Invoke Lambda directly to see error
aws lambda invoke `
  --function-name llm-cp-sandbox-llm-cp `
  --payload '{"rawPath":"/health","requestContext":{"http":{"method":"GET"}}}' `
  response.json

cat response.json
```

**Common issues:**
- **Database connection timeout:** Check VPC configuration, security groups
- **Redis connection refused:** Check ElastiCache endpoint, security groups
- **Secrets Manager error:** Verify IAM role has `secretsmanager:GetSecretValue` permission

### 8.2 Database Migration Failures

**Symptom:** Lambda invocation for migrations returns error

**Diagnosis:**
```powershell
# Check migration logs
aws logs filter-log-events `
  --log-group-name /aws/lambda/llm-cp-sandbox-llm-cp `
  --filter-pattern "migration"

# Or check Datadog APM traces
```

**Common issues:**
- **Duplicate migration:** Already applied, check `schema_migrations` table
- **Syntax error:** Fix SQL in `backend/migrations/schema/`, rollback, re-apply
- **Connection timeout:** Aurora scaling up, wait and retry

### 8.3 API Gateway 403 Forbidden

**Symptom:** All requests return 403

**Diagnosis:**
```powershell
# Check WAF rules
aws wafv2 list-web-acls --scope REGIONAL --region us-east-1

# Check API Gateway resource policy
aws apigateway get-rest-api --rest-api-id <API_ID> --query 'policy'
```

**Common issues:**
- **IP blocked by WAF:** Whitelist your IP in WAF rules
- **CORS preflight failing:** Check CORS configuration
- **API Gateway throttling:** Increase throttling limits

### 8.4 Aurora Connection Issues

**Symptom:** Lambda times out connecting to Aurora

**Diagnosis:**
```powershell
# Check Aurora status
aws rds describe-db-clusters --query 'DBClusters[?contains(DBClusterIdentifier, `llm-cp-sandbox`)].{Status:Status,Endpoint:Endpoint}'

# Check security groups
aws ec2 describe-security-groups --filters "Name=group-name,Values=*llm-cp*" --query 'SecurityGroups[*].{ID:GroupId,Name:GroupName,IngressRules:IpPermissions}'
```

**Common issues:**
- **Security group:** Lambda SG not allowed in Aurora SG inbound rules
- **Aurora scaling down:** Takes 30-60s to scale up, retry with backoff
- **VPC configuration:** Lambda not in same VPC as Aurora

### 8.5 Redis Connection Issues

**Symptom:** Rate limiting not working, errors in logs

**Diagnosis:**
```powershell
# Check ElastiCache status
aws elasticache describe-cache-clusters --cache-cluster-id llm-cp-sandbox --show-cache-node-info

# Test Redis from Lambda (add test endpoint)
aws lambda invoke `
  --function-name llm-cp-sandbox-llm-cp `
  --payload '{"type":"redis-test"}' `
  response.json
```

**Common issues:**
- **Security group:** Lambda SG not allowed in ElastiCache SG inbound rules
- **Incorrect endpoint:** Check Redis endpoint in Lambda environment variables
- **Redis memory full:** Increase node size or enable eviction policy

### 8.6 GitHub Actions Deployment Failures

**Symptom:** Workflow fails at specific step

**Diagnosis:**
```powershell
# View workflow logs
gh run view <run-id> --log

# Re-run failed jobs
gh run rerun <run-id> --failed
```

**Common issues:**
- **OIDC authentication failed:** Check IAM role trust policy, GitHub secrets
- **Terraform state locked:** Wait for lock timeout (5 minutes) or manually unlock in DynamoDB
- **Resource limit exceeded:** Increase AWS service quotas (e.g., Lambda concurrent executions)

### 8.7 Frontend Not Loading

**Symptom:** CloudFront returns 404 or blank page

**Diagnosis:**
```powershell
# Check CloudFront distribution
aws cloudfront list-distributions --query 'DistributionList.Items[?contains(DomainName, `llm-cp`)].{ID:Id,Status:Status,Domain:DomainName}'

# Check S3 bucket contents
aws s3 ls s3://llm-cp-sandbox-website/ --recursive

# Expected: index.html, assets/, etc.
```

**Common issues:**
- **CloudFront cache:** Invalidate CloudFront cache
  ```powershell
  aws cloudfront create-invalidation --distribution-id <DISTRIBUTION_ID> --paths "/*"
  ```
- **Missing index.html:** Frontend build failed, check GitHub Actions logs
- **CORS error:** Update CORS in S3 bucket or CloudFront

### 8.8 Useful AWS CLI Commands

**Lambda:**
```powershell
# List all Lambda functions
aws lambda list-functions --query 'Functions[*].{Name:FunctionName,Runtime:Runtime,Memory:MemorySize}'

# Update environment variable
aws lambda update-function-configuration `
  --function-name llm-cp-sandbox-llm-cp `
  --environment Variables="{ENVIRONMENT=sandbox,REDIS_URL=new-endpoint:6379}"

# Invoke function (test)
aws lambda invoke `
  --function-name llm-cp-sandbox-llm-cp `
  --payload '{"rawPath":"/health","requestContext":{"http":{"method":"GET"}}}' `
  response.json
```

**Aurora:**
```powershell
# Stop Aurora cluster (sandbox only, save costs)
aws rds stop-db-cluster --db-cluster-identifier llm-cp-sandbox

# Start Aurora cluster
aws rds start-db-cluster --db-cluster-identifier llm-cp-sandbox

# Get connection string
aws secretsmanager get-secret-value `
  --secret-id <AURORA_SECRET_ARN> `
  --query SecretString `
  --output text
```

**S3:**
```powershell
# List buckets
aws s3 ls

# Copy file to bucket
aws s3 cp test.json s3://llm-cp-sandbox-files/test.json

# Generate presigned URL
aws s3 presign s3://llm-cp-sandbox-files/test.json --expires-in 3600
```

**Secrets Manager:**
```powershell
# List secrets
aws secretsmanager list-secrets

# Get secret value
aws secretsmanager get-secret-value --secret-id /llm-cp/sandbox/providers

# Update secret
aws secretsmanager update-secret `
  --secret-id /llm-cp/sandbox/providers `
  --secret-string '{"openai_api_key":"sk-NEW_KEY"}'
```

---

## Next Steps

1. **Set up Datadog dashboards** for monitoring
2. **Configure alerting** (PagerDuty, Slack, SNS)
3. **Run load tests** (using Apache Bench, k6, or Locust)
4. **Document API** (OpenAPI/Swagger spec)
5. **Create runbooks** for common operational tasks
6. **Plan multi-tenancy migration** (Phase 2)

---

## Appendix: Quick Reference

### Common Commands

```powershell
# Start local development
make dev

# Run backend tests
make backend-test

# Run migrations
make migrate-up

# Reset local database
make reset-db

# Deploy sandbox (push to main)
git push origin main

# Deploy production (tag release)
git tag v1.0.0 && git push origin v1.0.0

# Check GitHub Actions
gh run watch

# Check Lambda logs
aws logs tail /aws/lambda/llm-cp-sandbox-llm-cp --follow

# Invoke Lambda
aws lambda invoke --function-name llm-cp-sandbox-llm-cp --payload '{}' response.json
```

### Useful Links

- **AWS Console:** https://console.aws.amazon.com
- **GitHub Actions:** https://github.com/<org>/<repo>/actions
- **Datadog:** https://app.datadoghq.com
- **Cognito Console:** https://console.aws.amazon.com/cognito
- **Aurora Console:** https://console.aws.amazon.com/rds
- **ElastiCache Console:** https://console.aws.amazon.com/elasticache

---

**End of Setup Guide**

**Version:** 1.0  
**Last Updated:** February 6, 2026  
**Feedback:** [Create issue on GitHub](https://github.com/<org>/<repo>/issues)
