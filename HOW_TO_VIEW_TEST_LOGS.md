# How to View Full Test Logs

## 1. GitHub Actions Workflow Logs

### During/After Workflow Run:
1. Go to: https://github.com/dbccompany/terragrunt-atlantis-config/actions
2. Click on the latest workflow run
3. Click on the job (e.g., `run-tests (ubuntu-latest, 1.25)`)
4. Click on the **"Run tests"** step
5. Scroll through the log output - it shows all test execution in real-time

### Test Log Artifacts (if tests fail):
1. Go to the workflow run page
2. Scroll down to the **"Artifacts"** section
3. Download `test-logs-ubuntu-latest` or `test-logs-windows-latest`
4. Extract and open `test_output.log` - contains full test output

## 2. Local Docker Test Logs

The full test output from the Docker run is saved locally:

```bash
cd /home/dboulas/work/dbcc/repos/terragrunt-atlantis-config
cat docker_test_output.log
# or
less docker_test_output.log
# or
tail -100 docker_test_output.log  # last 100 lines
```

## 3. Run Tests Locally to See Live Output

### Using Docker (recommended - matches CI):
```bash
cd /home/dboulas/work/dbcc/repos/terragrunt-atlantis-config
docker build -f Dockerfile.test -t terragrunt-atlantis-config-test:1.25 .
docker run --rm terragrunt-atlantis-config-test:1.25
```

### Using Local Go Installation:
```bash
cd /home/dboulas/work/dbcc/repos/terragrunt-atlantis-config
mkdir -p cmd/test_artifacts
CGO_ENABLED=0 GOFLAGS=-mod=readonly go test -v -cover -timeout 10m -count=1 -parallel 4 ./...
```

## 4. Workflow Configuration

The workflow is configured to:
- ✅ Capture all test output to `test_output.log` using `tee`
- ✅ Upload logs as artifacts when tests fail
- ✅ Display panic/fatal errors with context in GitHub UI
- ✅ Show full output on failure

## 5. Viewing Specific Test Failures

If a specific test fails, you can:

1. **In GitHub Actions logs**: Search for the test name (e.g., `TestSettingRoot`)
2. **In artifacts**: Download and search the log file
3. **Locally**: Run the specific test:
   ```bash
   go test -v -run TestSettingRoot ./cmd
   ```

## Quick Links

- **GitHub Actions**: https://github.com/dbccompany/terragrunt-atlantis-config/actions
- **Latest Run**: Check the `golang-upg` branch runs
- **Local Log**: `docker_test_output.log` in the repo root

