# Tools Directory

This directory contains a separate `go.mod` file for managing development tools without installing them globally.

## 🎯 Purpose

Instead of using `go install` to install tools globally, we use `go run` to run tools directly from their modules. This approach:

- ✅ **No global installations** - Tools don't pollute the global Go environment
- ✅ **Version pinning** - Tools are pinned to specific versions
- ✅ **Reproducible builds** - Same tool versions across all environments
- ✅ **Clean separation** - Tools are separate from the main application

## 📦 Available Tools

### `golangci-lint`
- **Version**: v1.64.8
- **Purpose**: Code linting and static analysis
- **Usage**: `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run`

### `gosec`
- **Version**: v2.19.0
- **Purpose**: Security scanning
- **Usage**: `go run github.com/securego/gosec/v2/cmd/gosec@v2.19.0 -fmt=json -out=security-report.json ./cache ./config ./pypi ./proxy ./integration`
- **Note**: Currently has issues with the main package due to flag package usage

## 🚀 Usage

### From Makefile
```bash
make lint      # Run linting (✅ Working)
make security  # Run security scan (⚠️ Has issues with main package)
make ci-ready  # Run all CI checks (includes tools)
```

### Direct Usage
```bash
# Run linting
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run

# Run security scan (with exclusions)
go run github.com/securego/gosec/v2/cmd/gosec@v2.19.0 -fmt=json -out=security-report.json -exclude=main.go ./cache ./config ./pypi ./proxy ./integration
```

## 🔧 Current Status

### ✅ Working
- **golangci-lint**: Fully functional with `go run` approach
- **Linting**: All linting commands work correctly
- **Version pinning**: Tools are pinned to specific versions

### ⚠️ Known Issues
- **gosec**: Has issues with the main package due to `flag` package usage
- **Security scanning**: May need alternative approach or tool configuration

## 🎯 Benefits Achieved

1. **No Global Pollution**: Tools don't install globally ✅
2. **Version Control**: Tool versions are tracked ✅
3. **CI Consistency**: Same tool versions in local and CI environments ✅
4. **Easy Updates**: Update tools by changing version in Makefile ✅
5. **Clean Environment**: No need to manage global tool installations ✅

## 📋 Migration from Global Tools

If you previously had tools installed globally, you can remove them:

```bash
# Remove global installations (if any)
go clean -i github.com/golangci/golangci-lint/cmd/golangci-lint
go clean -i github.com/securego/gosec/v2/cmd/gosec

# Verify tools work with go run approach
make lint
```

## 🔍 Troubleshooting

### gosec Issues
The gosec tool has issues with the main package due to the `flag` package usage. This is a known limitation and doesn't affect the core functionality of the tools approach.

### Alternative Security Scanning
If gosec continues to have issues, consider:
1. Using a different security scanning tool
2. Configuring gosec with different exclusions
3. Running security scans only on specific packages

## 📊 Success Metrics

- ✅ **Linting**: Works perfectly with `go run` approach
- ✅ **No global installations**: Tools run directly from modules
- ✅ **Version pinning**: Tools are pinned to specific versions
- ✅ **CI integration**: Works in both local and CI environments
- ⚠️ **Security scanning**: Has known issues with main package 