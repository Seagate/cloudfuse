# Azure Storage Fuse (cloudfuse)

Cloudfuse is a  FUSE filesystem driver that provides virtual filesystem backed by S3 or Azure Blob Storage. It uses libfuse (fuse3) to communicate with the Linux FUSE kernel module and implements filesystem operations using the AWS S3 or Azure Storage REST APIs.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap, Build, and Test the Repository

**CRITICAL**: All build and test commands include specific timeout warnings. NEVER CANCEL long-running operations.

- Install required dependencies:
  ```bash
  sudo apt update
  sudo apt install -y libfuse3-dev fuse3 gcc
  ```

- Install Go 1.25.4+ (already available in most environments):
  ```bash
  go version  # Should show 1.25.4 or higher
  ```

- Build cloudfuse binary:
  ```bash
  ./build.sh
  ```
  **Timing**: ~30 seconds. NEVER CANCEL. Use timeout 120+ seconds.

- Build health monitor binary:
  ```bash
  ./build.sh health
  ```
  **Timing**: ~5 seconds. Use timeout 60+ seconds.

- Verify binary functionality:
  ```bash
  ./cloudfuse --version
  ./cloudfuse -h
  ```

### Testing

- Run unit tests (core components only):
  ```bash
  go test -v -timeout=10m ./internal/... ./common/... --tags=unittest,fuse3
  ```
  **Timing**: ~2 minutes. NEVER CANCEL. Use timeout 15+ minutes.

- Run full unit tests (some may fail without Azure credentials):
  ```bash
  go test -v -timeout=45m ./... --tags=unittest,fuse3
  ```
  **WARNING**: Expected network/credential test failures. **Timing**: ~5-10 minutes. NEVER CANCEL. Use timeout 60+ minutes.

- Run linting:
  ```bash
  # Install golangci-lint if not available
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
  
  # Run linting
  $(go env GOPATH)/bin/golangci-lint run --tests=false --build-tags fuse3 --max-issues-per-linter=0
  ```
  **Timing**: ~10 seconds. Use timeout 60+ seconds.

- Check code formatting:
  ```bash
  gofmt -s -l -d .
  ```
  **Timing**: ~2 seconds. Should return no output if properly formatted.

### Validation Scenarios

**ALWAYS test these scenarios after making changes**:

1. **Binary Creation and Basic Commands**:
   ```bash
   ./build.sh
   ./cloudfuse --version
   ./cloudfuse -h
   ./cloudfuse mount --help
   ```

2. **Config Generation**:
   ```bash
   mkdir -p /tmp/cloudfuse-test
   ./cloudfuse gen-config --tmp-path=/tmp/cloudfuse-test --o /tmp/cloudfuse-test/config.yaml
   cat /tmp/cloudfuse-test/config.yaml
   ```

3. **Health Monitor**:
   ```bash
   ./build.sh health
   ./cfusemon --help
   ```

4. **Format and Lint Validation**:
   ```bash
   gofmt -s -l -d .  # Should return no output
   $(go env GOPATH)/bin/golangci-lint run --tests=false --build-tags fuse3 --max-issues-per-linter=0
   ```

## Build System Details

- **Primary Build Script**: `./build.sh` - builds cloudfuse with fuse3 by default
- **Build Variants**: 
  - `./build.sh` - standard fuse3 build
  - `./build.sh fuse2` - legacy fuse2 build
  - `./build.sh health` - health monitor binary
- **Output**: `cloudfuse` binary (~30MB) and optionally `cfusemon` binary (~6MB)
- **Go Version**: Requires Go 1.25.4+ (specified in go.mod)
- **Tags**: Use `fuse3` tag for testing/building (default), `fuse2` for legacy systems

## Testing Infrastructure

- **Unit Tests**: Use `--tags=unittest,fuse3` to run unit tests
- **E2E Tests**: Located in `test/e2e_tests/` - require Azure Storage credentials
- **Mount Tests**: `test/mount_test/` - comprehensive filesystem testing
- **Performance Tests**: `test/scripts/` - benchmarking and stress testing
- **Test Timeout**: Mount tests can take up to 120 minutes - NEVER CANCEL

## Key Components and Architecture

- **cmd/**: CLI commands and main entry points
- **component/**: Core components (libfuse, azstorage, caching)
- **common/**: Shared utilities, configuration, logging
- **internal/**: Internal APIs and pipeline management
- **test/**: All testing code and scripts
- **tools/health-monitor/**: cloudfuse monitoring tool

## Configuration

- **Sample Configs**: 
  - `sampleFileCacheConfig.yaml` - file-based caching
  - `sampleBlockCacheConfig.yaml` - block-based caching
  - `setup/baseConfig.yaml` - complete configuration options
- **Config Generation**: Use `cloudfuse gen-config` to auto-generate configs
- **Authentication**: Supports account keys, SAS tokens, MSI, SPN, Azure CLI

## Important Notes

- **Mount Operations**: Require Azure Storage credentials - will fail in testing without them
- **Permissions**: May require sudo for actual mount operations
- **FUSE Configuration**: `/etc/fuse.conf` may need `user_allow_other` enabled for multi-user access
- **Dependencies**: Requires libfuse3-dev for building, fuse3 for runtime
- **Platform**: Linux only (Ubuntu 20+, other distros listed in wiki)

## Common Pre-commit Validation

Always run these before committing changes:

```bash
# Format check
gofmt -s -l -d .

# Build verification
./build.sh

# Core unit tests
go test -v -timeout=10m ./internal/... ./common/... --tags=unittest,fuse3

# Linting
$(go env GOPATH)/bin/golangci-lint run --tests=false --build-tags fuse3 --max-issues-per-linter=0

# Binary functionality
./cloudfuse --version
./cloudfuse gen-config --tmp-path=/tmp/test --o /tmp/test-config.yaml
```

## CI/CD Integration

- **Build Pipeline**: Github Actions
- **Testing**: Automated on Ubuntu 24, both x86 and ARM64
- **Performance**: Dedicated benchmark workflows
- **Security**: CodeQL analysis and dependency scanning
- **Release**: Automated package building and distribution

## Troubleshooting

- **Build Failures**: Check Go version, ensure libfuse3-dev installed
- **Test Failures**: Network/credential tests expected to fail without Azure setup
- **Mount Failures**: Verify FUSE permissions and Azure credentials
- **Performance**: Use health monitor (`cfusemon`) for runtime diagnostics

## Key Files to Monitor

When making changes, always check these files for consistency:
- `go.mod` - dependency versions
- `main.go` - entry point
- `build.sh` - build configuration
- `cmd/mount.go` - core mount functionality
- Configuration templates in `setup/` and root directory