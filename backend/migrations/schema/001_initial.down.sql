-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS rate_limit_requests;
DROP TABLE IF EXISTS budget_tracking;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS organizations;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";
