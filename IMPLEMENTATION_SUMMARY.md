# Terragrunt Stacks Implementation - Summary

## Overview

This repository contains a comprehensive proposal and prototype implementation for adding Terragrunt "stacks" support to `terragrunt-atlantis-config`. 

## Documents Created

### 1. Main Proposal Document
**File**: `STACKS_IMPLEMENTATION_PROPOSAL.md`

Comprehensive 60+ page proposal covering:
- 5 implementation variants with detailed pros/cons
- Complete technical specifications
- Migration strategies
- Example configurations
- Implementation timeline (6-8 weeks)
- Testing requirements

### 2. Prototype Code

#### Core Stack Infrastructure
- **`cmd/stack.go`**: Core data structures and stack manager
  - `Stack` struct
  - `StackManager` for orchestration
  - Stack discovery and project generation logic

- **`cmd/parse_stack_file.go`**: External file parsing
  - YAML/JSON stack definition parsing
  - Glob pattern matching
  - Module-to-stack assignment
  - Validation logic
  - Template generation

- **`cmd/stack_test.go`**: Comprehensive test suite
  - Unit tests for parsing
  - Validation tests
  - Pattern matching tests
  - Integration tests

### 3. Test Examples

#### Basic Stack Example
**Directory**: `test_examples/stacks_basic/`
- Simple stack with explicit module list
- Stack dependencies
- Golden file for expected output

#### Pattern-Based Example
**Directory**: `test_examples/stacks_with_patterns/`
- Glob pattern matching
- Include/exclude patterns
- Multi-environment setup
- Shared infrastructure

## Implementation Variants (Summary)

### Recommended: Phased Approach

#### Phase 1: External Definition File (4 weeks)
**Priority**: High  
**Effort**: Medium  
**Impact**: Immediate value

- Stack definitions in `atlantis-stacks.yaml`
- Glob pattern matching
- No changes to Terragrunt configs
- Full backward compatibility

**Example**:
```yaml
version: 1
stacks:
  - name: production
    include: ["environments/production/**"]
    atlantis:
      workflow: production
      autoplan: false
```

#### Phase 2: Directory Inference (2 weeks)
**Priority**: Medium  
**Effort**: Low  
**Impact**: Convenience

- Automatic stack detection from directory structure
- Optional marker files
- Convention over configuration

#### Phase 3: Tag Support (2 weeks)
**Priority**: Medium  
**Effort**: Low  
**Impact**: Flexibility

- Module-level stack assignment via locals
- Multi-stack membership
- Module-specific overrides

### Alternative Variants (Detailed in Proposal)

1. **Variant 1: HCL Blocks** - First-class stack blocks in Terragrunt
2. **Variant 2: Directory Convention** - Pure convention-based
3. **Variant 3: Tag-Based** - Metadata in locals
4. **Variant 4: External File** - YAML/JSON definitions (Recommended MVP)
5. **Variant 5: Hybrid** - Combination of all methods

## Key Features

### Stack Definition
- **Multiple sources**: External files, HCL blocks, tags, inference
- **Glob patterns**: Flexible module selection
- **Exclusions**: Fine-grained control
- **Dependencies**: Stack-level dependency graph

### Atlantis Integration
- **Stack-level projects**: One project per stack
- **Module-level projects**: Traditional per-module projects
- **Mixed mode**: Both simultaneously
- **Execution ordering**: Proper dependency sequencing
- **Custom workflows**: Per-stack workflow configuration

### Configuration Options

New CLI flags:
```bash
--enable-stacks                    # Enable stack support
--stack-definition-file string     # Path to YAML/JSON file
--stack-level-projects            # Generate stack projects
--module-level-projects           # Generate module projects (can combine)
--validate-stack-coverage         # Ensure all modules in stacks
--infer-stacks                   # Auto-detect from structure
```

## Example Use Cases

### 1. Environment Stacks
```yaml
stacks:
  - name: production
    include: ["environments/prod/**"]
    atlantis:
      apply_requirements: [approved, mergeable]
      execution_order_group: 100
```

### 2. Service Stacks
```yaml
stacks:
  - name: auth-service
    modules:
      - services/auth/api
      - services/auth/database
    depends_on: [platform]
```

### 3. Layer-Based Stacks
```yaml
stacks:
  - name: networking
    include: ["**/networking/**"]
    execution_order_group: 1
  - name: data-layer
    include: ["**/databases/**"]
    depends_on: [networking]
    execution_order_group: 2
```

## Generated Atlantis Output

### Stack-Level Project
```yaml
projects:
  - name: production-environment
    dir: environments/production
    workflow: production
    workspace: production
    autoplan:
      enabled: true
      when_modified:
        - "environments/production/**/*.hcl"
        - "environments/production/**/*.tf*"
    apply_requirements:
      - approved
      - mergeable
    execution_order_group: 100
```

### Module Projects with Stack Context
```yaml
projects:
  - name: production_vpc
    dir: environments/production/vpc
    workflow: production
    workspace: production_vpc
    autoplan:
      enabled: true
      when_modified:
        - "*.hcl"
        - "*.tf*"
    execution_order_group: 100
    # Future: could add stack metadata
```

## Migration Path

### For Existing Users

**No Impact** - Stack support is completely opt-in:
```bash
# No changes needed
terragrunt-atlantis-config generate --output atlantis.yaml
```

### Gradual Adoption

**Step 1**: Test with inference
```bash
terragrunt-atlantis-config generate \
  --enable-stacks \
  --infer-stacks \
  --output atlantis.yaml
```

**Step 2**: Create explicit definitions
```bash
# Review output, create atlantis-stacks.yaml
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --output atlantis.yaml
```

**Step 3**: Fine-tune configuration
```bash
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --stack-level-projects \
  --module-level-projects \
  --validate-stack-coverage \
  --output atlantis.yaml
```

## Technical Architecture

### Data Flow

```
1. Discovery Phase
   ├─ Load external stack definition file (if specified)
   ├─ Scan for stack HCL blocks (if enabled)
   ├─ Infer from directory structure (if enabled)
   └─ Parse module-level stack tags (if enabled)

2. Assignment Phase
   ├─ Match modules to stacks (glob patterns or explicit)
   ├─ Resolve precedence (external > HCL > tags > inference)
   └─ Build stack-to-modules mapping

3. Validation Phase
   ├─ Validate stack definitions
   ├─ Check for circular dependencies
   └─ Verify module coverage (if required)

4. Generation Phase
   ├─ Generate stack-level projects (if enabled)
   ├─ Generate module-level projects (if enabled)
   ├─ Set execution order based on dependencies
   └─ Merge into Atlantis configuration
```

### Integration Points

**Modified Files**:
- `cmd/generate.go` - Add stack generation logic
- `cmd/config.go` - Add stack-related structs
- `cmd/parse_locals.go` - Add stack tag parsing

**New Files**:
- `cmd/stack.go` - Core stack logic
- `cmd/parse_stack_file.go` - External file parsing
- `cmd/stack_test.go` - Test suite

**Test Files**:
- `test_examples/stacks_basic/` - Basic stack example
- `test_examples/stacks_with_patterns/` - Pattern matching
- `cmd/golden/stacks_basic.yaml` - Expected output

## Implementation Effort

### Timeline: 6-8 Weeks

**Week 1-2**: Core Infrastructure
- Stack data structures
- External file parsing
- Module matching logic

**Week 2-3**: CLI Integration
- Add command flags
- Integrate with main generation flow
- Error handling

**Week 3-4**: Testing
- Unit tests
- Integration tests
- Golden file tests
- Documentation

**Week 4-6**: Additional Variants
- Directory inference
- Tag support
- Precedence resolution

**Week 6-8**: Polish
- Bug fixes
- Performance optimization
- Documentation
- Examples

### Resource Requirements
- 1 Senior Developer (full-time)
- Code reviews from 1-2 team members
- Testing support

## Benefits

### For Users
✅ **Logical grouping** - Manage related modules as units  
✅ **Simplified workflows** - Single project for entire environment  
✅ **Better organization** - Clear stack boundaries  
✅ **Faster execution** - Optimized parallelization  
✅ **Flexible adoption** - Multiple implementation methods  

### For Operations
✅ **Reduced config size** - Fewer Atlantis projects  
✅ **Better visibility** - Stack-level status  
✅ **Easier debugging** - Clear module groupings  
✅ **Maintainability** - Centralized definitions  

### For Organizations
✅ **Scalability** - Handle large monorepos  
✅ **Standardization** - Consistent stack patterns  
✅ **Governance** - Stack-level policies  
✅ **Flexibility** - Multiple organizational patterns  

## Risks and Mitigations

### Risk: Breaking Changes
**Mitigation**: Completely opt-in, backward compatible

### Risk: Complexity
**Mitigation**: Start with simplest variant (external file), add features incrementally

### Risk: Performance
**Mitigation**: Optimize glob matching, cache stack assignments, parallel processing

### Risk: User Confusion
**Mitigation**: Clear documentation, examples, templates, good defaults

## Success Metrics

### Adoption
- % of users enabling stacks
- Number of stacks per repository
- Average modules per stack

### Performance
- Generation time with stacks
- Atlantis execution time
- Memory usage

### Quality
- Bug reports
- Support requests
- User satisfaction

## Next Steps

1. **Review Proposal** - Get stakeholder feedback
2. **Prioritize Variant** - Confirm Phase 1 approach
3. **Create Epic** - Break down into stories
4. **Prototype** - Build MVP of external file support
5. **Test** - Validate with real-world repos
6. **Iterate** - Refine based on feedback
7. **Document** - User guides and examples
8. **Release** - Beta → stable

## Questions?

For more details, see:
- **`STACKS_IMPLEMENTATION_PROPOSAL.md`** - Complete proposal
- **`cmd/stack.go`** - Core implementation
- **`cmd/parse_stack_file.go`** - Parsing logic
- **`test_examples/stacks_*/`** - Working examples

## Comparison with Other Tools

### vs Terragrunt run-all
- **Stacks**: Atlantis-specific, PR automation focused
- **run-all**: CLI execution, manual operation

### vs Terraform Cloud Workspaces
- **Stacks**: Logical grouping of modules
- **Workspaces**: Separate state management

### vs Atlantis Projects
- **Stacks**: Higher-level abstraction, groups projects
- **Projects**: Individual Terragrunt modules

## Conclusion

The proposed stacks implementation provides:

1. **Immediate value** through external definition files
2. **Future flexibility** with multiple sources
3. **Zero disruption** for existing users
4. **Clear migration path** for adoption
5. **Comprehensive testing** strategy

Recommended approach: **Start with Variant 4 (External Definition File)**, then incrementally add other variants based on user feedback.

**Total effort**: 6-8 weeks for full implementation  
**MVP effort**: 3-4 weeks for external file support  
**ROI**: High - enables better organization of large Terragrunt monorepos


