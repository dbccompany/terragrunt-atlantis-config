package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/gruntwork-io/terragrunt/pkg/config"
	"github.com/gruntwork-io/terragrunt/pkg/log"
	"github.com/gruntwork-io/terragrunt/pkg/log/format"
	"github.com/gruntwork-io/terragrunt/pkg/options"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// quietTerragruntLogger is the logger handed to terragrunt's parser. Its
// diagnostics writer surfaces every HCL eval hiccup at its default ERROR
// level — in real repositories, catalog/template files evaluated outside
// their runtime context (get_path_from_repo_root() arithmetic on short
// paths, read_terragrunt_config into non-existent parents, sops locks)
// produce an unreadable wall of "Error: Invalid index" spam. Nothing
// user-actionable travels this channel: failures reach users as returned Go
// errors with positions (we log those ourselves). A bare formatter is still
// required: terragrunt's parser options dereference Logger.Formatter().
func quietTerragruntLogger() log.Logger {
	return log.New(
		log.WithOutput(io.Discard),
		log.WithFormatter(format.NewFormatter(format.NewBareFormatPlaceholders())),
	)
}

// newParsingContext wraps config.NewParsingContext with the quiet logger so
// call sites stay one-liners, mirroring the pre-0.98 API shape. Parsing
// contexts also pick up a sibling terragrunt.values.hcl when present.
func newParsingContext(ctx context.Context, opts *options.TerragruntOptions) *config.ParsingContext {
	_, pctx := config.NewParsingContext(ctx, quietTerragruntLogger(), opts)

	// Command-executing config functions must never run during generation.
	// PredefinedFunctions take precedence over terragrunt's defaults
	// (createTerragruntEvalContext copies them last), so these stubs fail
	// closed: a config that calls them errors out instead of silently
	// executing commands.
	pctx.PredefinedFunctions = map[string]function.Function{
		config.FuncNameRunCmd:          disabledExecFunction(config.FuncNameRunCmd),
		config.FuncNameSopsDecryptFile: disabledExecFunction(config.FuncNameSopsDecryptFile),
	}

	return attachSidecarValues(ctx, pctx)
}

// disabledExecFunction returns a function stub that always errors, so HCL
// evaluation cannot execute external commands during config generation.
func disabledExecFunction(name string) function.Function {
	return function.New(&function.Spec{
		VarParam: &function.Parameter{Name: "args", Type: cty.DynamicPseudoType},
		Type:     function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.DynamicVal, fmt.Errorf("the %q function is not available during config generation", name)
		},
	})
}
