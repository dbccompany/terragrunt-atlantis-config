package cmd

import (
	"path/filepath"
	"strings"
)

// filterWhenModifiedExcludes drops autoplan patterns whose repo-relative form
// matches any exclude glob (see atlantis_when_modified_exclude locals).
func filterWhenModifiedExcludes(
	projectDir string,
	patterns []string,
	excludeGlobs []string,
) []string {
	if len(excludeGlobs) == 0 || len(patterns) == 0 {
		return patterns
	}

	projectDir = filepath.ToSlash(filepath.Clean(projectDir))
	if projectDir == "." {
		projectDir = ""
	}

	filtered := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		repoRelative := repoRelativeWhenModifiedPattern(projectDir, pattern)
		if whenModifiedPatternExcluded(repoRelative, excludeGlobs) {
			continue
		}
		filtered = append(filtered, pattern)
	}
	return filtered
}

func repoRelativeWhenModifiedPattern(projectDir, pattern string) string {
	pattern = filepath.ToSlash(pattern)
	var joined string
	if projectDir == "" {
		joined = filepath.Clean(pattern)
	} else {
		joined = filepath.Clean(filepath.Join(projectDir, pattern))
	}
	return filepath.ToSlash(joined)
}

func whenModifiedPatternExcluded(repoRelative string, excludeGlobs []string) bool {
	repoRelative = filepath.ToSlash(repoRelative)
	for _, exclude := range excludeGlobs {
		exclude = filepath.ToSlash(strings.TrimSpace(exclude))
		if exclude == "" {
			continue
		}
		if excludePatternMatches(repoRelative, exclude) {
			return true
		}
	}
	return false
}

func excludePatternMatches(path, exclude string) bool {
	if strings.Contains(exclude, "**") {
		prefix := strings.TrimSuffix(exclude, "**")
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix == "" {
			return true
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	matched, err := filepath.Match(exclude, path)
	if err == nil && matched {
		return true
	}
	return false
}

func mergedWhenModifiedExcludeGlobs(locals ResolvedLocals) []string {
	merged := make([]string, 0, len(locals.AtlantisWhenModifiedExclude)+len(whenModifiedExcludePaths))
	merged = append(merged, locals.AtlantisWhenModifiedExclude...)
	merged = append(merged, whenModifiedExcludePaths...)
	return uniqueStrings(merged)
}