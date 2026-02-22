# Cognito actions-aws-auth Adaptation Summary

**Date:** February 2025  
**Option:** B — Adapt llm-control-plane to use actions-aws-auth attributes

---

## Overview

The llm-control-plane backend was adapted to use the custom attributes created by `realsensesolutions/actions-aws-auth` instead of the previously expected `custom:org_id`, `custom:app_id`, and `custom:role`. This allows the project to work with Cognito User Pools deployed via the GitHub Actions workflow without modifying the actions-aws-auth Terraform.

---

## Attribute Mapping

| actions-aws-auth (Cognito) | llm-control-plane (Backend) | Notes |
|----------------------------|----------------------------|-------|
| `custom:tenantId` | `OrgID` | Required; must be UUID format |
| `custom:userRole` | `Role` | Required; values: admin, developer, user, viewer |
| `custom:app_id` | `AppID` | Optional; not in actions-aws-auth schema; omit for new users |

---

## Files Modified

### 1. `backend/cognito/validator.go`

- **Claims struct** (lines 62–66): Updated JSON tags:
  - `OrgID`: `json:"custom:org_id"` → `json:"custom:tenantId"`
  - `Role`: `json:"custom:role"` → `json:"custom:userRole"`
  - `AppID`: kept `json:"custom:app_id"` (optional, for backward compatibility)
- **Error message**: `invalid org_id UUID` → `invalid custom:tenantId UUID`

### 2. `backend/cognito/claims.go`

- **parseClaims**: Error messages updated to reference `custom:tenantId` and `custom:userRole`
- **ExtractOrgID**: Error messages updated
- **ValidateCustomClaims**: Error messages and comments updated

### 3. `backend/middleware/auth_middleware.go`

- **ExtractTenant**: AppID is now optional; uses `uuid.Nil` when empty (actions-aws-auth does not create `custom:app_id`)
- **extractToken**: Added support for `session` cookie (set by auth handler after OAuth callback) in addition to `auth_token`

### 4. `scripts/create-test-user.ps1`

- User attributes changed from `custom:org_id`, `custom:app_id`, `custom:role` to `custom:tenantId`, `custom:userRole`
- `AppId` parameter default set to empty; only added when provided (app_id not in actions-aws-auth schema)
- Default `OrgId` changed to UUID format: `11111111-1111-1111-1111-111111111111`

### 5. `scripts/list-test-users.ps1`

- Attribute names updated: `custom:org_id` → `custom:tenantId`, `custom:role` → `custom:userRole`

### 6. `docs/setup/COGNITO_DEPLOYMENT.md`

- Custom attribute documentation updated to reflect actions-aws-auth schema
- AWS CLI examples updated
- Attribute table updated

### 7. Test Files

- **backend/cognito/claims_test.go**: Assertions updated for new error messages
- **backend/cognito/validator_test.go**: Assertion for `TestValidateToken_MissingOrgID` updated

---

## Creating Test Users

Use the updated script with `custom:tenantId` and `custom:userRole`:

```powershell
.\scripts\create-test-user.ps1 `
    -UserPoolId "us-east-1_XXXXXXXXX" `
    -Email "dev@example.com" `
    -OrgId "550e8400-e29b-41d4-a716-446655440000" `
    -Role "developer"
```

Or via AWS CLI:

```powershell
aws cognito-idp admin-create-user `
    --user-pool-id us-east-1_XXXXXXXXX `
    --username "dev@example.com" `
    --user-attributes `
        Name=email,Value=dev@example.com `
        Name=email_verified,Value=true `
        Name=custom:tenantId,Value=550e8400-e29b-41d4-a716-446655440000 `
        Name=custom:userRole,Value=developer `
    --temporary-password "TempPass123!" `
    --message-action SUPPRESS `
    --region us-east-1
```

---

## Backward Compatibility

- **AppID**: The backend still accepts `custom:app_id` when present. If you add this attribute to your Cognito User Pool schema separately, it will work. For pools created by actions-aws-auth, AppID will be empty and the backend uses `uuid.Nil` for org-level operations.
- **Session cookie**: The auth middleware now checks both `auth_token` and `session` cookies, so OAuth callback flow works correctly.

---

## Verification

Run auth-related tests:

```powershell
cd backend
go test ./cognito/... ./middleware/... ./auth/... ./handlers/... -count=1
```
