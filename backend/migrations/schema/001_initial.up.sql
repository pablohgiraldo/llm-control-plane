-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_organizations_slug ON organizations(slug);
CREATE INDEX idx_organizations_created_at ON organizations(created_at);

-- Applications table
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    api_key_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_applications_org_id ON applications(org_id);
CREATE INDEX idx_applications_created_at ON applications(created_at);
CREATE UNIQUE INDEX idx_applications_api_key_hash ON applications(api_key_hash);

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL,
    cognito_sub VARCHAR(255) NOT NULL UNIQUE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'member', 'viewer')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_org_id ON users(org_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_cognito_sub ON users(cognito_sub);
CREATE INDEX idx_users_created_at ON users(created_at);

-- Policies table
CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    app_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    policy_type VARCHAR(50) NOT NULL CHECK (policy_type IN (
        'rate_limit', 'budget', 'routing', 'pii_detection', 
        'injection_guard', 'rag', 'retry', 'fallback', 'load_balance'
    )),
    config JSONB NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_policies_org_id ON policies(org_id);
CREATE INDEX idx_policies_app_id ON policies(app_id);
CREATE INDEX idx_policies_user_id ON policies(user_id);
CREATE INDEX idx_policies_policy_type ON policies(policy_type);
CREATE INDEX idx_policies_enabled ON policies(enabled);
CREATE INDEX idx_policies_created_at ON policies(created_at);

-- Audit logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    app_id UUID REFERENCES applications(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    details JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    request_id VARCHAR(255),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- LLM-specific fields
    model VARCHAR(100),
    provider VARCHAR(100),
    tokens_used INTEGER,
    cost DECIMAL(10, 6),
    latency_ms INTEGER,
    status_code INTEGER,
    error_message TEXT
);

CREATE INDEX idx_audit_logs_org_id ON audit_logs(org_id);
CREATE INDEX idx_audit_logs_app_id ON audit_logs(app_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX idx_audit_logs_resource_id ON audit_logs(resource_id);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_logs_request_id ON audit_logs(request_id);
CREATE INDEX idx_audit_logs_provider ON audit_logs(provider);
CREATE INDEX idx_audit_logs_model ON audit_logs(model);

-- Budget tracking table
CREATE TABLE budget_tracking (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    app_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    period VARCHAR(20) NOT NULL CHECK (period IN ('daily', 'monthly', 'yearly')),
    period_start TIMESTAMP NOT NULL,
    total_cost DECIMAL(10, 6) NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, app_id, period, period_start)
);

CREATE INDEX idx_budget_tracking_org_id ON budget_tracking(org_id);
CREATE INDEX idx_budget_tracking_app_id ON budget_tracking(app_id);
CREATE INDEX idx_budget_tracking_period_start ON budget_tracking(period_start);
CREATE INDEX idx_budget_tracking_period ON budget_tracking(period);

-- Rate limit requests table (for PostgreSQL-based rate limiting)
CREATE TABLE rate_limit_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rate_limit_requests_key ON rate_limit_requests(key);
CREATE INDEX idx_rate_limit_requests_timestamp ON rate_limit_requests(timestamp);
CREATE INDEX idx_rate_limit_requests_key_timestamp ON rate_limit_requests(key, timestamp);
