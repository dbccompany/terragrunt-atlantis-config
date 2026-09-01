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
	"github.com/zclconf/go-cty/cty"
)

// UnitBlock represents a unit block in terragrunt.stack.hcl
// Per Terragrunt docs, `source` is where the unit configuration comes from
// (often a remote git ref or a local catalog directory) and `path` is the
// directory (relative to the stack file) the unit gets generated into by
// `terragrunt stack generate`.
type UnitBlock struct {
	Name   string   `hcl:"name,label"`
	Source *string  `hcl:"source,attr"`
	Path   *string  `hcl:"path,attr"`
	Remain hcl.Body `hcl:",remain"`
}

// StackBlock represents a (nested) stack block in terragrunt.stack.hcl.
// In real Terragrunt stacks, `stack` blocks define nested stacks with their
// own `source` and `path`; `description` is not an official attribute but is
// tolerated for convenience.
type StackBlock struct {
	Name        string   `hcl:"name,label"`
	Source      *string  `hcl:"source,attr"`
	Path        *string  `hcl:"path,attr"`
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
	FilePath string
	Units    []UnitBlock
	// Stacks are the nested stack blocks declared inside the file.
	Stacks []StackBlock
}

// ParseStackHclFile reads and parses a terragrunt.stack.hcl file.
//
// The file is first decoded with a full Terragrunt evaluation context so that
// functions like find_in_parent_folders() work. Real-world stack files often
// reference files or functions that are not resolvable at generation time
// (e.g. remote sources, missing parent files); in that case we fall back to a
// literal decoding pass that only extracts statically-evaluable attributes,
// instead of failing the whole file.
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

	parsed := ParsedStackHcl{}
	evaluated := false

	// Try decoding with a full Terragrunt evaluation context first
	evalContext, evalCtxErr := createTerragruntEvalContext(ctx, path)
	if evalCtxErr == nil {
		decodeDiagnostics := gohcl.DecodeBody(file.Body, evalContext, &parsed)
		if decodeDiagnostics != nil && decodeDiagnostics.HasErrors() {
			log.Debugf("Failed to evaluate stack HCL file %s with Terragrunt context (%v), falling back to literal parsing", path, decodeDiagnostics)
		} else {
			evaluated = true
		}
	} else {
		log.Debugf("Failed to create eval context for stack HCL file %s (%v), falling back to literal parsing", path, evalCtxErr)
	}

	if !evaluated {
		parsed = ParsedStackHcl{}
		if err := parseStackHclLiteral(file, &parsed); err != nil {
			return nil, fmt.Errorf("failed to decode stack HCL file: %w", err)
		}
	}

	return &StackHclDefinition{
		FilePath: path,
		Units:    parsed.Units,
		Stacks:   parsed.Stacks,
	}, nil
}

// parseStackHclLiteral extracts unit/nested-stack blocks from an already parsed
// HCL file without evaluating any functions or variables. Attributes that do
// not evaluate to a static string are silently skipped (set to nil).
func parseStackHclLiteral(file *hcl.File, parsed *ParsedStackHcl) error {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "unit", LabelNames: []string{"name"}},
			{Type: "stack", LabelNames: []string{"name"}},
		},
	}

	content, _, diags := file.Body.PartialContent(schema)
	if diags != nil && diags.HasErrors() {
		return diags
	}

	literalString := func(body hcl.Body, attrName string) *string {
		attrs, diags := body.JustAttributes()
		if diags != nil && diags.HasErrors() {
			return nil
		}
		attr, ok := attrs[attrName]
		if !ok {
			return nil
		}
		val, diags := attr.Expr.Value(nil)
		if diags != nil && diags.HasErrors() {
			return nil
		}
		if val.Type() != cty.String {
			return nil
		}
		s := val.AsString()
		return &s
	}

	for _, block := range content.Blocks {
		if len(block.Labels) == 0 {
			continue
		}
		switch block.Type {
		case "unit":
			parsed.Units = append(parsed.Units, UnitBlock{
				Name:   block.Labels[0],
				Source: literalString(block.Body, "source"),
				Path:   literalString(block.Body, "path"),
			})
		case "stack":
			parsed.Stacks = append(parsed.Stacks, StackBlock{
				Name:        block.Labels[0],
				Source:      literalString(block.Body, "source"),
				Path:        literalString(block.Body, "path"),
				Description: literalString(block.Body, "description"),
			})
		}
	}

	return nil
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

// FindStackHclFiles searches for terragrunt.stack.hcl files in the given root directory.
// VCS metadata and Terragrunt-generated directories (.terragrunt-stack,
// .terragrunt-cache) are excluded from the search.
func FindStackHclFiles(rootDir string) ([]string, error) {
	var stackFiles []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			switch info.Name() {
			case ".git", ".terragrunt-stack", ".terragrunt-cache":
				return filepath.SkipDir
			}
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

// ConvertStackHclToStacks converts parsed HCL stack definitions to internal Stack structs.
//
// For every unit (and nested stack) we record two kinds of directories,
// relative to gitRoot:
//   - Modules: directories at the unit's `path` that contain a terragrunt.hcl
//     on disk. These are treated as stack members and do not get individual
//     Atlantis projects.
//   - UnitSources: directories referenced through a unit's local `source`
//     (e.g. a catalog directory such as ../../units/vpc). These are added to
//     the stack project's autoplan when_modified patterns. Remote sources
//     cannot be watched and are skipped.
func ConvertStackHclToStacks(definitions []StackHclDefinition, gitRoot string) []Stack {
	stacks := []Stack{}
	cleanGitRoot := filepath.Clean(gitRoot)

	addUnique := func(list *[]string, entry string) {
		for _, e := range *list {
			if e == entry {
				return
			}
		}
		*list = append(*list, entry)
	}

	// relDirInsideRepo converts an absolute path to a slash-separated path
	// relative to gitRoot. Returns false when the path is outside the repo.
	relDirInsideRepo := func(absPath string) (string, bool) {
		rel, err := filepath.Rel(cleanGitRoot, absPath)
		if err != nil {
			return "", false
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", false
		}
		return filepath.ToSlash(rel), true
	}

	for _, def := range definitions {
		stackDir := filepath.Dir(def.FilePath)

		relStackDir, err := filepath.Rel(cleanGitRoot, stackDir)
		if err != nil {
			relStackDir = stackDir
		}
		relStackDir = filepath.ToSlash(relStackDir)

		relStackFile, err := filepath.Rel(cleanGitRoot, def.FilePath)
		if err != nil {
			relStackFile = def.FilePath
		}

		members := []string{}
		unitSources := []string{}

		// processBlock records member and/or source directories for a unit or
		// nested stack block.
		processBlock := func(source, path *string) {
			if path != nil {
				unitDir := filepath.Join(stackDir, *path)
				if _, err := os.Stat(filepath.Join(unitDir, "terragrunt.hcl")); err == nil {
					if rel, ok := relDirInsideRepo(unitDir); ok {
						addUnique(&members, rel)
					}
				}
			}
			// A source is only watched when it resolves to an existing local
			// directory inside the repo. Remote sources (git refs, registry,
			// etc.) never resolve locally and are skipped.
			if source != nil {
				sourceDir := *source
				if !filepath.IsAbs(sourceDir) {
					sourceDir = filepath.Join(stackDir, sourceDir)
				}
				if info, err := os.Stat(sourceDir); err == nil && info.IsDir() {
					if rel, ok := relDirInsideRepo(sourceDir); ok {
						addUnique(&unitSources, rel)
					}
				}
			}
		}

		for _, unit := range def.Units {
			processBlock(unit.Source, unit.Path)
		}
		for _, nested := range def.Stacks {
			processBlock(nested.Source, nested.Path)
		}

		// Use an optional description from the first stack block that declares one
		description := ""
		for _, nested := range def.Stacks {
			if nested.Description != nil {
				description = *nested.Description
				break
			}
		}

		stacks = append(stacks, Stack{
			Name:         relStackDir,
			Description:  description,
			Modules:      members,
			UnitSources:  unitSources,
			Dependencies: []string{},
			Source:       filepath.ToSlash(relStackFile),
		})
	}

	return stacks
}
