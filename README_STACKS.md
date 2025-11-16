# Terragrunt Stacks Support - Quick Start Guide

## 🎯 What Are Stacks?

**Stacks** are logical groupings of related Terragrunt modules that should be managed together as a single unit in Atlantis. Instead of treating each Terragrunt module as a separate Atlantis project, stacks allow you to:

- Group all modules for an environment (dev, staging, production)
- Organize modules by service (auth-service, api-gateway, database)
- Create infrastructure layers (networking, data, application)
- Define cross-cutting concerns (security, monitoring, compliance)

## 📁 Files in This Repository

### Documentation
- **`STACKS_IMPLEMENTATION_PROPOSAL.md`** - Complete 60+ page technical proposal
- **`IMPLEMENTATION_SUMMARY.md`** - Executive summary and quick reference
- **`README_STACKS.md`** (this file) - Quick start guide

### Code
- **`cmd/stack.go`** - Core stack data structures and manager
- **`cmd/parse_stack_file.go`** - YAML/JSON parsing and validation
- **`cmd/stack_test.go`** - Comprehensive test suite
- **`cmd/generate_integration_example.go.example`** - Integration guide

### Examples
- **`test_examples/stacks_basic/`** - Basic stack with explicit modules
- **`test_examples/stacks_with_patterns/`** - Advanced with glob patterns
- **`cmd/golden/stacks_basic.yaml`** - Expected Atlantis output

## 🚀 Quick Start

### Step 1: Create Stack Definition File

Create `atlantis-stacks.yaml` in your repository root:

```yaml
version: 1
stacks:
  - name: production-environment
    description: Complete production infrastructure
    include:
      - "environments/production/**"
    exclude:
      - "environments/production/experimental/**"
    depends_on:
      - shared-infrastructure
    atlantis:
      workflow: production
      autoplan: false
      apply_requirements:
        - approved
        - mergeable
      execution_order_group: 100

  - name: shared-infrastructure
    description: Shared resources
    modules:
      - shared/vpc
      - shared/dns
      - shared/monitoring
    atlantis:
      workflow: shared
      autoplan: true
      execution_order_group: 1
```

### Step 2: Generate Atlantis Configuration

```bash
# Generate with stack support
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --stack-level-projects \
  --output atlantis.yaml
```

### Step 3: Review Generated Configuration

The tool will generate Atlantis projects for each stack:

```yaml
version: 3
projects:
  - name: shared-infrastructure
    dir: shared
    workflow: shared
    workspace: shared-infrastructure
    autoplan:
      enabled: true
      when_modified:
        - "shared/**/*.hcl"
        - "shared/**/*.tf*"
    execution_order_group: 1

  - name: production-environment
    dir: environments/production
    workflow: production
    workspace: production-environment
    autoplan:
      enabled: false
      when_modified:
        - "environments/production/**/*.hcl"
        - "environments/production/**/*.tf*"
        - "shared/**/*.hcl"  # dependency
    apply_requirements:
      - approved
      - mergeable
    depends_on:
      - shared-infrastructure
    execution_order_group: 100
```

## 📖 Common Use Cases

### Environment-Based Stacks

```yaml
stacks:
  - name: dev
    include: ["environments/dev/**"]
    atlantis: {workflow: dev, autoplan: true}
  
  - name: staging
    include: ["environments/staging/**"]
    depends_on: [dev]
    atlantis: {workflow: staging, autoplan: true}
  
  - name: production
    include: ["environments/prod/**"]
    depends_on: [staging]
    atlantis: {workflow: production, autoplan: false, apply_requirements: [approved]}
```

### Service-Based Stacks

```yaml
stacks:
  - name: platform
    modules: [shared/vpc, shared/dns, shared/monitoring]
    atlantis: {execution_order_group: 1}
  
  - name: auth-service
    include: ["services/auth/**"]
    depends_on: [platform]
    atlantis: {execution_order_group: 10}
  
  - name: api-gateway
    include: ["services/api/**"]
    depends_on: [platform, auth-service]
    atlantis: {execution_order_group: 20}
```

### Layer-Based Stacks

```yaml
stacks:
  - name: networking
    include: ["**/networking/**"]
    atlantis: {execution_order_group: 1}
  
  - name: databases
    include: ["**/databases/**", "**/storage/**"]
    depends_on: [networking]
    atlantis: {execution_order_group: 2}
  
  - name: applications
    include: ["**/apps/**", "**/services/**"]
    depends_on: [networking, databases]
    atlantis: {execution_order_group: 3}
```

## 🎛️ Configuration Options

### CLI Flags

```bash
# Stack enablement
--enable-stacks                    # Enable stack support (required)
--stack-definition-file string     # Path to YAML/JSON file (default: atlantis-stacks.yaml)

# Project generation
--stack-level-projects            # Generate one project per stack
--module-level-projects           # Generate traditional per-module projects (default: true)
                                  # Can use both together!

# Discovery
--infer-stacks                    # Auto-detect stacks from directory structure
--stack-directory-depth int       # Directory depth for inference (default: 2)
--stack-marker-file string        # Marker file name (default: .atlantis-stack)

# Validation
--validate-stack-coverage         # Ensure all modules belong to a stack
--allow-multi-stack              # Allow modules in multiple stacks (default: true)

# Customization
--stack-workspace-prefix string   # Prefix for stack workspace names
```

### Stack Definition Options

```yaml
stacks:
  - name: required-unique-name
    description: Optional description
    
    # Module selection (choose one)
    modules: [explicit, list, of, modules]
    # OR
    include: [glob, patterns]
    exclude: [exclusion, patterns]
    
    # Dependencies
    depends_on: [other, stack, names]
    
    # Atlantis configuration
    atlantis:
      workflow: custom-workflow-name
      autoplan: true|false
      parallel: true|false
      apply_requirements: [approved, mergeable]
      execution_order_group: 10
      workspace: custom-workspace
      terraform_version: "1.5.0"
```

## 🔄 Migration from Module-Only

### Phase 1: Test with Existing Setup

```bash
# Your existing command still works - no changes needed!
terragrunt-atlantis-config generate --output atlantis.yaml
```

### Phase 2: Preview with Inference

```bash
# See what stacks would be detected
terragrunt-atlantis-config generate \
  --enable-stacks \
  --infer-stacks \
  --stack-directory-depth 2 \
  --output atlantis-preview.yaml

# Review the output
cat atlantis-preview.yaml
```

### Phase 3: Create Explicit Definitions

Based on the preview, create `atlantis-stacks.yaml`:

```bash
# Generate template to get started
cat > atlantis-stacks.yaml << 'EOF'
version: 1
stacks:
  - name: my-first-stack
    include: ["path/to/modules/**"]
    atlantis:
      workflow: default
      autoplan: true
EOF
```

### Phase 4: Generate with Stacks

```bash
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --stack-level-projects \
  --output atlantis.yaml
```

### Phase 5: Fine-Tune

Add more stacks, dependencies, and Atlantis configurations as needed.

## 🧪 Testing

### Run Example Tests

```bash
# Navigate to repo
cd /home/dboulas/work/dene14/repos/terragrunt-atlantis-config

# Run stack-specific tests
go test -v ./cmd -run TestParseStackDefinitionFile
go test -v ./cmd -run TestMatchGlobPattern
go test -v ./cmd -run TestStackManager

# Run all tests
make test
```

### Test with Your Own Repo

```bash
# Test without making changes
terragrunt-atlantis-config generate \
  --enable-stacks \
  --infer-stacks \
  --root /path/to/your/repo \
  > test-output.yaml

# Review output
less test-output.yaml
```

## 🎨 Advanced Patterns

### Mixed Stack and Module Projects

Generate both stack-level AND module-level projects:

```bash
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --stack-level-projects \
  --module-level-projects \
  --output atlantis.yaml
```

This creates:
- High-level stack projects for coordinated operations
- Individual module projects for granular changes

### Multi-Stack Membership

Allow modules to belong to multiple stacks:

```yaml
stacks:
  - name: production-environment
    include: ["environments/production/**"]
  
  - name: critical-infrastructure
    modules:
      - environments/production/databases/main
      - environments/production/networking/vpc
      - shared/dns
  
  - name: all-databases
    include: ["**/databases/**"]
```

With `--allow-multi-stack`, the production database module appears in all three stacks.

### Dynamic Workspaces

```bash
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --create-workspace \
  --stack-workspace-prefix "env-" \
  --output atlantis.yaml
```

Generates unique workspaces like `env-production-environment`.

## 📊 Benefits

### Before (Module-Only)
```yaml
projects:
  - {name: prod-vpc, dir: env/prod/vpc}
  - {name: prod-db, dir: env/prod/db}
  - {name: prod-app1, dir: env/prod/app1}
  - {name: prod-app2, dir: env/prod/app2}
  - {name: prod-app3, dir: env/prod/app3}
  # ... 50 more modules ...
```

### After (With Stacks)
```yaml
projects:
  - name: production-environment
    dir: env/prod
    when_modified: ["env/prod/**"]
    depends_on: [shared-infrastructure]
```

**Results:**
- ✅ 55 projects → 1 stack project
- ✅ Clearer intent and organization
- ✅ Faster Atlantis plan/apply operations
- ✅ Better dependency management
- ✅ Easier to understand and maintain

## 🐛 Troubleshooting

### "No stacks defined"
- Check `atlantis-stacks.yaml` exists
- Verify `version: 1` is present
- Ensure at least one stack is defined

### "Stack 'X' must specify either 'include' patterns or 'modules' list"
- Each stack needs either `modules: [...]` or `include: [...]`

### Modules not matching patterns
- Check glob patterns are relative to git root
- Use `--infer-stacks` to see auto-detected groupings
- Test patterns: `"environments/production/**"` matches all files under that path

### Performance issues
- Use specific include patterns instead of `**/*`
- Consider `--no-module-level-projects` for large repos
- Increase `--num-executors` for parallelization

## 🤝 Contributing

See the implementation proposal for:
- Architecture details
- Development guidelines
- Testing requirements
- Code review checklist

## 📚 Further Reading

- **Full Proposal**: See `STACKS_IMPLEMENTATION_PROPOSAL.md` for complete technical details
- **Summary**: See `IMPLEMENTATION_SUMMARY.md` for executive overview
- **Integration Guide**: See `cmd/generate_integration_example.go.example`
- **Test Examples**: Browse `test_examples/stacks_*/`

## ❓ FAQ

**Q: Is this backward compatible?**  
A: Yes! Stack support is completely opt-in via `--enable-stacks`. Without this flag, behavior is unchanged.

**Q: Can I use both stacks and individual module projects?**  
A: Yes! Use both `--stack-level-projects` and `--module-level-projects` together.

**Q: Do I need to change my Terragrunt configs?**  
A: No! Stacks are defined separately in `atlantis-stacks.yaml`. No changes to `.hcl` files needed.

**Q: What if a module matches multiple stacks?**  
A: With `--allow-multi-stack` (default), modules can belong to multiple stacks. Each stack gets its own project.

**Q: How do stack dependencies work?**  
A: Use `depends_on: [stack-name]` in your stack definition. Atlantis will respect the execution order.

**Q: Can I see what stacks would be created before committing?**  
A: Yes! Run with `--output test.yaml` to preview, or use `--infer-stacks` to see auto-detection.

**Q: Does this work with existing Atlantis workflows?**  
A: Yes! Stacks can use custom workflows via `atlantis.workflow` in the stack definition.

**Q: What about state management?**  
A: Stacks don't change Terraform state. Each module maintains its own state as usual.

## 🎯 Next Steps

1. ✅ Review the implementation proposal
2. ✅ Examine the code examples
3. ✅ Test with the provided examples
4. ⏭️ Try with your own repository
5. ⏭️ Provide feedback and suggestions

## 📞 Support

For questions or issues:
1. Review the full proposal in `STACKS_IMPLEMENTATION_PROPOSAL.md`
2. Check test examples in `test_examples/stacks_*/`
3. Look at integration example in `cmd/generate_integration_example.go.example`

---

**Status**: Proposal and prototype implementation (not yet merged)  
**Estimated Implementation Time**: 6-8 weeks full, 3-4 weeks MVP  
**Backward Compatibility**: 100% - completely opt-in feature


