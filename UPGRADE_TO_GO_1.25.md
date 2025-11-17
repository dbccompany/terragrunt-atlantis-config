# Upgrade to Go 1.25

## Summary

The project has been successfully updated to use **Go 1.25**. 


## Changes Made

### 1. `go.mod`
- Updated from `go 1.23.5` to `go 1.25`

### 2. `Dockerfile`
- Updated `ARG GO_VERSION=1.23` to `ARG GO_VERSION=1.25`

### 3. CI/CD Workflows
- **`.github/workflows/run_tests.yml`**: Updated `golang-version: ["1.25"]`
- **`.github/workflows/release.yml`**: Updated `go-version: "^1.25"`
- Added fallback cache keys for Go 1.23 to maintain cache efficiency during transition

### 4. `README.md`
- Updated documentation to reflect Go 1.25 support

## Go 1.25 Features & Improvements

According to the Go 1.25 release notes, this version includes:

1. **Container-Aware GOMAXPROCS**: Automatically adjusts CPU cores based on container limits
2. **DWARF 5 Debug Information**: Smaller debug data and faster linking
3. **Experimental Garbage Collector**: Optional "greenteagc" for improved memory management
4. **New Testing Package**: `testing/synctest` for concurrent code testing
5. **Nil Pointer Fix**: Compiler bug fix that may cause previously working code to correctly panic on nil dereferences

## Important Considerations

### Breaking Changes
- **Nil Pointer Dereference Checks**: A compiler bug fix means code that previously executed without panicking may now correctly panic if it dereferences a nil pointer before checking for errors. Review your error handling patterns.

### Testing Required
Before deploying, ensure you:
1. Run the full test suite: `go test -v ./...`
2. Verify all dependencies resolve: `go mod tidy`
3. Test builds on all target platforms
4. Review any nil pointer usage in error handling paths

### Dependency Compatibility
The project's dependencies should be compatible with Go 1.25, but verify:
- `github.com/gruntwork-io/terragrunt v0.72.5` - Check their go.mod for compatibility
- All other dependencies should work with Go 1.18+ (Go 1.25 is backward compatible)

## Next Steps

1. **Install Go 1.25** on your development machine:
   ```bash
   # Download from https://go.dev/dl/
   # Or use your system's package manager
   ```

2. **Verify the upgrade**:
   ```bash
   go mod tidy
   go test -v ./...
   make build
   ```

3. **Monitor CI/CD**: The GitHub Actions workflows will automatically use Go 1.25 on the next push

## Rollback Plan

If issues arise, you can rollback by:
1. Reverting the changes in `go.mod`, `Dockerfile`, and workflow files
2. Or use git to revert: `git revert <commit-hash>`

## Go 1.26 Status

Go 1.26 is **not yet available** (expected February 2026). Once released, you can follow a similar process to upgrade to 1.26.

