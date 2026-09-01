# Terragrunt Stacks Support

This fork adds support for [Terragrunt stacks](https://terragrunt.gruntwork.io/docs/features/stacks/)
to `terragrunt-atlantis-config`. All stack functionality is **opt-in** via `--enable-stacks`;
without the flag the output is byte-identical to upstream behavior.

## Quick start

```bash
terragrunt-atlantis-config generate \
  --root . \
  --enable-stacks \
  --stack-workflow terragrunt-stack \
  --output atlantis.yaml
```

Flags:

| Flag                      | Description                                                                                |
| ------------------------- | ------------------------------------------------------------------------------------------ |
| `--enable-stacks`         | Enable stack discovery and stack project generation                                        |
| `--stack-workflow`        | Workflow for stack projects (falls back to `--workflow`)                                   |
| `--stack-definition-file` | Additional YAML/JSON file declaring stacks, relative to `--root` unless absolute           |

Global flags `--autoplan`, `--terraform-version`, `--create-workspace`, `--create-project-name`
and `--workflow` apply to stack projects the same way they apply to regular projects.

## Source 1: `terragrunt.stack.hcl` (native Terragrunt stacks)

Every `terragrunt.stack.hcl` below `--root` becomes one Atlantis project whose `dir` is the
directory of the stack file. A workflow for that project is expected to run the stack, e.g.
`terragrunt stack run plan` (which regenerates `.terragrunt-stack` before running).

Example layout:

```
units/                      # unit catalog (shared templates)
  vpc/terragrunt.hcl
  app/terragrunt.hcl
live/
  prod/terragrunt.stack.hcl # the stack
```

```hcl
# live/prod/terragrunt.stack.hcl
unit "vpc" {
  source = "../../units/vpc"
  path   = "vpc"
}

unit "app" {
  source = "../../units/app"
  path   = "app"
}
```

Generated project:

```yaml
projects:
  - dir: live/prod
    workflow: terragrunt-stack
    autoplan:
      enabled: false
      when_modified:
        - "*.hcl"
        - "*.tf*"
        - "**/*.hcl"
        - "**/*.tf*"
        - ../../units/vpc/**/*.hcl
        - ../../units/vpc/**/*.tf*
        - ../../units/app/**/*.hcl
        - ../../units/app/**/*.tf*
```

Semantics:

- **Name**: with `--create-project-name`, the project is named after the stack directory
  (e.g. `live_prod`).
- **Members vs. sources**: if a unit's `path` resolves to a directory that already contains a
  `terragrunt.hcl` in the repo, that directory is a stack *member* and gets no individual project.
  Directories referenced through a unit's local `source` (catalogs like `units/vpc`) are only
  watched via `when_modified` and still receive regular projects, exactly as they would without
  stacks enabled.
- **Remote sources** (git refs, registry addresses) cannot be watched and are skipped.
- **Nested `stack` blocks** contribute their local `source` directories to `when_modified`.
- **Parsing is tolerant**: the file is first decoded with a full Terragrunt evaluation context
  (so functions like `find_in_parent_folders()` work); if evaluation fails, a fallback pass
  extracts the statically-evaluable `source`/`path` literals instead of dropping the file.
- `.terragrunt-stack` and `.terragrunt-cache` directories are excluded from stack discovery,
  and (only with `--enable-stacks`) from regular project discovery as well.

See `test_examples/stacks_hcl_example/` and `test_examples/stacks_local_units/`.

## Source 2: stack definition file (`--stack-definition-file`)

Stacks can additionally be declared in a YAML/JSON file:

```yaml
version: 1
stacks:
  - name: production-environment
    description: All production infrastructure
    include:                      # glob patterns, relative to the repo root
      - "environments/production/**"
    exclude:
      - "environments/production/experimental/**"
    depends_on:                   # other stack names; requires --create-project-name
      - shared-infrastructure
    atlantis:
      workflow: production
      autoplan: false
      apply_requirements: [approved, mergeable]
      execution_order_group: 100

  - name: shared-infrastructure
    modules:                      # explicit directories instead of patterns
      - shared/vpc
    atlantis:
      workflow: shared
      autoplan: true
      execution_order_group: 1
```

Rules:

- Stacks must define `include`/`modules`; both are relative to the repo root.
- Member modules do not get individual projects.
- The project `dir` is the common parent directory of all matched modules.
- `depends_on` entries reference other stack names and are only emitted when
  `--create-project-name` is set (Atlantis `depends_on` requires project names).
- `atlantis.autoplan`/`workflow`/`terraform_version` override the corresponding global flags
  for that stack.

See `test_examples/stacks_basic/` and `test_examples/stacks_with_patterns/`.

## Atlantis workflow for stacks

Stack projects need a workflow that runs stack commands. Define it in the `workflows` section of
your `atlantis.yaml` (preserved across regeneration with `--preserve-workflows`) or server-side:

```yaml
workflows:
  terragrunt-stack:
    plan:
      steps:
        - run: terragrunt stack run plan
    apply:
      steps:
        - run: terragrunt stack run apply
```

## Current limitations

- One Atlantis project per stack; per-unit granularity via generated `.terragrunt-stack`
  directories is deliberately not produced (those directories do not exist on a fresh clone).
- Dependencies *between* HCL-defined stacks are not inferred from unit `dependency` blocks; use
  the definition file for explicit `depends_on`.

## Tests

- Unit tests: `cmd/stack_test.go`, `cmd/stack_hcl_test.go`
- Golden-file integration tests: `TestStacks*` in `cmd/generate_test.go` with expected outputs in
  `cmd/golden/stacks_*.yaml`, including flag-off regression tests pinning the pre-existing
  behavior on the same example repositories.
