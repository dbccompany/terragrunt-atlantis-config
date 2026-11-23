package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhenModifiedPatternExcluded_modulesGlob(t *testing.T) {
	assert.True(t, whenModifiedPatternExcluded(
		"_modules/widget/main.tf",
		[]string{"_modules/**"},
	))
	assert.True(t, whenModifiedPatternExcluded(
		"_modules/widget/*.tf*",
		[]string{"_modules/**"},
	))
	assert.False(t, whenModifiedPatternExcluded(
		"consumer-dev/terragrunt.hcl",
		[]string{"_modules/**"},
	))
}

func TestFilterWhenModifiedExcludesConsumerModule(t *testing.T) {
	patterns := []string{
		"*.hcl",
		"*.tf*",
		"../../../_modules/widget/*.tf*",
		"../../../_modules/widget/main.tf",
	}
	out := filterWhenModifiedExcludes("live/dev/app", patterns, []string{"_modules/**"})
	assert.Equal(t, []string{"*.hcl", "*.tf*"}, out)
}

func TestRepoRelativeWhenModifiedPattern(t *testing.T) {
	got := repoRelativeWhenModifiedPattern(
		"live/dev/app",
		"../../../_modules/widget/*.tf*",
	)
	assert.Equal(t, "_modules/widget/*.tf*", got)
}