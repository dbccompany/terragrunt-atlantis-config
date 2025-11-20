package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	log "github.com/sirupsen/logrus"
)

// Stack represents a logical grouping of Terragrunt modules
type Stack struct {
	// Unique name for the stack
	Name string

	// Optional description
	Description string

	// List of module paths belonging to this stack
	Modules []string

	// Stack dependencies (other stack names)
	Dependencies []string

	// Atlantis configuration for this stack
	AtlantisConfig StackAtlantisConfig

	// Execution order for this stack
	ExecutionOrder int

	// Source of stack definition (for debugging)
	Source string
}

// StackAtlantisConfig contains Atlantis-specific configuration for a stack
type StackAtlantisConfig struct {
	Workflow          string
	AutoPlan          bool
	Parallel          bool
	ApplyRequirements []string
	Workspace         string
	TerraformVersion  string
}

// StackDefinitionFile represents the external YAML/JSON stack definition file
type StackDefinitionFile struct {
	Version int                   `yaml:"version" json:"version"`
	Stacks  []ExternalStackConfig `yaml:"stacks" json:"stacks"`
}

// ExternalStackConfig represents a stack defined in external file
type ExternalStackConfig struct {
	Name        string              `yaml:"name" json:"name"`
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	Include     []string            `yaml:"include,omitempty" json:"include,omitempty"`
	Exclude     []string            `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Modules     []string            `yaml:"modules,omitempty" json:"modules,omitempty"`
	DependsOn   []string            `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Atlantis    AtlantisStackConfig `yaml:"atlantis,omitempty" json:"atlantis,omitempty"`
}

// AtlantisStackConfig represents Atlantis configuration in external file
type AtlantisStackConfig struct {
	Workflow            string   `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	AutoPlan            bool     `yaml:"autoplan" json:"autoplan"`
	Parallel            bool     `yaml:"parallel" json:"parallel"`
	ApplyRequirements   []string `yaml:"apply_requirements,omitempty" json:"apply_requirements,omitempty"`
	ExecutionOrderGroup int      `yaml:"execution_order_group,omitempty" json:"execution_order_group,omitempty"`
	Workspace           string   `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	TerraformVersion    string   `yaml:"terraform_version,omitempty" json:"terraform_version,omitempty"`
}

// StackManagerConfig configures the stack manager
type StackManagerConfig struct {
	GitRoot          string
	DefinitionFile   string
	InferFromDir     bool
	DirectoryDepth   int
	AllowMultiStack  bool
	StackMarkerFile  string
	ValidateCoverage bool
	StackWorkflow    string // Default workflow for stack projects
	DefaultWorkflow   string // Fallback workflow if stack workflow not set
	CreateProjectName bool  // Whether to include project name in generated config
}

// StackManager manages stack discovery and project generation
type StackManager struct {
	config         StackManagerConfig
	stacks         []Stack
	moduleToStacks map[string][]string
	stackToModules map[string][]string
}

// NewStackManager creates a new stack manager
func NewStackManager(config StackManagerConfig) *StackManager {
	return &StackManager{
		config:         config,
		stacks:         []Stack{},
		moduleToStacks: make(map[string][]string),
		stackToModules: make(map[string][]string),
	}
}

// DiscoverStacks discovers all stacks from configured sources
// Priority order:
// 1. HCL stack files (terragrunt.stack.hcl) - Primary method per Terragrunt official docs
// 2. External definition file (YAML/JSON) - Legacy/alternative method
// 3. Directory inference - Future implementation
func (sm *StackManager) DiscoverStacks() ([]Stack, error) {
	var discoveredStacks []Stack

	// Source 1: HCL stack files (terragrunt.stack.hcl) - PRIMARY METHOD
	stacks, err := sm.loadStackHclFiles()
	if err != nil {
		return nil, err
	}
	discoveredStacks = append(discoveredStacks, stacks...)

	// Source 2: External definition file (YAML/JSON) - LEGACY/ALTERNATIVE
	if sm.config.DefinitionFile != "" {
		stacks, err := sm.loadStackDefinitionFile()
		if err != nil {
			return nil, err
		}
		discoveredStacks = append(discoveredStacks, stacks...)
	}

	// Source 3: Directory inference
	if sm.config.InferFromDir {
		stacks, err := sm.inferStacksFromDirectory()
		if err != nil {
			return nil, err
		}
		discoveredStacks = append(discoveredStacks, stacks...)
	}

	// TODO: Source 4: Module-level tags

	sm.stacks = discoveredStacks
	return discoveredStacks, nil
}

// AssignModulesToStacks assigns terragrunt modules to stacks
func (sm *StackManager) AssignModulesToStacks(modules []string) (map[string][]string, error) {
	assignments := make(map[string][]string)

	for _, stack := range sm.stacks {
		for _, module := range modules {
			if sm.moduleMatchesStack(module, stack) {
				assignments[stack.Name] = append(assignments[stack.Name], module)
				sm.moduleToStacks[module] = append(sm.moduleToStacks[module], stack.Name)
			}
		}
	}

	sm.stackToModules = assignments
	return assignments, nil
}

// GenerateStackProject generates an Atlantis project for a stack
// Note: This uses stack.Modules which are the units defined in the stack file,
// NOT the modules assigned via AssignModulesToStacks (which are in stackToModules)
func (sm *StackManager) GenerateStackProject(stack Stack) (*AtlantisProject, error) {
	// Aggregate all dependencies from modules in the stack
	allDependencies := []string{
		"*.hcl",
		"*.tf*",
		"**/*.hcl",
		"**/*.tf*",
	}

	// Add stack-level dependencies
	for _, depStack := range stack.Dependencies {
		if modules, ok := sm.stackToModules[depStack]; ok {
			for _, module := range modules {
				// Convert to relative path from stack root
				allDependencies = append(allDependencies, module)
			}
		}
	}

	// Determine the directory for the stack project
	// Priority: Use stack source directory (where terragrunt.stack.hcl is located) if available
	// Fallback: Use common parent of stack.Modules (units defined in stack file) if modules exist
	var stackDir string
	
	// Always prefer Source directory if available (most accurate)
	if stack.Source != "" && stack.Source != "external" {
		// Extract directory from source file path (Source is already relative to gitRoot)
		// Handle both forward slashes (already normalized) and OS-specific separators
		normalizedSource := filepath.FromSlash(stack.Source) // Convert to OS-specific
		stackDir = filepath.Dir(normalizedSource)            // Get directory
		stackDir = filepath.ToSlash(stackDir)                // Convert back to forward slashes
		if stackDir == "." || stackDir == "" {
			// If Source directory is ".", try using stack.Modules instead
			if len(stack.Modules) > 0 {
				stackDir = sm.findCommonParent(stack.Modules)
				log.Infof("Stack %s: Source is '.', using common parent %s (from %d modules)", stack.Name, stackDir, len(stack.Modules))
			} else {
				stackDir = "."
				log.Infof("Stack %s: Source is '.' and no modules, using '.'", stack.Name)
			}
		} else {
			log.Infof("Stack %s: Using source directory %s (from Source: %s, Modules count: %d)", stack.Name, stackDir, stack.Source, len(stack.Modules))
		}
	} else if len(stack.Modules) > 0 {
		// No Source, but we have modules - use common parent
		stackDir = sm.findCommonParent(stack.Modules)
		log.Infof("Stack %s: No Source, using common parent %s (from %d modules)", stack.Name, stackDir, len(stack.Modules))
	} else {
		// No Source and no modules - use current directory
		stackDir = "."
		log.Infof("Stack %s: No Source and no modules, using '.'", stack.Name)
	}

	// Determine workflow: stack config > stack workflow flag > default workflow flag
	workflow := stack.AtlantisConfig.Workflow
	if workflow == "" && sm.config.StackWorkflow != "" {
		workflow = sm.config.StackWorkflow
	} else if workflow == "" && sm.config.DefaultWorkflow != "" {
		workflow = sm.config.DefaultWorkflow
	}

	project := &AtlantisProject{
		Dir:              stackDir,
		Workflow:         workflow,
		Workspace:        stack.AtlantisConfig.Workspace,
		TerraformVersion: stack.AtlantisConfig.TerraformVersion,
		Autoplan: AutoplanConfig{
			Enabled:      stack.AtlantisConfig.AutoPlan,
			WhenModified: uniqueStrings(allDependencies),
		},
	}

	// Only set Name if createProjectName flag is enabled (consistent with regular projects)
	if sm.config.CreateProjectName {
		project.Name = stack.Name
	}

	if len(stack.AtlantisConfig.ApplyRequirements) > 0 {
		project.ApplyRequirements = &stack.AtlantisConfig.ApplyRequirements
	}

	if stack.ExecutionOrder > 0 {
		project.ExecutionOrderGroup = &stack.ExecutionOrder
	}

	// Generate depends_on if there are stack dependencies
	if len(stack.Dependencies) > 0 {
		project.DependsOn = stack.Dependencies
	}

	return project, nil
}

// Helper methods

// loadStackHclFiles discovers and parses terragrunt.stack.hcl files
func (sm *StackManager) loadStackHclFiles() ([]Stack, error) {
	// Find all terragrunt.stack.hcl files
	stackFiles, err := FindStackHclFiles(sm.config.GitRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find stack HCL files: %w", err)
	}

	if len(stackFiles) == 0 {
		// No stack files found, that's okay
		return []Stack{}, nil
	}

	// Create parsing context - use empty options for now
	// TODO: Pass actual terragrunt options if available
	terragruntOptions := options.NewTerragruntOptions()
	ctx := config.NewParsingContext(context.Background(), terragruntOptions)

	// Parse each stack file
	stackDefinitions := []StackHclDefinition{}
	for _, stackFile := range stackFiles {
		def, err := ParseStackHclFile(stackFile, ctx)
		if err != nil {
			log.Warnf("Failed to parse stack HCL file %s: %v", stackFile, err)
			continue
		}
		stackDefinitions = append(stackDefinitions, *def)
	}

	// Convert to internal Stack structs
	stacks := ConvertStackHclToStacks(stackDefinitions, sm.config.GitRoot)
	log.Infof("Discovered %d stack(s) from %d terragrunt.stack.hcl file(s)", len(stacks), len(stackFiles))

	return stacks, nil
}

func (sm *StackManager) loadStackDefinitionFile() ([]Stack, error) {
	// Parse the stack definition file (YAML/JSON - legacy method)
	stackDef, err := ParseStackDefinitionFile(sm.config.DefinitionFile)
	if err != nil {
		return nil, err
	}

	// Convert external stack configs to internal Stack structs
	stacks := ConvertExternalStacksToStacks(stackDef.Stacks, sm.config.GitRoot)
	return stacks, nil
}

func (sm *StackManager) inferStacksFromDirectory() ([]Stack, error) {
	// TODO: Implement directory-based inference
	return []Stack{}, nil
}

func (sm *StackManager) moduleMatchesStack(module string, stack Stack) bool {
	// Convert stack to ExternalStackConfig format for matching
	extStack := ExternalStackConfig{
		Name:    stack.Name,
		Include: []string{}, // Will be populated if needed
		Exclude: []string{},
		Modules: stack.Modules,
	}

	// Use the MatchModuleToStacks function to check if module matches
	// We need to normalize the module path relative to gitRoot
	relModule, err := filepath.Rel(sm.config.GitRoot, module)
	if err != nil {
		// If relative path calculation fails, use absolute path
		relModule = module
	}
	relModule = filepath.ToSlash(relModule)

	// Check explicit module list
	for _, explicitModule := range extStack.Modules {
		explicitModule = filepath.ToSlash(explicitModule)
		if strings.HasSuffix(relModule, explicitModule) || relModule == explicitModule {
			return true
		}
	}

	return false
}

func (sm *StackManager) findCommonParent(modules []string) string {
	if len(modules) == 0 {
		return "."
	}

	// Convert all module paths to absolute directory paths relative to gitRoot
	// Modules can be either file paths (e.g., path/to/terragrunt.hcl) or directory paths
	absDirPaths := []string{}
	for _, module := range modules {
		var moduleDir string
		// Check if module is a file path (ends with terragrunt.hcl) or directory path
		if strings.HasSuffix(module, "terragrunt.hcl") || strings.HasSuffix(module, "terragrunt.hcl.json") {
			// It's a file path, get its directory
			moduleDir = filepath.Dir(module)
		} else {
			// It's already a directory path
			moduleDir = module
		}
		
		// Convert to absolute path and ensure it's a directory
		absPath := filepath.Join(sm.config.GitRoot, moduleDir)
		absDirPaths = append(absDirPaths, absPath)
	}

	// Find the common prefix
	if len(absDirPaths) == 1 {
		// Single module - return its directory relative to gitRoot
		moduleDir := absDirPaths[0]
		relPath, err := filepath.Rel(sm.config.GitRoot, moduleDir)
		if err == nil && relPath != "." {
			return filepath.ToSlash(relPath)
		}
		// Fallback: use the module path as-is (already relative)
		if strings.HasSuffix(modules[0], "terragrunt.hcl") || strings.HasSuffix(modules[0], "terragrunt.hcl.json") {
			return filepath.ToSlash(filepath.Dir(modules[0]))
		}
		return filepath.ToSlash(modules[0])
	}

	// Find common directory prefix
	commonPrefix := absDirPaths[0]
	for i := 1; i < len(absDirPaths); i++ {
		commonPrefix = findCommonPath(commonPrefix, absDirPaths[i])
		if commonPrefix == "" {
			break
		}
	}

	// Convert back to relative path from gitRoot
	if commonPrefix != "" {
		relPath, err := filepath.Rel(sm.config.GitRoot, commonPrefix)
		if err == nil && relPath != "." {
			return filepath.ToSlash(relPath)
		}
	}

	// Fallback: use directory of first module relative to gitRoot
	var moduleDir string
	if strings.HasSuffix(modules[0], "terragrunt.hcl") || strings.HasSuffix(modules[0], "terragrunt.hcl.json") {
		moduleDir = filepath.Dir(modules[0])
	} else {
		moduleDir = modules[0]
	}
	relPath, err := filepath.Rel(sm.config.GitRoot, filepath.Join(sm.config.GitRoot, moduleDir))
	if err == nil && relPath != "." {
		return filepath.ToSlash(relPath)
	}
	return filepath.ToSlash(moduleDir)
}

// findCommonPath finds the common directory path between two paths
func findCommonPath(path1, path2 string) string {
	dir1 := filepath.Dir(path1)
	dir2 := filepath.Dir(path2)

	// Walk up from the shorter path
	parts1 := strings.Split(filepath.ToSlash(dir1), "/")
	parts2 := strings.Split(filepath.ToSlash(dir2), "/")

	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	commonParts := []string{}
	for i := 0; i < minLen; i++ {
		if parts1[i] == parts2[i] {
			commonParts = append(commonParts, parts1[i])
		} else {
			break
		}
	}

	if len(commonParts) == 0 {
		return ""
	}

	return strings.Join(commonParts, string(filepath.Separator))
}

// GetStackForModule returns the stack(s) a module belongs to
func (sm *StackManager) GetStackForModule(module string) []string {
	return sm.moduleToStacks[module]
}

// ValidateStackCoverage ensures all modules are assigned to at least one stack
func (sm *StackManager) ValidateStackCoverage(allModules []string) error {
	// TODO: Implement validation
	return nil
}
