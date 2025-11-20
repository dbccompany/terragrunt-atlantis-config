package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/config/hclparse"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	log "github.com/sirupsen/logrus"
)

// UnitBlock represents a unit block in terragrunt.stack.hcl
type UnitBlock struct {
	Name   string   `hcl:"name,label"`
	Source *string  `hcl:"source,attr"`
	Path   *string  `hcl:"path,attr"`
	Remain hcl.Body `hcl:",remain"`
}

// StackBlock represents a stack block in terragrunt.stack.hcl
type StackBlock struct {
	Name        string   `hcl:"name,label"`
	Description *string  `hcl:"description,attr"`
	Remain      hcl.Body `hcl:",remain"`
}

// ParsedStackHcl represents the parsed contents of a terragrunt.stack.hcl file
type ParsedStackHcl struct {
	Units  []UnitBlock  `hcl:"unit,block"`
	Stacks []StackBlock `hcl:"stack,block"`
	Remain hcl.Body     `hcl:",remain"`
}

// StackHclDefinition represents a complete stack definition from HCL
type StackHclDefinition struct {
	FilePath   string
	Units      []UnitBlock
	StackBlock *StackBlock
}

// ParseStackHclFile reads and parses a terragrunt.stack.hcl file
func ParseStackHclFile(path string, ctx *config.ParsingContext) (*StackHclDefinition, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("stack HCL file not found: %s", path)
	}

	// Read file contents
	configString, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack HCL file: %w", err)
	}

	// Parse HCL
	parser := hclparse.NewParser()
	file, err := parseHclForStack(parser, string(configString), path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stack HCL file: %w", err)
	}

	// Create evaluation context using the same method as parse_hcl.go
	// Note: For simple unit/stack block parsing, we may not need full eval context
	// but we use it to support Terragrunt functions like find_in_parent_folders()
	evalContext, err := createTerragruntEvalContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create eval context: %w", err)
	}

	// Decode HCL into struct
	var parsed ParsedStackHcl
	decodeDiagnostics := gohcl.DecodeBody(file.Body, evalContext, &parsed)
	if decodeDiagnostics != nil && decodeDiagnostics.HasErrors() {
		return nil, fmt.Errorf("failed to decode stack HCL file: %w", decodeDiagnostics)
	}

	// Extract stack block (if present) - for now we use the first stack block found
	var stackBlock *StackBlock
	if len(parsed.Stacks) > 0 {
		stackBlock = &parsed.Stacks[0]
		if len(parsed.Stacks) > 1 {
			log.Warnf("Multiple stack blocks found in %s, using first one: %s", path, stackBlock.Name)
		}
	}

	return &StackHclDefinition{
		FilePath:   path,
		Units:      parsed.Units,
		StackBlock: stackBlock,
	}, nil
}

// parseHclForStack is a wrapper around HCL parsing for stack files
func parseHclForStack(parser *hclparse.Parser, hcl string, filename string) (*hcl.File, error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err := errors.WithStackTrace(hclparse.PanicWhileParsingConfigError{RecoveredValue: recovered, ConfigFile: filename})
			log.Errorf("Panic while parsing stack HCL: %v", err)
		}
	}()

	if filepath.Ext(filename) == ".json" {
		file, parseDiagnostics := parser.ParseJSON([]byte(hcl), filename)
		if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
			return nil, parseDiagnostics
		}
		return file, nil
	}

	file, parseDiagnostics := parser.ParseHCL([]byte(hcl), filename)
	if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics
	}

	return file, nil
}

// FindStackHclFiles searches for terragrunt.stack.hcl files in the given root directory
func FindStackHclFiles(rootDir string) ([]string, error) {
	var stackFiles []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check for terragrunt.stack.hcl files
		if info.Name() == "terragrunt.stack.hcl" {
			stackFiles = append(stackFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search for stack HCL files: %w", err)
	}

	return stackFiles, nil
}

// ConvertStackHclToStacks converts parsed HCL stack definitions to internal Stack structs
func ConvertStackHclToStacks(definitions []StackHclDefinition, gitRoot string) []Stack {
	stacks := []Stack{}

	for _, def := range definitions {
		// Determine stack name - prioritize stack block name, then directory name
		var stackName string
		if def.StackBlock != nil && def.StackBlock.Name != "" {
			stackName = def.StackBlock.Name
		} else {
			// If no stack block or empty name, use directory name where stack file is located
			stackDir := filepath.Dir(def.FilePath)
			stackName = filepath.Base(stackDir)
			// If directory name is empty or ".", use parent directory
			if stackName == "" || stackName == "." {
				stackName = filepath.Base(filepath.Dir(stackDir))
			}
		}

		// Collect unit paths - map units to actual terragrunt.hcl file locations
		unitPaths := []string{}
		stackDir := filepath.Dir(def.FilePath)

		for _, unit := range def.Units {
			var unitPath string

			if unit.Path != nil {
				// Path is specified - resolve relative to stack file directory
				unitPath = filepath.Join(stackDir, *unit.Path)

				// Check if there's a terragrunt.hcl file at this path
				terragruntFile := filepath.Join(unitPath, "terragrunt.hcl")
				if _, err := os.Stat(terragruntFile); os.IsNotExist(err) {
					// Try with units/ subdirectory (common pattern)
					unitPathWithUnits := filepath.Join(stackDir, "units", *unit.Path)
					terragruntFileWithUnits := filepath.Join(unitPathWithUnits, "terragrunt.hcl")
					if _, err := os.Stat(terragruntFileWithUnits); err == nil {
						unitPath = unitPathWithUnits
						terragruntFile = terragruntFileWithUnits
					} else {
						// Try using source if available to find the correct path
						if unit.Source != nil {
							// Source might contain the actual path - try to extract it
							// Source format is often like "${get_terragrunt_dir()}/units/main"
							sourceStr := *unit.Source
							// Try to extract path from source (simple heuristic)
							if strings.Contains(sourceStr, "/units/") {
								parts := strings.Split(sourceStr, "/units/")
								if len(parts) > 1 {
									// Reconstruct path using units/ prefix
									unitPath = filepath.Join(stackDir, "units", strings.TrimSuffix(parts[1], "\""))
									terragruntFile = filepath.Join(unitPath, "terragrunt.hcl")
								}
							}
						}

						// Check again after trying source-based path
						if _, err := os.Stat(terragruntFile); os.IsNotExist(err) {
							// Maybe the path itself is the terragrunt.hcl file
							if filepath.Base(unitPath) == "terragrunt.hcl" {
								unitPath = filepath.Dir(unitPath)
							} else {
								log.Warnf("No terragrunt.hcl found at %s for unit %s", terragruntFile, unit.Name)
							}
						}
					}
				}
				// Found terragrunt.hcl or path is valid, use the directory path
			} else if unit.Source != nil {
				// If path is not specified but source is, we need to resolve the source
				// For now, use source as-is (this may need Terragrunt function evaluation)
				unitPath = *unit.Source
				log.Warnf("Unit %s has source but no path, using source directly: %s", unit.Name, unitPath)
			} else {
				log.Warnf("Unit %s has no path or source, skipping", unit.Name)
				continue
			}

			// Convert to relative path from git root
			relPath, err := filepath.Rel(gitRoot, unitPath)
			if err != nil {
				relPath = unitPath
			}
			unitPaths = append(unitPaths, filepath.ToSlash(relPath))
		}

		description := ""
		if def.StackBlock != nil && def.StackBlock.Description != nil {
			description = *def.StackBlock.Description
		}

		// Convert stack file path to relative path from git root for Source field
		stackSourceRelPath, err := filepath.Rel(gitRoot, def.FilePath)
		if err != nil {
			stackSourceRelPath = def.FilePath
		}
		
		stack := Stack{
			Name:           stackName,
			Description:    description,
			Modules:        unitPaths,
			Dependencies:   []string{}, // TODO: Parse dependencies from stack blocks
			Source:         filepath.ToSlash(stackSourceRelPath),
			AtlantisConfig: StackAtlantisConfig{
				// Default values - could be extended to parse from locals or stack blocks
			},
		}

		stacks = append(stacks, stack)
	}

	return stacks
}
