-- Additional composite indexes for policy queries
-- Index for org-wide policies (org_id with null app_id)
CREATE INDEX idx_policies_org_wide ON policies(org_id) WHERE app_id IS NULL;

-- Index for app-specific policies
CREATE INDEX idx_policies_org_app ON policies(org_id, app_id) WHERE app_id IS NOT NULL;

-- Index for user-specific policies
CREATE INDEX idx_policies_org_user ON policies(org_id, user_id) WHERE user_id IS NOT NULL;

-- Index for enabled policies by org and type
CREATE INDEX idx_policies_org_type_enabled ON policies(org_id, policy_type, enabled);

-- Index for policy priority ordering
CREATE INDEX idx_policies_org_priority ON policies(org_id, priority DESC);

-- GIN index for JSONB config field to enable efficient querying
CREATE INDEX idx_policies_config_gin ON policies USING GIN (config);

-- Specific JSONB path indexes for common policy configurations
CREATE INDEX idx_policies_config_enabled ON policies((config->>'enabled'));
CREATE INDEX idx_policies_config_provider ON policies((config->>'primary_provider'));

-- Index for active policies sorted by priority
CREATE INDEX idx_policies_active_priority ON policies(org_id, enabled, priority DESC) 
    WHERE enabled = TRUE;

-- Composite index for finding policies by org, app, and type
CREATE INDEX idx_policies_org_app_type ON policies(org_id, app_id, policy_type);

-- Index for policies by updated time (useful for cache invalidation)
CREATE INDEX idx_policies_updated_at ON policies(updated_at);
