# TSum SaaS Architecture

A reference document for the complete TSum architecture: multi-tenancy, AWS infrastructure, authentication, backend, frontend, and development environment. Intended as a replication guide for other SaaS projects on the same stack.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Multi-Tenancy](#2-multi-tenancy)
3. [AWS Infrastructure](#3-aws-infrastructure)
4. [Deployment Environments & CI/CD](#4-deployment-environments--cicd)
5. [Authentication](#5-authentication)
6. [Backend Architecture](#6-backend-architecture)
7. [Frontend Architecture](#7-frontend-architecture)
8. [Database Migrations & Seeding](#8-database-migrations--seeding)
9. [Local Development Environment](#9-local-development-environment)
10. [Replication Guide for Other SaaS Projects](#10-replication-guide-for-other-saas-projects)
11. [Key Design Decisions](#11-key-design-decisions)
12. [Reference Files Index](#12-reference-files-index)

---

## 1. System Overview

TSum is a multi-tenant SaaS application where:

- **TSum** is the platform provider
- **Service Providers** are companies that use TSum (e.g., "Inrush", "LDENG")
- **Tenants** are customers of a service provider (e.g., "BP", "Suncor")
- **Users** belong to a tenant and authenticate via Cognito

The backend is a Go Lambda function behind API Gateway. The frontend is a React SPA served via CloudFront and S3. PostgreSQL (Aurora Serverless v2) provides database-per-tenant isolation. AWS Cognito handles authentication.

```
Request Flow:
  Browser → CloudFront → S3 (frontend assets)
  API calls → API Gateway → Lambda (Go backend) → Aurora PostgreSQL
  Auth: Browser ↔ Lambda ↔ Cognito (OIDC/OAuth2)
```

### Tenant Hierarchy

```
TSum Platform
└── Service Provider (e.g., "inrush")
    ├── Cognito User Pool (one per service provider)
    ├── Tenant: "bp"      → database: inrush_bp
    ├── Tenant: "suncor"  → database: inrush_suncor
    └── Tenant: "shell"   → database: inrush_shell
```

---

## 2. Multi-Tenancy

### Isolation Strategy: Database-Per-Tenant

TSum uses physical database isolation. Each tenant gets its own PostgreSQL database named `{serviceProviderId}_{tenantId}` (e.g., `inrush_bp`). There are no `tenant_id` columns — queries execute against the tenant's isolated database.

There are two database tiers:

- **Control Plane DB** (`{serviceProviderID}_db`): Stores `service_providers`, `tenants`, `tenant_users`, `cp_service_provider_config`, invitation tokens.
- **Tenant DB** (`{serviceProviderID}_{tenantId}`): Stores domain data (sites, areas, assets, files, test sheets, etc.).

### Tenant Context

`backend/tenant/context.go` defines the tenant context that flows through every request:

```go
type Context struct {
    TenantID          string // e.g., "bp"
    ServiceProviderID string // e.g., "inrush"
    UserRole          string // e.g., "admin"
}

func (tc *Context) DatabaseName() string {
    return fmt.Sprintf("%s_%s", tc.ServiceProviderID, tc.TenantID)
}

func (tc *Context) S3Prefix() string {
    return fmt.Sprintf("%s/files/", tc.TenantID)
}
```

Context is stored in Go's `context.Context` and retrieved anywhere in the call stack:

```go
tc := tenant.FromContext(ctx)          // returns nil if absent
tc := tenant.MustFromContext(ctx)      // panics if absent
ctx := tenant.WithContext(ctx, tc)     // store in context
```

### Connection Manager

`backend/db/tenant_connection.go` maintains a per-tenant connection pool:

```go
// Get tenant DB from the current request context
db, err := tenantManager.GetConnectionFromContext(r.Context())

// Create a new tenant database (provisioning flow)
err := tenantManager.CreateTenantDatabase(ctx, tc)

// List all tenant databases for a provider
databases, err := tenantManager.ListTenantDatabases(ctx, "inrush")
```

The manager is thread-safe (uses `sync.RWMutex`) and re-uses existing connections per tenant.

### Middleware Chain

Every authenticated, tenant-scoped request passes through this middleware chain in `backend/routes/routes.go`:

```
RequireAuthMiddleware      (validates JWT via Cognito OIDC)
  └── RequireTenantMiddleware  (extracts tenantId + serviceProviderId from JWT claims)
        └── DatadogTenantMiddleware  (tags APM spans with tenant context)
              └── Handler (gets tenant DB via GetConnectionFromContext)
```

`RequireTenantMiddleware` in `backend/middleware/tenant.go`:

```go
func RequireTenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims, ok := user.GetClaimsFromContext(r)
        if !ok {
            http.Error(w, `{"error": "Failed to extract tenant context"}`, http.StatusBadRequest)
            return
        }
        tc := claimsToTenantContext(claims)
        if tc == nil || tc.TenantID == "" || tc.ServiceProviderID == "" {
            http.Error(w, `{"error": "Tenant context required"}`, http.StatusForbidden)
            return
        }
        ctx := tenant.WithContext(r.Context(), tc)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### JWT Claims for Tenancy

Tenant identity is carried in Cognito custom attributes embedded in the JWT:

| Cognito Attribute | JWT Claim | Example | Mutability |
|---|---|---|---|
| `custom:tenantId` | `tenantId` | `bp` | Mutable |
| `custom:serviceProviderId` | `serviceProviderId` | `inrush` | Immutable |
| `custom:userRole` | `userRole` | `admin` | Mutable |

Users without a tenant assignment receive a 403:

```json
{"error": "User configuration error: missing tenant assignment. Contact administrator."}
```

### S3 Tenant Isolation

Files are stored under tenant-scoped prefixes:

```
s3://{bucket}/
├── bp/files/{uuid}.pdf
├── suncor/files/{uuid}.pdf
└── shell/files/{uuid}.pdf
```

### Tenant Onboarding Flow

1. Insert service provider into control plane DB
2. Configure Cognito settings in `cp_service_provider_config`
3. Call `POST /api/admin/tenants` with `{"tenantId": "bp"}` (requires `superadmin` JWT)
4. System creates database `inrush_bp` and runs migrations
5. Set `custom:tenantId` and `custom:serviceProviderId` on Cognito users
6. Users log in and are routed to `inrush_bp` automatically

### Control Plane Schema

```sql
-- service_providers
CREATE TABLE service_providers (
    provider_id TEXT UNIQUE NOT NULL,  -- e.g., 'inrush'
    name TEXT NOT NULL,
    status TEXT DEFAULT 'active'
);

-- tenants
CREATE TABLE tenants (
    service_provider_id TEXT NOT NULL REFERENCES service_providers(provider_id),
    tenant_id TEXT NOT NULL,           -- e.g., 'bp'
    name TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    UNIQUE(service_provider_id, tenant_id)
);

-- cp_service_provider_config
CREATE TABLE cp_service_provider_config (
    service_provider_id TEXT UNIQUE NOT NULL,
    cognito_user_pool_id TEXT NOT NULL,
    cognito_client_id TEXT NOT NULL,
    cognito_client_secret_arn TEXT,
    cognito_domain TEXT NOT NULL,
    cognito_region TEXT DEFAULT 'us-east-1',
    cognito_redirect_uri TEXT,
    max_tenants INT DEFAULT 100,
    max_users_per_tenant INT DEFAULT 1000
);

-- tenant_users
CREATE TABLE tenant_users (
    cognito_sub TEXT NOT NULL,
    email TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    service_provider_id TEXT NOT NULL,
    role TEXT DEFAULT 'user',          -- user|admin|manager|readonly
    tsum_role TEXT,                    -- 'super_admin' for platform admins
    UNIQUE(cognito_sub, service_provider_id, tenant_id)
);
```

---

## 2. Request Flow Diagram

```
Client Request
    │
    ▼
API Gateway  (pass-through, no auth)
    │
    ▼
Lambda
    ├── RequireAuthMiddleware ──────── validates JWT via Cognito OIDC
    │
    ├── RequireTenantMiddleware ─────── extracts tenantId/serviceProviderId from JWT
    │
    ├── DatadogTenantMiddleware ─────── tags APM spans
    │
    └── Handler
            │
            └── TenantConnectionManager.GetConnectionFromContext()
                        │
                        ▼
              Tenant DB (e.g., inrush_bp)
```

---

## 3. AWS Infrastructure

### Services

| Service | Purpose | Configuration |
|---|---|---|
| **Lambda** | Go backend | 256 MB memory, 900 s timeout, VPC-enabled |
| **Aurora PostgreSQL Serverless v2** | Database | 0.5–2 ACU, database-per-tenant |
| **API Gateway** | REST API | Custom domains per environment |
| **Cognito User Pool** | Authentication | OIDC/OAuth2, custom branding, per service provider |
| **S3** | File storage + frontend hosting | CORS-enabled, tenant-prefixed paths |
| **CloudFront** | Frontend CDN | SPA routing (404 → index.html) |
| **Secrets Manager** | Database credentials | Aurora secret ARN via `AURORA_SECRET_ARN` |
| **SES** | Transactional email | User invitations |
| **VPC** | Network isolation | Lambda + Aurora in private subnets |
| **Terraform (S3 + DynamoDB)** | IaC state management | Per-instance state bucket |
| **Datadog** | APM + RUM | Lambda Extension for backend, browser SDK for frontend |

### Infrastructure as Code

TSum does not store Terraform files in the repo directly. Reusable GitHub Actions (maintained separately) wrap Terraform modules:

| Action | Purpose |
|---|---|
| `realsensesolutions/actions-aws-backend-setup` | S3 + DynamoDB Terraform backend |
| `realsensesolutions/actions-aws-network` | VPC, subnets, security groups |
| `realsensesolutions/actions-aws-bucket` | S3 with CORS configuration |
| `realsensesolutions/actions-aws-auth` | Cognito User Pool with branding |
| `realsensesolutions/actions-aws-postgres-aurora` | Aurora Serverless v2 |
| `realsensesolutions/actions-aws-function-go` | Lambda deployment (Go binary) |
| `realsensesolutions/actions-aws-api-gateway` | API Gateway + custom domain |
| `realsensesolutions/actions-aws-website` | S3 + CloudFront frontend |

### VPC Architecture

Lambda functions run in private subnets within a VPC. Aurora is only reachable within the VPC. The Lambda execution role has minimum required IAM permissions:

- S3: write to file storage bucket
- Cognito: user management (invitations)
- Secrets Manager: read Aurora credentials
- SES: send emails

### Database Credentials

Aurora credentials are stored in Secrets Manager. The Lambda reads them at startup via `AURORA_SECRET_ARN`:

```go
// backend/config/secrets.go
secret, err := secretsmanager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
    SecretId: aws.String(cfg.AuroraSecretARN),
})
// Parse JSON secret: {username, password, host, port, dbname}
```

---

## 4. Deployment Environments & CI/CD

### Environments

| Environment | Instance Name | Web Domain | API Domain | Trigger | AWS Account |
|---|---|---|---|---|---|
| **Sandbox** | `sandbox-tsum` | `sandbox.tsum.app` | `api.sandbox.tsum.app` | Push to `main` | Non-Prod |
| **Demo (RC)** | `demo-tsum` | `demo.tsum.app` | `api.demo.tsum.app` | Tag `v*rc*` | Non-Prod |
| **Production** | `inrush-tsum` | `inrush.tsum.app` | `api.inrush.tsum.app` | Tag `v*` (not RC) | Prod |

Two AWS accounts are used:
- **Non-Prod** (`AWS_ROLE_ARN`): Sandbox and Demo
- **Prod** (`AWS_ROLE_ARN_PROD`): Production

### Workflow Files

| File | Trigger | Deploys to |
|---|---|---|
| `.github/workflows/infra.yml` | Manual / called by others | Reusable infra workflow |
| `.github/workflows/on-push.yml` | Push to `main` | Sandbox |
| `.github/workflows/on-tags-rc.yml` | Tag `v*rc*` | Demo |
| `.github/workflows/on-tags.yml` | Tag `v*` | Production |

### Deployment Steps (in order)

1. Checkout repository + submodules
2. Authenticate to AWS via OIDC (`aws-actions/configure-aws-credentials`)
3. Setup Terraform backend (S3 + DynamoDB)
4. Deploy VPC/network
5. Deploy S3 file storage bucket with CORS
6. Deploy Cognito User Pool with custom branding
7. Deploy Aurora PostgreSQL Serverless v2
8. Build and deploy Lambda (Go binary)
9. Invoke Lambda with `{"type": "user-migration"}` — run DB migrations
10. Invoke Lambda with `{"type": "database-seeder"}` — run DB seeders
11. Invoke Lambda with `{"type": "asset-details-seeder"}` — seed templates
12. Deploy API Gateway with custom domain
13. Build frontend with `VITE_*` environment variables
14. Deploy frontend to S3 + CloudFront invalidation

### Lambda Environment Variables

```yaml
ENVIRONMENT: sandbox|prod
SERVICE_PROVIDER_ID: inrush
AURORA_SECRET_ARN: arn:aws:secretsmanager:us-east-1:...
COGNITO_USER_POOL_ID: us-east-1_xxx
COGNITO_CLIENT_ID: xxx
COGNITO_CLIENT_SECRET: xxx
COGNITO_DOMAIN: xxx.auth.us-east-1.amazoncognito.com
COGNITO_REDIRECT_URI: https://api.sandbox.tsum.app/oauth2/idpresponse
FRONT_END_URL: https://sandbox.tsum.app
SES_FROM_EMAIL: noreply@tsum.app
AWS_USE_DUALSTACK_ENDPOINT: true
DD_SITE: us5.datadoghq.com
DD_SERVICE: tsum.app
DD_ENV: sandbox
DD_VERSION: {git-sha}
```

### Frontend Build Variables

```yaml
VITE_BACKEND_URL: https://api.sandbox.tsum.app
VITE_DATADOG_APPLICATION_ID: xxx
VITE_DATADOG_CLIENT_TOKEN: xxx
VITE_DATADOG_SITE: us5.datadoghq.com
VITE_DATADOG_SERVICE: tsum.app
VITE_DATADOG_ENV: sandbox
VITE_DATADOG_VERSION: {git-sha}
VITE_DATADOG_SESSION_SAMPLE_RATE: 100
VITE_DATADOG_SESSION_REPLAY_SAMPLE_RATE: 100
```

### Lambda Event Handlers

`backend/lambda.go` handles both API requests and administrative invocations:

| Event Type | Purpose |
|---|---|
| API Gateway / Function URL | Regular HTTP requests |
| `user-migration` | Run database migrations |
| `database-seeder` | Run database seeders |
| `asset-details-seeder` | Seed asset templates |
| `database-reset` | Reset database (sandbox only) |
| `scheduled-notification` | EventBridge scheduled events |

---

## 5. Authentication

### Provider: AWS Cognito (OIDC/OAuth2)

Authentication is implemented in the `external/go-user-management` Git submodule.

### Full Auth Flow

```
1. User opens app → frontend calls /api/auth/profile
2. Profile returns 401 → frontend redirects to /api/auth/login
3. Backend builds Cognito authorization URL and redirects
4. User authenticates with Cognito hosted UI
5. Cognito redirects to /oauth2/idpresponse with auth code
6. Backend exchanges code for JWT tokens via Cognito token endpoint
7. Backend sets jwt cookie (HttpOnly, Secure, SameSite=Lax)
8. Backend redirects to frontend /dashboard
9. Frontend receives cookie, calls /api/auth/profile
10. Profile response contains user + tenant info
11. Zustand store updated, user sees dashboard
```

### Backend Auth Middleware

`external/go-user-management/middleware.go` — `RequireAuthMiddleware()`:

1. Look for `jwt` cookie → validate as OIDC JWT
2. If no cookie, look for `Authorization: Bearer <token>` header
3. If token has 3 dot-separated parts → validate as OIDC JWT
4. If token is opaque → lookup in Cognito via `FindUserByToken()`
5. If all fail → return 401

Claims are stored in the request context and retrieved via:

```go
claims, ok := user.GetClaimsFromContext(r)   // returns *Claims, bool
userID, ok := user.GetUserIDFromContext(r)   // returns string, bool
```

### Claims Structure

```go
type Claims struct {
    Sub               string // Cognito user ID
    Email             string
    GivenName         string
    FamilyName        string
    Username          string
    UserRole          string // custom:userRole
    TenantID          string // custom:tenantId
    ServiceProviderID string // custom:serviceProviderId
    Provider          string // "cognito" or "token"
}
```

### JWT Validation

`external/go-user-management/oidc.go` validates tokens using the Cognito OIDC discovery endpoint:

```
https://cognito-idp.{region}.amazonaws.com/{userPoolId}/.well-known/openid-configuration
```

The OIDC verifier fetches public keys automatically and caches them.

### RBAC

Role middleware lives in `backend/middleware/tenant.go`:

```go
func RequireRoleMiddleware(requiredRole string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, _ := user.GetClaimsFromContext(r)
            role := claims.UserRole
            if role != requiredRole {
                http.Error(w, `{"error": "Insufficient permissions"}`, http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

Superadmin-only routes:

```go
// backend/routes/routes.go
r.Use(middleware.RequireRoleMiddleware("superadmin"))
```

Frontend superadmin guard:

```tsx
// frontend/src/components/SuperAdminRoute.tsx
if (user["custom:userRole"] !== "superadmin") {
    return <Navigate to="/dashboard" />
}
```

### Frontend Auth State

`frontend/src/store/useStore.ts` (Zustand):

```ts
interface AuthState {
    user: AuthProfile | null
    isAuthenticated: boolean
    isLoadingAuth: boolean
    isOfflineAuth: boolean
}

// Called on app mount
checkAuth()  → GET /api/auth/profile → updates store
login()      → redirect to /api/auth/login
logout()     → GET /api/auth/logout → clears cookie + store
```

Profile is cached in localStorage (`tsum:auth-profile`) for offline support.

### Token Transmission

The frontend sends the JWT cookie automatically (`credentials: "include"`). For cross-subdomain requests the cookie is extracted and added as an `Authorization: Bearer` header:

```ts
// frontend/src/api/apiGateway.ts
const token = document.cookie.match(/(?:^|;\s*)jwt=([^;]*)/)
if (token) headers["Authorization"] = `Bearer ${token[1]}`
```

---

## 6. Backend Architecture

### Tech Stack

- **Language**: Go 1.23
- **Router**: [Chi](https://github.com/go-chi/chi) v5
- **Database driver**: `pgx` via `database/sql`
- **Query helper**: `github.com/realsensesolutions/go-database` (internal library)
- **Migrations**: `golang-migrate/migrate/v4`
- **Lambda adapter**: `github.com/awslabs/aws-lambda-go-api-proxy`
- **Auth submodule**: `external/go-user-management`

### Project Structure

```
backend/
├── app/
│   └── dependencies.go       # Dependency injection container
├── cmd/templates/            # Code generation tool for templates
├── config/
│   ├── config.go             # Environment-based configuration
│   └── secrets.go            # Secrets Manager integration
├── db/
│   ├── database.go           # DB driver registration
│   └── tenant_connection.go  # Multi-tenant connection manager
├── generated/                # Auto-generated code (do not edit)
├── internal/
│   ├── inrush/               # Sites/Areas/Assets domain
│   ├── testsheets/           # Test sheet domain
│   ├── reports/              # PDF report generation
│   └── controlplane/         # Tenant management
├── middleware/
│   └── tenant.go             # Tenant + RBAC middleware
├── migrations/
│   ├── postgres_migrator.go  # Migration runner
│   ├── service-provider/     # Control plane SQL migrations
│   └── schema/               # Tenant SQL migrations
├── routes/
│   └── routes.go             # Router + middleware setup
├── seeders/
│   ├── service-provider/     # Control plane seed data
│   └── tenant/               # Tenant seed data
├── tenant/
│   └── context.go            # Tenant context type + helpers
├── utils/
│   └── http_response.go      # Standard JSON response helpers
├── lambda.go                 # Lambda entry point
├── main.go                   # CLI (dev/migrate/seed commands)
└── server.go                 # Local HTTPS server
```

### Domain Module Pattern

Every domain lives under `internal/{domain}/` with four files:

| File | Responsibility |
|---|---|
| `models.go` | Domain structs, request/response DTOs |
| `repository.go` | All SQL queries; takes `context.Context` + `*sql.DB` |
| `handler.go` | HTTP handlers; calls repository, writes responses |
| `routes.go` | Route registration; wires handlers to paths |
| `errors.go` | Domain-specific errors; maps to HTTP status codes |

### Handler Pattern

```go
// Handlers return http.HandlerFunc closures to allow dependency injection
func HandleGetSites(tenantManager *db.TenantConnectionManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tenantDB, err := tenantManager.GetConnectionFromContext(r.Context())
        if err != nil {
            utils.WriteError(w, "Database connection failed", http.StatusInternalServerError)
            return
        }
        sites, err := GetSites(r.Context(), tenantDB)
        if err != nil {
            utils.WriteError(w, err.Error(), http.StatusInternalServerError)
            return
        }
        utils.WriteOK(w, sites, "")
    }
}
```

### Repository Pattern

```go
// Plain SQL — no ORM. Tenant isolation is at the connection level.
func GetSites(ctx context.Context, db *sql.DB) ([]Site, error) {
    query := `
        SELECT s.id, s.name, s.slug, s.location
        FROM sites s
        ORDER BY s.name ASC`
    rows, err := database.QueryContext(ctx, db, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var sites []Site
    for rows.Next() {
        var s Site
        if err := rows.Scan(&s.ID, &s.Name, &s.Slug, &s.Location); err != nil {
            return nil, err
        }
        sites = append(sites, s)
    }
    return sites, rows.Err()
}
```

### Error Handling Pattern

Domain errors defined in `errors.go`:

```go
var (
    ErrSiteNotFound = errors.New("site not found")
    ErrAreaNotFound = errors.New("area not found")
)

func IsFKViolationError(err error) bool {
    var pqErr *pq.Error
    return errors.As(err, &pqErr) && pqErr.Code == "23503"
}

func IsUniqueViolationError(err error) bool {
    var pqErr *pq.Error
    return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
```

Domain error → HTTP status mapping:

```go
func WriteInrushError(w http.ResponseWriter, err error) bool {
    switch {
    case errors.Is(err, ErrSiteNotFound):
        utils.WriteError(w, err.Error(), http.StatusNotFound)
        return true
    case IsFKViolationError(err):
        utils.WriteError(w, "Referenced resource not found", http.StatusBadRequest)
        return true
    }
    return false
}
```

### Standard Response Format

`backend/utils/http_response.go`:

```go
type StandardResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Message string      `json:"message,omitempty"`
    Error   string      `json:"error,omitempty"`
}

utils.WriteOK(w, data, "")           // 200
utils.WriteCreated(w, data, "")      // 201
utils.WriteError(w, "msg", 400)      // 4xx/5xx
```

### Dependency Injection

`backend/app/dependencies.go` holds the application dependency container:

```go
type Dependencies struct {
    Config        *config.Config
    TenantManager *db.TenantConnectionManager
    FileService   *filestorage.FileService
    // ...
}
```

Dependencies are created once at Lambda cold start and passed to handlers via function parameters — no global state.

### Local vs Lambda Execution

Both entry points use the same router:

```go
router := routes.NewRouter(deps)

// Local: backend/server.go
http.ListenAndServeTLS(":3000", "certs/localhost.crt", "certs/localhost.key", router)

// Lambda: backend/lambda.go
lambda.Start(httpadapter.NewV2(router).ProxyWithContext)
```

---

## 7. Frontend Architecture

### Tech Stack

| Tool | Version | Purpose |
|---|---|---|
| React | 18 | UI framework |
| TypeScript | 5 | Type safety |
| Vite | 5 | Build tool |
| React Router | v6 | Client-side routing |
| TanStack Query | v5 | Server state management |
| Zustand | 4 | Client state management |
| React Hook Form | 7 | Form handling |
| Zod | 3 | Schema validation |
| Tailwind CSS | 3 | Utility-first styling |
| shadcn/ui | — | Component primitives |

### Directory Structure

```
frontend/src/
├── api/
│   ├── apiGateway.ts      # Core fetch wrapper + JWT handling
│   ├── hooks.ts           # React Query hooks (useSites, useCreateSite, ...)
│   └── schema.ts          # Zod schemas for all API responses
├── components/
│   ├── atoms/             # Minimal, single-purpose components
│   ├── molecules/         # Composed from atoms
│   ├── organisms/         # Complex feature components
│   └── ui/                # shadcn/ui primitives
├── hooks/                 # Domain-specific custom hooks
├── lib/
│   └── queryClient.ts     # TanStack Query client config
├── pages/                 # Page components (export Loading + Error variants)
├── store/
│   └── useStore.ts        # Zustand store (auth + UI state)
├── templates/
│   ├── MainTemplate.tsx   # App shell layout
│   └── ControlPlaneTemplate.tsx
├── types/
│   ├── domain.ts          # Domain types (Site, Area, Asset, ...)
│   └── api.ts             # API types (AuthProfile, ApiError)
└── router.tsx             # Route definitions
```

### Routing

`frontend/src/router.tsx` uses React Router v6 layout routes:

```tsx
{
  path: "/",
  element: <MainTemplate />,
  errorElement: <UnderConstructionPage />,
  children: [
    {
      element: <ProtectedRoute />,        // layout route: checks auth
      children: [
        {
          path: "dashboard",
          element: (
            <Suspense fallback={<LoadingSitesPage />}>
              <SitesPage />
            </Suspense>
          ),
          errorElement: <ErrorSitesPage />,
        },
        // ... other protected routes
      ]
    },
    {
      element: <SuperAdminRoute />,       // layout route: checks superadmin role
      children: [/* control plane routes */]
    }
  ]
}
```

Rules:
- Every page route is wrapped in `<Suspense>` with a skeleton fallback
- Every page route has an `errorElement` for error boundary handling
- `<ProtectedRoute>` and `<SuperAdminRoute>` are layout routes that render `<Outlet />`
- No manual `<ErrorBoundary>` components inside routes — the router handles it

### Data Fetching

Always use `useSuspenseQuery` — it guarantees `data` is defined in the component:

```ts
// frontend/src/api/hooks.ts
export const useSites = () => {
  return useSuspenseQuery<Site[]>({
    queryKey: ["sites"],
    queryFn: api.getSites,
  })
}

// Component — no loading check needed
const SitesPage = () => {
  const { data: sites } = useSites()   // always defined
  if (sites.length === 0) return <EmptySitesPage />
  return <SitesList sites={sites} />
}
```

Mutations invalidate the relevant query cache on success:

```ts
export const useCreateSite = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createSite,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["sites"] }),
  })
}
```

### API Client

`frontend/src/api/apiGateway.ts` — single typed fetch wrapper:

```ts
async function apiRequest<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-TSum-Device-Id": getOrCreateDeviceId(),
  }
  const token = document.cookie.match(/(?:^|;\s*)jwt=([^;]*)/)
  if (token) headers["Authorization"] = `Bearer ${token[1]}`

  const response = await fetch(`${BACKEND_URL}${endpoint}`, {
    ...options,
    headers,
    credentials: "include",
  })

  if (response.status === 204) return null as T
  if (!response.ok) {
    const error = await response.json()
    throw new ApiError(error.error, response.status)
  }
  const json = await response.json()
  return schema.parse(json.data)   // Zod validation
}
```

### Schema Validation

All API responses are validated with Zod before reaching components. Null arrays are transformed to empty arrays (soft fail):

```ts
// frontend/src/api/schema.ts
export const sitesResponseSchema = z
  .array(siteSchema)
  .nullable()
  .transform((val) => {
    if (val === null) {
      console.error("API VIOLATION: /sites returned null instead of []")
      return []
    }
    return val
  })
```

This prevents API contract violations from crashing the UI while surfacing the problem visibly.

### Page Component Structure

Every page file exports three components:

```tsx
// frontend/src/pages/SitesPage.tsx

export const LoadingSitesPage = () => <SitesSkeleton />

export const ErrorSitesPage = () => <ErrorMessage />

const SitesPage = () => {
  const { data: sites } = useSites()
  if (sites.length === 0) return <EmptySitesPage />
  return <SitesList sites={sites} />
}

export default SitesPage
```

### State Management

- **Server state**: TanStack Query only — no manual `useState` for API data
- **Client state**: Zustand for auth, UI preferences, and shared non-server state
- **Form state**: React Hook Form — local to the form component

QueryClient configuration (`frontend/src/lib/queryClient.ts`):

```ts
new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,   // 5 minutes
      gcTime: 10 * 60 * 1000,     // 10 minutes
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status < 500) return false
        return failureCount < 3
      },
    },
  },
})
```

### Forms

React Hook Form with Zod resolver:

```tsx
const form = useForm<SiteFormData>({
  resolver: zodResolver(siteFormSchema),
  defaultValues: { name: "", description: "", location: "" },
})

return (
  <Form {...form}>
    <FormField control={form.control} name="name" render={({ field }) => (
      <FormItem>
        <FormLabel>Name</FormLabel>
        <FormControl><Input {...field} /></FormControl>
        <FormMessage />
      </FormItem>
    )} />
    <Button type="submit">Create</Button>
  </Form>
)
```

---

## 8. Database Migrations & Seeding

### Migration System

Uses `golang-migrate/migrate/v4` with embedded SQL files. Migration tracking tables:

- Control plane: `ts_sp_schema_migrations`
- Tenant: `ts_schema_migrations`

### Migration Files

```
backend/migrations/
├── service-provider/
│   └── 000_sp_schema.up.sql    # tenants, service_providers, cp_ tables
└── schema/
    └── 000_consolidated.up.sql # sites, areas, assets, files, test_sheets, ...
```

### Seeder System

Seeders are idempotent — each file is tracked in a `seeder_history` table and skipped if already run:

```
backend/seeders/
├── service-provider/
│   └── 000_placeholder.sql
└── tenant/
    ├── 001_classes.sql
    └── 003_test_sheet_templates.sql
```

Seeders run inside transactions. Files execute in alphabetical order.

### CLI Commands

```bash
cd backend

go run . migrate-up                       # All migrations + seeders
go run . migrate-sp                       # Control plane migrations only
go run . migrate-tenant --tenant=bp       # Tenant migrations only
go run . seed-tenant --tenant=bp          # Seed specific tenant
go run . create-tenant --tenant=newco     # Create new tenant + run migrations
go run . seed-asset-details               # Seed asset templates
```

### Makefile Shortcuts

```bash
make migrate-up                           # Full migration + seed
make migrate-sp                           # SP migrations only
make migrate-tenant TENANT=demo           # Tenant migrations
make seed-asset-details                   # Asset templates
make reset-db                             # Drop + recreate all databases
```

---

## 9. Local Development Environment

### Prerequisites

- Go 1.23+
- Node.js 20+ with pnpm
- Docker (for PostgreSQL)
- AWS credentials with access to the sandbox Cognito User Pool
- HTTPS certificates (see below)

### First-Time Setup

```bash
# 1. Initialize submodules
git submodule update --init --recursive

# 2. Install dependencies (frontend pnpm, Go modules, tooling)
make install

# 3. Copy and configure environment
cp backend/.env.example backend/.env
# Edit backend/.env — minimum required:
# COGNITO_USER_POOL_ID, COGNITO_CLIENT_ID, COGNITO_CLIENT_SECRET, COGNITO_DOMAIN

# 4. Generate self-signed HTTPS certificates
make certs

# 5. Start everything (database, migrations, servers)
make dev
```

After `make dev`:
- Backend: `https://localhost:3000`
- Frontend: `https://localhost:8000`
- PostgreSQL: `localhost:5432`

### Environment File

`backend/.env.example`:

```bash
ENVIRONMENT=dev
SERVICE_PROVIDER_ID=sandbox
DATABASE_URL=postgresql://tsum:tsum_password@localhost:5432/sandbox_demo?sslmode=disable
COGNITO_USER_POOL_ID=us-east-1_XXXXXXXXX
COGNITO_CLIENT_ID=abcdefg
COGNITO_CLIENT_SECRET=your-secret
COGNITO_DOMAIN=https://your-cognito.auth.us-east-1.amazoncognito.com
COGNITO_REDIRECT_URI=https://localhost:3000/oauth2/idpresponse
AWS_REGION=us-east-1
FRONT_END_URL=https://localhost:8000
S3_BUCKET_NAME=your-sandbox-bucket
SES_FROM_EMAIL=noreply@yourdomain.com
```

### Local Database

`backend/docker-compose.yml` runs PostgreSQL 15:

```yaml
services:
  postgres:
    image: postgres:15-alpine
    ports: ["5432:5432"]
    environment:
      POSTGRES_USER: tsum
      POSTGRES_PASSWORD: tsum_password
```

`backend/init-db.sh` creates two databases on startup:

```bash
psql -c "CREATE DATABASE sandbox_db;"    # control plane (service provider)
psql -c "CREATE DATABASE sandbox_demo;"  # tenant database
```

### HTTPS Requirement

Cognito OAuth2 callback requires HTTPS even locally. Vite checks for certificates at startup and exits with a helpful error if they are missing. Trust `certs/ca.crt` in your browser to avoid certificate warnings.

### Daily Commands

```bash
make dev        # Start everything
make stop       # Stop servers
make restart    # Restart backend + frontend

make db-up      # Start PostgreSQL only
make db-down    # Stop PostgreSQL (removes volumes)
make db-logs    # View database logs
```

### Quality Gates

```bash
# Frontend
cd frontend && pnpm lint
cd frontend && pnpm type-check
cd frontend && pnpm test

# Backend
cd backend && go test ./...
cd backend && go test ./tests/integration/...   # requires Docker

# Templates
make validate-templates
make compile-templates
```

### Code Generation

TSum has a code generation tool for test sheet templates:

```bash
make compile-templates
# Runs: cd backend && go run ./cmd/templates compile
```

This generates:
- `backend/generated/template_ir.go`
- `backend/generated/formulas.go`
- `frontend/src/generated/template-ir.ts`
- `frontend/src/generated/formulas.ts`

Do not hand-edit files in `backend/generated/` or `frontend/src/generated/`.

---

## 10. Replication Guide for Other SaaS Projects

This section describes how to replicate the TSum architecture in a new SaaS project.

### Repository Structure

```
your-saas/
├── backend/
│   ├── app/dependencies.go
│   ├── config/config.go
│   ├── db/tenant_connection.go
│   ├── internal/{domain}/
│   ├── middleware/tenant.go
│   ├── migrations/
│   │   ├── service-provider/
│   │   └── schema/
│   ├── routes/routes.go
│   ├── seeders/
│   ├── tenant/context.go
│   ├── utils/http_response.go
│   ├── lambda.go
│   ├── main.go
│   └── server.go
├── frontend/
│   ├── src/api/
│   │   ├── apiGateway.ts
│   │   ├── hooks.ts
│   │   └── schema.ts
│   ├── src/components/
│   ├── src/pages/
│   ├── src/store/useStore.ts
│   └── src/router.tsx
├── infra/{instance}/cors.json
├── .github/workflows/
│   ├── infra.yml
│   ├── on-push.yml
│   ├── on-tags.yml
│   └── on-tags-rc.yml
└── external/go-user-management/   # git submodule
```

### Step 1: Cognito User Pool

1. Create a Cognito User Pool with the following custom attributes:
   - `custom:tenantId` (String, mutable)
   - `custom:serviceProviderId` (String, immutable)
   - `custom:userRole` (String, mutable)
2. Create an App Client with OAuth2 enabled, `openid email profile` scopes
3. Configure callback URLs: `https://{api_domain}/oauth2/idpresponse`
4. Configure logout URLs: `https://{web_domain}`
5. Enable hosted UI with custom branding (optional)

### Step 2: Control Plane Database

Create the control plane schema (`{serviceProviderId}_db`) with:
- `service_providers`
- `tenants`
- `cp_service_provider_config` (stores Cognito config per service provider)
- `tenant_users`
- `cp_invitation_tokens`

See [Section 2](#2-multi-tenancy) for the full DDL.

### Step 3: Auth Submodule

Use `external/go-user-management` as a Git submodule. Wire it into your router:

```go
import user "github.com/realsensesolutions/go-user-management"

oauthConfig := user.OAuthConfig{
    ClientID:     cfg.Cognito.ClientID,
    ClientSecret: cfg.Cognito.ClientSecret,
    UserPoolID:   cfg.Cognito.UserPoolID,
    RedirectURI:  cfg.Cognito.RedirectURI,
    Region:       cfg.Cognito.Region,
    Domain:       cfg.Cognito.Domain,
    FrontEndURL:  cfg.FrontendURL,
    Scopes:       []string{"openid", "email", "profile"},
}

// Auth routes: /api/auth/login, /oauth2/idpresponse, /api/auth/logout, /api/auth/profile
user.SetupRoutes(r, oauthConfig)

// Protected routes
r.Group(func(r chi.Router) {
    r.Use(user.RequireAuthMiddleware(oauthConfig))
    r.Use(middleware.RequireTenantMiddleware)
    // your domain routes here
})
```

### Step 4: Multi-Tenant Middleware

Copy `backend/tenant/context.go`, `backend/db/tenant_connection.go`, and `backend/middleware/tenant.go`. The only project-specific change is `DatabaseName()` — the naming convention for your tenant databases.

### Step 5: Domain Modules

For each domain, create `internal/{domain}/` with:

```go
// models.go — domain types
type Thing struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    CreatedAt *time.Time `json:"created_at"`
}

// repository.go — SQL queries
func GetThings(ctx context.Context, db *sql.DB) ([]Thing, error) { ... }
func CreateThing(ctx context.Context, db *sql.DB, t *Thing) error { ... }

// handler.go — HTTP handlers
func HandleGetThings(tm *db.TenantConnectionManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        db, err := tm.GetConnectionFromContext(r.Context())
        if err != nil { utils.WriteError(w, "db error", 500); return }
        things, err := GetThings(r.Context(), db)
        if err != nil { utils.WriteError(w, err.Error(), 500); return }
        utils.WriteOK(w, things, "")
    }
}

// routes.go — route registration
func SetupRoutes(r chi.Router, tm *db.TenantConnectionManager) {
    r.Get("/api/things", HandleGetThings(tm))
    r.Post("/api/things", HandleCreateThing(tm))
}

// errors.go — domain errors
var ErrThingNotFound = errors.New("thing not found")
```

### Step 6: Frontend Page Pattern

For each page:

```tsx
// pages/ThingsPage.tsx

export const LoadingThingsPage = () => <ThingsSkeleton />
export const ErrorThingsPage = () => <ErrorMessage />

const ThingsPage = () => {
  const { data: things } = useThings()   // useSuspenseQuery
  if (things.length === 0) return <EmptyThingsPage />
  return <ThingsList things={things} />
}

export default ThingsPage
```

Register in router:

```tsx
{
  path: "things",
  element: (
    <Suspense fallback={<LoadingThingsPage />}>
      <ThingsPage />
    </Suspense>
  ),
  errorElement: <ErrorThingsPage />,
}
```

### Step 7: CI/CD Pipeline

Create `.github/workflows/infra.yml` as a reusable workflow that accepts these inputs:

```yaml
inputs:
  environment:        # sandbox | prod
  instance_name:      # e.g., sandbox-yourapp
  web_domain:         # e.g., sandbox.yourapp.com
  api_domain:         # e.g., api.sandbox.yourapp.com
  service_provider_id: # e.g., yoursp
  ses_from_email:     # noreply@yourapp.com
```

Deploy in this order:
1. Terraform backend (S3 + DynamoDB)
2. VPC/Network
3. S3 file bucket
4. Cognito User Pool
5. Aurora PostgreSQL Serverless v2
6. Lambda (Go binary)
7. Invoke Lambda: `user-migration`
8. Invoke Lambda: `database-seeder`
9. API Gateway
10. Frontend build + S3/CloudFront

Create trigger workflows:
- `on-push.yml` — sandbox, triggers on push to `main`
- `on-tags-rc.yml` — demo/staging, triggers on `v*rc*` tags
- `on-tags.yml` — production, triggers on `v*` tags

Use OIDC for AWS authentication (no long-lived access keys):

```yaml
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
    aws-region: us-east-1
```

---

## 11. Key Design Decisions

| Decision | Rationale |
|---|---|
| **Database-per-tenant** | Strongest data isolation. No `tenant_id` columns needed — queries are simpler and cannot accidentally leak across tenants. Easier to backup/restore individual tenants. |
| **Raw SQL, no ORM** | Full PostgreSQL feature access (triggers, views, CTEs, window functions). Explicit queries are easier to optimize and reason about. |
| **Cognito custom attributes for tenancy** | JWT carries tenant context — no database lookup per request to determine tenant. Immutable `serviceProviderId` prevents privilege escalation. |
| **HttpOnly cookies for JWT** | Not accessible to JavaScript, immune to XSS-based token theft. SameSite=Lax prevents CSRF on same-origin redirects. |
| **OIDC token validation** | Stateless validation using Cognito's public JWKS endpoint. No session store required. Works identically locally and in Lambda. |
| **`useSuspenseQuery` exclusively** | Eliminates defensive coding (`data || []`, loading checks) in components. Loading and error states are handled at the route boundary. |
| **Zod soft-fail transforms** | API contract violations don't crash the UI. Null-to-empty-array transforms with console errors make violations visible in monitoring without breaking the user experience. |
| **Reusable GitHub Actions for IaC** | Terraform configuration is version-controlled separately from application code. Multiple projects share the same infrastructure primitives without duplication. |
| **Git submodules for shared Go libraries** | `go-user-management` and `go-database` are shared across projects. Submodules allow pinning to specific commits while accepting upstream updates explicitly. |
| **Chi router** | Lightweight, stdlib-compatible, supports middleware chaining and sub-routers cleanly. No magic. |
| **Lambda + Aurora Serverless v2** | Aurora scales to zero when idle (cost for sandbox/demo). Lambda eliminates server management. Both scale automatically under load. |

---

## 12. Reference Files Index

### Multi-Tenancy

| File | Purpose |
|---|---|
| `backend/tenant/context.go` | Tenant context type, `WithContext`, `FromContext` |
| `backend/db/tenant_connection.go` | Per-tenant connection pool manager |
| `backend/middleware/tenant.go` | `RequireTenantMiddleware`, `RequireRoleMiddleware` |
| `backend/migrations/service-provider/000_sp_schema.up.sql` | Control plane schema |
| `backend/migrations/schema/000_consolidated.up.sql` | Tenant schema |
| `backend/docs/MULTI_TENANT_IMPLEMENTATION.md` | Multi-tenancy implementation details |
| `backend/docs/TENANT_MANAGEMENT.md` | Tenant onboarding guide |

### Authentication

| File | Purpose |
|---|---|
| `external/go-user-management/middleware.go` | `RequireAuthMiddleware`, `GetClaimsFromContext` |
| `external/go-user-management/oidc.go` | OIDC token validation via Cognito JWKS |
| `external/go-user-management/oauth_handlers.go` | Login, logout, callback, profile handlers |
| `external/go-user-management/oauth2.go` | OAuth2 service implementation |
| `backend/routes/routes.go` | Route setup with auth + tenant middleware |
| `frontend/src/store/useStore.ts` | Auth state (Zustand) |
| `frontend/src/api/apiGateway.ts` | JWT cookie extraction, fetch wrapper |
| `frontend/src/components/ProtectedRoute.tsx` | Auth layout route guard |
| `frontend/src/components/SuperAdminRoute.tsx` | Superadmin layout route guard |

### Backend

| File | Purpose |
|---|---|
| `backend/app/dependencies.go` | Dependency injection container |
| `backend/config/config.go` | Environment configuration |
| `backend/config/secrets.go` | Secrets Manager integration |
| `backend/utils/http_response.go` | `WriteOK`, `WriteError`, `WriteCreated` |
| `backend/internal/inrush/handler.go` | Example handler implementation |
| `backend/internal/inrush/repository.go` | Example repository with raw SQL |
| `backend/internal/inrush/errors.go` | Domain error definitions and FK checks |
| `backend/lambda.go` | Lambda entry point |
| `backend/server.go` | Local HTTPS server |
| `backend/main.go` | CLI commands (dev, migrate, seed) |

### Frontend

| File | Purpose |
|---|---|
| `frontend/src/router.tsx` | Route definitions with Suspense + error boundaries |
| `frontend/src/api/apiGateway.ts` | API client with typed responses |
| `frontend/src/api/schema.ts` | Zod schemas with soft-fail transforms |
| `frontend/src/api/hooks.ts` | `useSuspenseQuery` / `useMutation` hooks |
| `frontend/src/lib/queryClient.ts` | QueryClient with retry and stale time config |
| `frontend/src/store/useStore.ts` | Zustand auth + UI state |
| `frontend/src/pages/SitesPage.tsx` | Example page with Loading/Error exports |

### Infrastructure

| File | Purpose |
|---|---|
| `.github/workflows/infra.yml` | Reusable infrastructure deployment workflow |
| `.github/workflows/on-push.yml` | Sandbox trigger (push to main) |
| `.github/workflows/on-tags.yml` | Production trigger (version tag) |
| `.github/workflows/on-tags-rc.yml` | Demo trigger (RC tag) |
| `infra/{instance}/cors.json` | S3 CORS config per environment |
| `backend/config/config.go` | Maps env vars to typed config struct |

### Development

| File | Purpose |
|---|---|
| `Makefile` | Top-level commands (`dev`, `stop`, `migrate-up`, etc.) |
| `make/dev.mk` | Server start/stop commands |
| `make/database.mk` | DB container and migration commands |
| `backend/docker-compose.yml` | Local PostgreSQL container |
| `backend/.env.example` | Environment variable template |
| `backend/init-db.sh` | Creates initial databases in Docker |
