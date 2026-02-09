# Conventions Documentation

This directory contains comprehensive guidelines and conventions for the LLM Control Plane project.

---

## 📚 Available Documents

### [GO_CONVENTIONS.md](GO_CONVENTIONS.md)
**664 lines | Coding Standards**

Comprehensive Go coding standards covering:
- Module structure and package organization
- Naming conventions (variables, functions, interfaces, constants)
- Code style and formatting
- Error handling patterns
- Testing standards (table-driven tests, coverage)
- Dependency management
- Configuration management
- Logging and observability
- Security best practices

**When to use:** Before writing any Go code, during code reviews

---

### [GROWTH_GUIDELINES.md](GROWTH_GUIDELINES.md)
**700+ lines | Scaling & Extension**

Guidelines for extending and scaling the system:

#### Feature Flags
- Where they live (`runtimeconfig/features.go`)
- Storage options (env vars, database, external service)
- Usage patterns and lifecycle
- Naming conventions

#### Runtime Configuration
- Configuration sources (env, AWS Secrets, files, database)
- Hot-reload support
- Static vs dynamic config
- Per-tenant overrides

#### Adding New Providers
- Step-by-step process with code examples
- Provider interface implementation
- Registry pattern
- Testing requirements
- Documentation checklist

#### Control Plane vs Runtime Plane
- Definitions and characteristics
- Separation strategies (monolithic → logical → physical)
- Request flow comparison
- Data access patterns
- Scaling considerations

#### Scaling Strategies
- When to scale each component
- Caching layers (L1, L2, L3)
- Multi-tenancy approaches
- Migration patterns

**When to use:** When adding features, providers, or scaling the system

---

## 🎯 Quick Reference

### For New Contributors
1. Read [GO_CONVENTIONS.md](GO_CONVENTIONS.md) first
2. Review [GROWTH_GUIDELINES.md](GROWTH_GUIDELINES.md) for architecture patterns
3. Check [CONTRIBUTING.md](../../CONTRIBUTING.md) for workflow

### For Adding Features
1. Check if feature flag needed → [GROWTH_GUIDELINES.md#feature-flags](GROWTH_GUIDELINES.md#feature-flags)
2. Follow coding standards → [GO_CONVENTIONS.md](GO_CONVENTIONS.md)
3. Write tests → [GO_CONVENTIONS.md#testing-standards](GO_CONVENTIONS.md#testing-standards)

### For Adding Providers
1. Follow provider checklist → [GROWTH_GUIDELINES.md#adding-new-providers](GROWTH_GUIDELINES.md#adding-new-providers)
2. Implement interface → [GO_CONVENTIONS.md#interfaces](GO_CONVENTIONS.md#interfaces)
3. Add tests → [GO_CONVENTIONS.md#testing-standards](GO_CONVENTIONS.md#testing-standards)

### For Scaling
1. Identify bottleneck → [GROWTH_GUIDELINES.md#scaling-considerations](GROWTH_GUIDELINES.md#scaling-considerations)
2. Check caching strategy → [GROWTH_GUIDELINES.md#caching-strategy](GROWTH_GUIDELINES.md#caching-strategy)
3. Consider plane separation → [GROWTH_GUIDELINES.md#control-plane-vs-runtime-plane](GROWTH_GUIDELINES.md#control-plane-vs-runtime-plane)

---

## 📖 Related Documentation

- **[README.md](../../README.md)** - Project overview
- **[CONTRIBUTING.md](../../CONTRIBUTING.md)** - Contribution guidelines
- **[QUICKSTART.md](../../QUICKSTART.md)** - Quick reference
- **[ARCHITECTURE.md](../approach/ARCHITECTURE.md)** - System architecture

---

## 🔄 Document Lifecycle

### Updates
These documents should be updated when:
- New patterns emerge from implementation
- Architecture decisions change
- Best practices evolve
- New technologies are adopted

### Review
- **Quarterly review** of conventions
- **Post-implementation review** of new features
- **Community feedback** incorporation

### Versioning
- Documents include version numbers and last updated dates
- Major changes should be noted in commit messages
- Breaking convention changes require team discussion

---

**Maintained by:** Development Team  
**Last Updated:** February 9, 2026
