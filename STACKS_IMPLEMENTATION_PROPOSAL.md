# Terragrunt Stacks Support - Implementation Proposal

## Executive Summary

This document proposes multiple implementation variants for adding Terragrunt "stacks" support to `terragrunt-atlantis-config`. Stacks are logical groupings of related Terragrunt modules that should be managed as cohesive units in Atlantis workflows.

## Background

### Current Architecture

The `terragrunt-atlantis-config` tool currently:
1. Discovers all `terragrunt.hcl` files recursively
2. Parses dependencies from:
   - `dependency` and `dependencies` blocks
   - `terraform.source` references
   - `extra_arguments` var files
   - `extra_atlantis_dependencies` locals
3. Builds a Directed Acyclic Graph (DAG) of dependencies
4. Generates one Atlantis project per Terragrunt module
5. Supports parallel execution with cascading dependencies

### What are Terragrunt Stacks?

Terragrunt stacks represent a higher-level abstraction for grouping related infrastructure modules. Examples:
- **Environment stacks**: All modules for a complete environment (networking, databases, apps)
- **Service stacks**: All infrastructure for a specific service across regions
- **Layer stacks**: All modules at a specific infrastructure layer (e.g., all databases)

---

## Implementation Variants

## Variant 1: Stack Definition via HCL Blocks (Explicit)

### Overview
Add first-class `stack` block support to Terragrunt configuration files.

### Implementation Details

#### 1.1 Configuration Format

**Root-level stack definition** (`stack.hcl`):
```hcl
# stack.hcl
stack {
  name = "production-environment"
  description = "Complete production environment infrastructure"
  
  # Modules that belong to this stack
  units = [
    "./networking/vpc",
    "./networking/security-groups",
    "./databases/postgresql",
    "./applications/api",
    "./applications/web",
  ]
  
  # Stack-level dependencies on other stacks
  dependencies = [
    "../shared-services",
  ]
  
  # Atlantis-specific configuration
  atlantis_config {
    workflow = "production"
    auto_plan = true
    parallel = true
    apply_requirements = ["approved", "mergeable"]
  }
}
```

**Inline stack membership** (in module's `terragrunt.hcl`):
```hcl
# terragrunt.hcl
locals {
  atlantis_stack = "production-environment"
  atlantis_stack_order = 10  # Execution order within stack
}
```

#### 1.2 Code Changes Required

**New Files:**
- `cmd/parse_stack.go` - Parse stack definitions
- `cmd/stack.go` - Stack data structures and logic

**Modified Files:**
- `cmd/config.go` - Add `AtlantisStack` struct
- `cmd/generate.go` - Add stack detection and aggregation logic
- `cmd/parse_locals.go` - Add stack membership parsing

**Data Structures:**
```go
// cmd/stack.go
type AtlantisStack struct {
    Name              string              `json:"name"`
    Description       string              `json:"description,omitempty"`
    Units             []string            `json:"units"`
    Dependencies      []string            `json:"dependencies,omitempty"`
    Workflow          string              `json:"workflow,omitempty"`
    AutoPlan          bool                `json:"auto_plan"`
    Parallel          bool                `json:"parallel"`
    ApplyRequirements []string            `json:"apply_requirements,omitempty"`
}

type StackConfig struct {
    Stacks   []AtlantisStack    `json:"stacks,omitempty"`
    Projects []AtlantisProject  `json:"projects"`
}
```

#### 1.3 Generation Logic

1. **Discovery Phase**:
   - Find all `stack.hcl` files in repository
   - Parse stack definitions
   - Build stack membership map

2. **Module Processing Phase**:
   - For each Terragrunt module, check stack membership
   - Tag projects with stack information
   - Aggregate stack-level dependencies

3. **Output Phase**:
   - Generate Atlantis projects grouped by stack
   - Create stack-level `when_modified` patterns
   - Set stack-wide configuration (workflow, requirements, etc.)

#### 1.4 CLI Flags

```bash
--enable-stacks                  # Enable stack support (default: false)
--stack-file-name string        # Name of stack definition file (default: "stack.hcl")
--stack-level-projects          # Generate one project per stack instead of per module
--stack-cascade-dependencies    # Include all module deps in stack deps (default: true)
```

#### 1.5 Example Output

**Generated `atlantis.yaml`:**
```yaml
version: 3
projects:
  # Stack-level project
  - name: production-environment
    dir: environments/production
    workflow: production
    workspace: production
    autoplan:
      enabled: true
      when_modified:
        - "environments/production/**/*.hcl"
        - "environments/production/**/*.tf*"
        - "../shared-services/**/*.hcl"
    apply_requirements:
      - approved
      - mergeable
    execution_order_group: 10
    
  # Individual module projects (if not using --stack-level-projects)
  - name: production-environment_networking_vpc
    dir: environments/production/networking/vpc
    workflow: production
    workspace: production-environment_networking_vpc
    autoplan:
      enabled: true
      when_modified:
        - "*.hcl"
        - "*.tf*"
    execution_order_group: 10
```

### Pros
✅ Explicit, clear stack definitions
✅ First-class Terragrunt feature support
✅ Full control over stack composition
✅ Easy to understand and document
✅ Supports complex stack hierarchies

### Cons
❌ Requires changes to Terragrunt core (or relies on convention)
❌ Requires users to create additional configuration files
❌ More complex initial setup
❌ Need to maintain stack definitions separately

---

## Variant 2: Directory-Based Stack Inference (Convention)

### Overview
Infer stacks automatically based on directory structure and naming conventions.

### Implementation Details

#### 2.1 Convention Rules

**Rule 1: Environment-based stacks**
```
environments/
├── production/          # Stack: "production"
│   ├── networking/
│   ├── databases/
│   └── applications/
├── staging/            # Stack: "staging"
│   ├── networking/
│   └── applications/
```

**Rule 2: Marker files**
```
environments/production/.atlantis-stack
```
Content:
```yaml
name: production-environment
workflow: production
auto_plan: true
```

**Rule 3: Parent terragrunt.hcl with stack locals**
```hcl
# environments/production/terragrunt.hcl
locals {
  atlantis_stack_name = "production"
  atlantis_stack_workflow = "production"
}
```

#### 2.2 Code Changes Required

**Modified Files:**
- `cmd/generate.go` - Add directory-based inference
- `cmd/parse_locals.go` - Add stack name extraction

**New Logic:**
```go
// cmd/stack_inference.go
func inferStackFromPath(modulePath string, gitRoot string) (*StackInference, error) {
    // Algorithm:
    // 1. Look for .atlantis-stack marker file in parent directories
    // 2. Check parent terragrunt.hcl for atlantis_stack_* locals
    // 3. Use directory name at configured depth (e.g., 2 levels from root)
    // 4. Return nil if no stack can be inferred
}

type StackInference struct {
    Name     string
    Root     string
    Depth    int
    Source   string  // "marker-file", "parent-locals", "directory-convention"
}
```

#### 2.3 CLI Flags

```bash
--infer-stacks                   # Enable automatic stack inference
--stack-marker-file string      # Marker filename (default: ".atlantis-stack")
--stack-directory-depth int     # Directory depth for stack name (default: 2)
--stack-naming-pattern string   # Regex pattern for stack directories
```

#### 2.4 Discovery Algorithm

```go
func discoverStacks(gitRoot string) (map[string]Stack, error) {
    stacks := make(map[string]Stack)
    
    // Phase 1: Find marker files
    markerFiles := findFiles(gitRoot, stackMarkerFile)
    for _, marker := range markerFiles {
        stack := parseStackMarker(marker)
        stacks[stack.Name] = stack
    }
    
    // Phase 2: Scan parent terragrunt.hcl files
    parentConfigs := findParentTerragruntFiles(gitRoot)
    for _, config := range parentConfigs {
        if hasStackLocals(config) {
            stack := inferFromLocals(config)
            stacks[stack.Name] = stack
        }
    }
    
    // Phase 3: Directory convention
    if useDirectoryConvention {
        modules := getAllTerragruntFiles(gitRoot)
        for _, module := range modules {
            stackName := extractStackFromPath(module, stackDirectoryDepth)
            if stackName != "" {
                addModuleToStack(stacks, stackName, module)
            }
        }
    }
    
    return stacks, nil
}
```

### Pros
✅ No additional configuration files required
✅ Works with existing directory structures
✅ Minimal user burden
✅ Backward compatible
✅ Quick to implement

### Cons
❌ Less explicit - harder to understand what's happening
❌ Convention conflicts possible
❌ Limited flexibility in stack composition
❌ Harder to have modules in multiple stacks
❌ Relies on consistent directory structure

---

## Variant 3: Tag-Based Stack Assignment (Metadata)

### Overview
Use metadata tags in `locals` blocks to assign modules to stacks.

### Implementation Details

#### 3.1 Configuration Format

```hcl
# terragrunt.hcl
locals {
  # Single stack membership
  atlantis_stack = "production-environment"
  
  # OR multiple stack membership
  atlantis_stacks = ["production-environment", "critical-infrastructure"]
  
  # Stack-specific settings
  atlantis_stack_config = {
    production-environment = {
      execution_order = 10
      critical = true
    }
    critical-infrastructure = {
      execution_order = 5
    }
  }
}
```

#### 3.2 Code Changes Required

**Modified Files:**
- `cmd/parse_locals.go` - Extract stack tags
- `cmd/generate.go` - Group by tags

**New Locals Structure:**
```go
type ResolvedLocals struct {
    // ... existing fields ...
    
    // Stack membership
    AtlantisStack       string              // Single stack
    AtlantisStacks      []string            // Multiple stacks
    AtlantisStackConfig map[string]StackModuleConfig
}

type StackModuleConfig struct {
    ExecutionOrder int
    Critical       bool
    CustomFields   map[string]interface{}
}
```

#### 3.3 Generation Algorithm

```go
func generateStackProjects(modules []string) ([]AtlantisProject, error) {
    // Phase 1: Build stack membership map
    stackMap := make(map[string][]string)
    for _, module := range modules {
        locals := parseLocals(module)
        
        // Handle both single and multiple stack assignment
        stacks := getModuleStacks(locals)
        for _, stackName := range stacks {
            stackMap[stackName] = append(stackMap[stackName], module)
        }
    }
    
    // Phase 2: Generate projects per stack
    projects := []AtlantisProject{}
    for stackName, stackModules := range stackMap {
        project := generateStackProject(stackName, stackModules)
        projects = append(projects, project)
        
        // Optionally generate individual module projects too
        if !stackLevelProjectsOnly {
            for _, module := range stackModules {
                moduleProject := generateModuleProject(module, stackName)
                projects = append(projects, moduleProject)
            }
        }
    }
    
    return projects, nil
}
```

#### 3.4 CLI Flags

```bash
--enable-stack-tags              # Enable tag-based stack assignment
--stack-tag-local string         # Local name for stack tags (default: "atlantis_stack")
--allow-multi-stack             # Allow modules in multiple stacks
--stack-prefix string           # Prefix for stack project names
```

### Pros
✅ Flexible - modules can be in multiple stacks
✅ Gradual adoption - tag modules incrementally
✅ Works with any directory structure
✅ Module-level control
✅ Easy to implement

### Cons
❌ Stack configuration scattered across files
❌ No single source of truth for stack composition
❌ Potential for inconsistency
❌ Harder to visualize full stack
❌ Requires updating every module

---

## Variant 4: External Stack Definition File (YAML/JSON)

### Overview
Define stacks in a separate YAML/JSON file outside Terragrunt configuration.

### Implementation Details

#### 4.1 Configuration Format

**`atlantis-stacks.yaml`:**
```yaml
version: 1
stacks:
  - name: production-environment
    description: Complete production infrastructure
    
    # Glob patterns for modules
    include:
      - "environments/production/**"
    exclude:
      - "environments/production/experimental/**"
    
    # Or explicit module list
    modules:
      - environments/production/networking/vpc
      - environments/production/databases/postgresql
      - environments/production/applications/api
    
    # Stack dependencies
    depends_on:
      - shared-services
      - platform-foundation
    
    # Atlantis configuration
    atlantis:
      workflow: production
      autoplan: true
      parallel: true
      apply_requirements:
        - approved
        - mergeable
      execution_order_group: 10
  
  - name: staging-environment
    include:
      - "environments/staging/**"
    atlantis:
      workflow: staging
      autoplan: true
      parallel: true

  - name: critical-infrastructure
    description: Cross-environment critical resources
    modules:
      - shared-services/dns
      - shared-services/secrets
      - environments/production/databases/postgresql
      - environments/production/networking/vpc
    atlantis:
      workflow: critical
      autoplan: false
      apply_requirements:
        - approved
        - mergeable
      execution_order_group: 5
```

#### 4.2 Code Changes Required

**New Files:**
- `cmd/parse_stack_file.go` - Parse external stack definitions
- `cmd/stack_matcher.go` - Match modules to stacks using glob patterns

**Data Structures:**
```go
type StackDefinitionFile struct {
    Version int                    `yaml:"version" json:"version"`
    Stacks  []ExternalStackConfig  `yaml:"stacks" json:"stacks"`
}

type ExternalStackConfig struct {
    Name        string              `yaml:"name" json:"name"`
    Description string              `yaml:"description,omitempty" json:"description,omitempty"`
    Include     []string            `yaml:"include,omitempty" json:"include,omitempty"`
    Exclude     []string            `yaml:"exclude,omitempty" json:"exclude,omitempty"`
    Modules     []string            `yaml:"modules,omitempty" json:"modules,omitempty"`
    DependsOn   []string            `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
    Atlantis    AtlantisStackConfig `yaml:"atlantis,omitempty" json:"atlantis,omitempty"`
}

type AtlantisStackConfig struct {
    Workflow            string   `yaml:"workflow,omitempty"`
    AutoPlan            bool     `yaml:"autoplan"`
    Parallel            bool     `yaml:"parallel"`
    ApplyRequirements   []string `yaml:"apply_requirements,omitempty"`
    ExecutionOrderGroup int      `yaml:"execution_order_group,omitempty"`
    Workspace           string   `yaml:"workspace,omitempty"`
}
```

#### 4.3 Module Matching Logic

```go
func matchModulesToStacks(modules []string, stacks []ExternalStackConfig) (map[string][]string, error) {
    stackModules := make(map[string][]string)
    
    for _, stack := range stacks {
        matched := []string{}
        
        if len(stack.Modules) > 0 {
            // Explicit module list
            for _, module := range stack.Modules {
                if moduleExists(module, modules) {
                    matched = append(matched, module)
                }
            }
        } else {
            // Glob pattern matching
            for _, module := range modules {
                if matchesIncludePattern(module, stack.Include) &&
                   !matchesExcludePattern(module, stack.Exclude) {
                    matched = append(matched, module)
                }
            }
        }
        
        stackModules[stack.Name] = matched
    }
    
    return stackModules, nil
}
```

#### 4.4 CLI Flags

```bash
--stack-definition-file string   # Path to stack definition file (default: "atlantis-stacks.yaml")
--stack-file-format string       # Format: yaml or json (default: "yaml")
--validate-stack-coverage        # Error if modules aren't assigned to stacks
--allow-stack-overlap           # Allow modules in multiple stacks
```

### Pros
✅ Centralized stack definitions
✅ Easy to visualize entire stack structure
✅ Powerful glob pattern matching
✅ Version controlled alongside code
✅ Supports complex stack relationships
✅ No changes to Terragrunt configs
✅ Easy to validate and test

### Cons
❌ Additional file to maintain
❌ Potential for definitions to drift from reality
❌ Need to update file when adding modules
❌ Glob patterns can be error-prone

---

## Variant 5: Hybrid Approach (Recommended)

### Overview
Combine the best aspects of multiple variants for maximum flexibility.

### Implementation Details

#### 5.1 Multi-Source Stack Definition

Support multiple ways to define stacks, with clear precedence:

1. **External definition file** (highest precedence)
2. **Explicit stack blocks** in HCL
3. **Module-level tags** in locals
4. **Inferred from directory structure** (lowest precedence)

#### 5.2 Configuration Example

**`atlantis-stacks.yaml`** (optional, global stacks):
```yaml
stacks:
  - name: production-environment
    include:
      - "environments/production/**"
    atlantis:
      workflow: production
      autoplan: true
```

**`environments/production/stack.hcl`** (optional, local stack override):
```hcl
stack {
  name = "production-environment"
  workflow = "production-custom"
  
  atlantis_config {
    execution_order_group = 10
  }
}
```

**`terragrunt.hcl`** (module-level override):
```hcl
locals {
  # Override stack membership
  atlantis_stack = "production-environment"
  
  # Override stack settings for this module
  atlantis_stack_execution_order = 15
}
```

#### 5.3 Precedence Resolution

```go
func resolveStackConfig(module string) (*StackConfig, error) {
    config := &StackConfig{}
    
    // Layer 1: Inferred defaults
    if inferStacks {
        config = mergeConfig(config, inferStackFromDirectory(module))
    }
    
    // Layer 2: Module-level tags
    locals := parseLocals(module)
    if locals.AtlantisStack != "" {
        config = mergeConfig(config, stackFromTag(locals))
    }
    
    // Layer 3: Stack HCL blocks
    stackHcl := findStackHclFile(module)
    if stackHcl != nil {
        config = mergeConfig(config, parseStackHcl(stackHcl))
    }
    
    // Layer 4: External definition file
    if stackDefFile != "" {
        extConfig := matchModuleInStackFile(module, stackDefFile)
        config = mergeConfig(config, extConfig)
    }
    
    return config, nil
}
```

#### 5.4 CLI Flags (Complete Set)

```bash
# Stack detection
--enable-stacks                        # Enable any stack support
--infer-stacks                        # Enable directory-based inference
--stack-definition-file string        # External YAML/JSON file
--stack-hcl-files strings            # Names of stack HCL files to look for

# Stack behavior
--stack-level-projects               # Generate one project per stack
--module-level-projects              # Generate projects per module (can combine with above)
--allow-multi-stack                  # Allow modules in multiple stacks
--validate-stack-coverage            # Ensure all modules are in stacks

# Stack configuration
--stack-directory-depth int          # For inference
--stack-marker-file string           # Marker file name
--stack-naming-pattern string        # Regex for stack names
--stack-prefix string                # Prefix for stack project names

# Integration
--stack-cascade-dependencies         # Include module deps in stack deps
--stack-execution-order-base int     # Base execution order for stacks
--stack-workspace-prefix string      # Prefix for stack workspaces
```

### Pros
✅ Maximum flexibility - choose what works best
✅ Gradual adoption path
✅ Supports all use cases
✅ Clear precedence rules
✅ Backward compatible

### Cons
❌ Most complex to implement
❌ Most complex for users to understand
❌ Potential for confusion with multiple options
❌ Larger codebase to maintain

---

## Comparison Matrix

| Feature | Variant 1 (HCL) | Variant 2 (Directory) | Variant 3 (Tags) | Variant 4 (External) | Variant 5 (Hybrid) |
|---------|-----------------|----------------------|------------------|----------------------|-------------------|
| **Explicitness** | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Ease of Setup** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| **Flexibility** | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Maintainability** | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Multi-stack Support** | ⭐⭐⭐ | ⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Visibility** | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Implementation Complexity** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| **Backward Compatibility** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

---

## Recommended Approach

### Phase 1: MVP (Variant 4 - External Definition File)

**Rationale:**
- Cleanest separation of concerns
- Easiest to implement
- No changes to Terragrunt configs
- Easy to test and validate
- Provides immediate value

**Timeline:** 2-3 weeks
**Effort:** Medium

### Phase 2: Enhancement (Add Variant 2 - Directory Inference)

**Rationale:**
- Provides automatic stack detection
- Works with existing structures
- Optional - users can choose

**Timeline:** 1-2 weeks
**Effort:** Low-Medium

### Phase 3: Advanced (Add Variant 3 - Tag Support)

**Rationale:**
- Allows module-level overrides
- Supports complex multi-stack scenarios
- Complements external definitions

**Timeline:** 1-2 weeks
**Effort:** Low-Medium

### Final State: Hybrid (Variant 5)

All three methods working together with clear precedence.

---

## Implementation Plan

### Stage 1: Core Infrastructure (Week 1-2)

**Files to Create:**
```
cmd/
├── stack.go              # Core stack data structures
├── parse_stack_file.go   # External file parsing
├── stack_matcher.go      # Module-to-stack matching
└── stack_generator.go    # Stack project generation
```

**Files to Modify:**
```
cmd/
├── config.go            # Add stack fields to AtlantisConfig
├── generate.go          # Integrate stack generation
└── parse_locals.go      # Add stack-related locals
```

**Key Functions:**
```go
// stack.go
type Stack struct {
    Name              string
    Description       string
    Modules           []string
    Dependencies      []string
    AtlantisConfig    AtlantisStackConfig
    ExecutionOrder    int
}

func NewStackManager(config StackManagerConfig) *StackManager
func (sm *StackManager) DiscoverStacks() ([]Stack, error)
func (sm *StackManager) AssignModulesToStacks(modules []string) (map[string][]string, error)
func (sm *StackManager) GenerateStackProjects(stacks []Stack) ([]AtlantisProject, error)

// parse_stack_file.go
func ParseStackDefinitionFile(path string) (*StackDefinitionFile, error)
func ValidateStackDefinition(def *StackDefinitionFile) error

// stack_matcher.go
func MatchModule(modulePath string, include []string, exclude []string) bool
func ResolveStackDependencies(stacks []Stack, projectMap map[string][]string) error
```

### Stage 2: CLI Integration (Week 2)

**Add CLI flags:**
```go
// cmd/generate.go init()
generateCmd.PersistentFlags().BoolVar(&enableStacks, "enable-stacks", false, "Enable stack support")
generateCmd.PersistentFlags().StringVar(&stackDefinitionFile, "stack-definition-file", "atlantis-stacks.yaml", "Stack definition file")
generateCmd.PersistentFlags().BoolVar(&stackLevelProjects, "stack-level-projects", false, "Generate projects at stack level")
generateCmd.PersistentFlags().BoolVar(&moduleLevelProjects, "module-level-projects", true, "Generate projects at module level")
generateCmd.PersistentFlags().BoolVar(&validateStackCoverage, "validate-stack-coverage", false, "Ensure all modules are in stacks")
```

### Stage 3: Generation Logic (Week 3)

**Modify main generation flow:**
```go
func main(cmd *cobra.Command, args []string) error {
    // ... existing setup ...
    
    if enableStacks {
        // Initialize stack manager
        stackMgr := NewStackManager(StackManagerConfig{
            GitRoot:           gitRoot,
            DefinitionFile:    stackDefinitionFile,
            InferFromDir:      inferStacks,
            AllowMultiStack:   allowMultiStack,
        })
        
        // Discover stacks
        stacks, err := stackMgr.DiscoverStacks()
        if err != nil {
            return err
        }
        
        // Assign modules to stacks
        stackAssignments, err := stackMgr.AssignModulesToStacks(terragruntFiles)
        if err != nil {
            return err
        }
        
        // Validate coverage if requested
        if validateStackCoverage {
            if err := validateAllModulesInStacks(terragruntFiles, stackAssignments); err != nil {
                return err
            }
        }
        
        // Generate stack projects
        if stackLevelProjects {
            stackProjects, err := stackMgr.GenerateStackProjects(stacks)
            if err != nil {
                return err
            }
            config.Projects = append(config.Projects, stackProjects...)
        }
        
        // Generate module projects with stack context
        if moduleLevelProjects {
            for _, terragruntPath := range terragruntFiles {
                project, err := createProjectWithStack(ctx, terragruntPath, stackAssignments)
                if err != nil {
                    return err
                }
                config.Projects = append(config.Projects, *project)
            }
        }
    } else {
        // ... existing logic ...
    }
    
    // ... rest of function ...
}
```

### Stage 4: Testing (Week 4)

**Test Examples:**
```
test_examples/
├── stacks_basic/
│   ├── atlantis-stacks.yaml
│   └── modules/
├── stacks_multi_environment/
│   ├── atlantis-stacks.yaml
│   ├── production/
│   └── staging/
├── stacks_overlapping/
│   ├── atlantis-stacks.yaml
│   └── shared/
└── stacks_dependencies/
    ├── atlantis-stacks.yaml
    ├── foundation/
    └── applications/
```

**Golden Files:**
```
cmd/golden/
├── stacks_basic.yaml
├── stacks_multi_environment.yaml
├── stacks_with_modules.yaml
└── stacks_complex_dependencies.yaml
```

**Unit Tests:**
```go
// cmd/stack_test.go
func TestParseStackDefinitionFile(t *testing.T)
func TestMatchModuleToStack(t *testing.T)
func TestStackDependencyResolution(t *testing.T)
func TestStackProjectGeneration(t *testing.T)
func TestMultiStackAssignment(t *testing.T)

// cmd/generate_stack_test.go
func TestGenerateWithStacks(t *testing.T)
func TestStackLevelProjects(t *testing.T)
func TestModuleLevelProjectsWithStacks(t *testing.T)
```

---

## Migration Guide

### For Existing Users

**Option 1: No Changes (Default)**
- Stack support is opt-in via `--enable-stacks`
- Existing configurations work without modification
- No breaking changes

**Option 2: Gradual Adoption**
```bash
# Step 1: Start with inference
terragrunt-atlantis-config generate --enable-stacks --infer-stacks --output atlantis.yaml

# Step 2: Review inferred stacks, create definition file
# Create atlantis-stacks.yaml based on inference results

# Step 3: Use explicit definitions
terragrunt-atlantis-config generate --enable-stacks --stack-definition-file atlantis-stacks.yaml --output atlantis.yaml
```

**Option 3: Full Stack Mode**
```bash
# Generate only stack-level projects
terragrunt-atlantis-config generate \
  --enable-stacks \
  --stack-definition-file atlantis-stacks.yaml \
  --stack-level-projects \
  --no-module-level-projects \
  --output atlantis.yaml
```

---

## Documentation Requirements

### User Documentation

1. **README.md Updates**
   - Add "Stack Support" section
   - Configuration examples
   - CLI flag documentation

2. **STACKS.md** (New)
   - Detailed stack concepts
   - Use cases and patterns
   - Complete examples
   - Troubleshooting

3. **MIGRATION.md** (New)
   - Migration from module-only to stacks
   - Common patterns
   - Best practices

### Developer Documentation

1. **ARCHITECTURE.md Updates**
   - Stack subsystem design
   - Data flow diagrams
   - Extension points

2. **CONTRIBUTING.md Updates**
   - Testing stack features
   - Adding new stack sources

---

## Open Questions

1. **Should stacks support nested stacks?**
   - Example: environment stack contains service stacks
   - Increases complexity significantly
   - Could be phase 2

2. **How to handle module in multiple stacks with different configs?**
   - Use last-match-wins?
   - Error out?
   - Generate multiple projects with different names?

3. **Should we support stack-level terraform state?**
   - Terragrunt doesn't have this concept natively
   - Would require significant architectural changes

4. **Integration with Terragrunt run-all?**
   - Should stacks map to run-all groups?
   - Need to understand Terragrunt's plans here

5. **Stack versioning and lifecycle?**
   - Should stacks have versions?
   - How to handle stack evolution over time?

---

## Success Metrics

1. **Adoption Metrics**
   - % of users enabling stack support
   - Number of stacks per repository
   - Average modules per stack

2. **Performance Metrics**
   - Generation time with stacks vs without
   - Atlantis plan/apply time improvements
   - Memory usage with large stacks

3. **Quality Metrics**
   - Bug reports related to stacks
   - Support requests
   - Documentation clarity (survey)

---

## Appendix A: Example Use Cases

### Use Case 1: Environment Stacks

**Scenario:** Organization has dev, staging, production environments

```yaml
# atlantis-stacks.yaml
stacks:
  - name: development
    include: ["environments/dev/**"]
    atlantis:
      workflow: development
      autoplan: true
      parallel: true
  
  - name: staging
    include: ["environments/staging/**"]
    depends_on: [development]
    atlantis:
      workflow: staging
      autoplan: true
      apply_requirements: [approved]
  
  - name: production
    include: ["environments/prod/**"]
    depends_on: [staging]
    atlantis:
      workflow: production
      autoplan: false
      apply_requirements: [approved, mergeable]
      execution_order_group: 100
```

### Use Case 2: Service-Oriented Stacks

**Scenario:** Microservices architecture with shared infrastructure

```yaml
stacks:
  - name: platform
    description: Shared platform services
    modules:
      - shared/vpc
      - shared/dns
      - shared/monitoring
    atlantis:
      workflow: platform
      execution_order_group: 1
  
  - name: auth-service
    include: ["services/auth/**"]
    depends_on: [platform]
    atlantis:
      workflow: service
      execution_order_group: 10
  
  - name: api-gateway
    include: ["services/api-gateway/**"]
    depends_on: [platform, auth-service]
    atlantis:
      workflow: service
      execution_order_group: 20
```

### Use Case 3: Layer-Based Stacks

**Scenario:** Traditional 3-tier architecture

```yaml
stacks:
  - name: networking
    include: ["**/networking/**"]
    atlantis:
      execution_order_group: 1
  
  - name: data-layer
    include: ["**/databases/**", "**/storage/**"]
    depends_on: [networking]
    atlantis:
      execution_order_group: 2
  
  - name: application-layer
    include: ["**/applications/**", "**/compute/**"]
    depends_on: [networking, data-layer]
    atlantis:
      execution_order_group: 3
  
  - name: presentation-layer
    include: ["**/frontend/**", "**/cdn/**"]
    depends_on: [application-layer]
    atlantis:
      execution_order_group: 4
```

---

## Appendix B: Generated atlantis.yaml Examples

### Stack-Level Projects Only

```yaml
version: 3
projects:
  - name: production
    dir: environments/production
    workflow: production
    workspace: production
    autoplan:
      enabled: true
      when_modified:
        - "environments/production/**/*.hcl"
        - "environments/production/**/*.tf*"
        - "../shared/**/*.hcl"  # from dependencies
    apply_requirements:
      - approved
      - mergeable
    execution_order_group: 100
  
  - name: staging
    dir: environments/staging
    workflow: staging
    workspace: staging
    autoplan:
      enabled: true
      when_modified:
        - "environments/staging/**/*.hcl"
        - "environments/staging/**/*.tf*"
        - "../shared/**/*.hcl"
    apply_requirements:
      - approved
    execution_order_group: 50
    depends_on:
      - production
```

### Mixed Stack and Module Projects

```yaml
version: 3
projects:
  # Stack-level aggregated project
  - name: production-stack
    dir: environments/production
    workflow: production
    workspace: production-stack
    autoplan:
      enabled: false  # Manual control at stack level
      when_modified:
        - "environments/production/**/*.hcl"
        - "environments/production/**/*.tf*"
    execution_order_group: 100
  
  # Individual module projects within stack
  - name: production_networking_vpc
    dir: environments/production/networking/vpc
    workflow: production
    workspace: production_networking_vpc
    autoplan:
      enabled: true
      when_modified:
        - "*.hcl"
        - "*.tf*"
    execution_order_group: 100
    # Could add: stack: production (custom field for reference)
  
  - name: production_databases_postgresql
    dir: environments/production/databases/postgresql
    workflow: production
    workspace: production_databases_postgresql
    autoplan:
      enabled: true
      when_modified:
        - "*.hcl"
        - "*.tf*"
        - "../../networking/vpc/**/*.hcl"  # dependency
    execution_order_group: 101
    depends_on:
      - production_networking_vpc
```

---

## Conclusion

The recommended implementation path is:

1. **Start with Variant 4 (External Definition File)** for MVP
   - Clean, explicit, easy to implement
   - Immediate value for users
   
2. **Add Variant 2 (Directory Inference)** for convenience
   - Optional automatic discovery
   - Reduces configuration burden
   
3. **Add Variant 3 (Tag Support)** for flexibility
   - Module-level control
   - Complex multi-stack scenarios

This provides a clear migration path while maintaining backward compatibility and offering flexibility for different use cases.

Total estimated effort: **6-8 weeks** for full implementation including:
- Core stack support (3 weeks)
- All three variants (2 weeks)
- Testing and documentation (2 weeks)
- Buffer for issues and refinement (1 week)


