# Tejedor E2E Test Suite

This directory contains comprehensive end-to-end tests for Tejedor that validate the complete workflow with real PyPI packages.

## Overview

The E2E test suite validates:

1. **Real PyPI Integration**: Tests with actual PyPI packages from public PyPI
2. **Private PyPI Server**: Uses a real PyPI server container populated with packages
3. **Package Filtering**: Verifies that public PyPI packages only return source distributions (no wheels)
4. **Mixed Package Sources**: Tests packages from both private and public PyPI
5. **Pip Install Workflow**: Tests actual `pip install` commands using Tejedor as the index

## Test Components

### 1. Test PyPI Server (`Dockerfile`)

A UBI-based container that runs a real PyPI server (`pypiserver`) populated with packages downloaded from public PyPI. This simulates a private PyPI repository.

**Packages included:**
- `flask` and its dependencies (click, jinja2, werkzeug, etc.)
- `requests`
- `six`

### 2. Tejedor Proxy (`Dockerfile.tejedor`)

A container that builds and runs Tejedor, configured to proxy between:
- **Public PyPI**: `https://pypi.org/simple/`
- **Private PyPI**: The test PyPI server

### 3. Test Scripts

- **`e2e_test.go`**: Basic unit tests for the proxy functionality
- **`e2e_comprehensive_test.go`**: Comprehensive tests that build containers and test pip installs
- **`test_e2e.sh`**: Manual test script for running the full E2E workflow

## Running the Tests

### Prerequisites

- Docker
- Python 3
- Go 1.21+
- curl

### Quick Test (Manual)

```bash
cd e2e
./test_e2e.sh
```

This script will:
1. Build and start the test PyPI server
2. Build Tejedor
3. Start Tejedor proxy
4. Run pip install tests with various requirements files
5. Verify filtering behavior
6. Clean up all resources

### Docker Compose (Alternative)

```bash
cd e2e
docker-compose up --build
```

### Individual Test Components

#### Test PyPI Server Only

```bash
cd e2e
docker build -t tejedor-test-pypi -f Dockerfile .
docker run -d --name test-pypi -p 8080:8080 tejedor-test-pypi
```

#### Test Tejedor Proxy Only

```bash
cd e2e
go build -o tejedor ../..
./tejedor -config config.json
```

## Test Scenarios

### 1. Private Packages Only

Tests packages that exist only in the private PyPI server:

```bash
pip install flask click jinja2 -i http://127.0.0.1:8081/simple/
```

**Expected behavior**: Packages served from private PyPI, can include both source and wheel distributions.

### 2. Public Packages Only

Tests packages that exist only in public PyPI:

```bash
pip install numpy pandas matplotlib -i http://127.0.0.1:8081/simple/
```

**Expected behavior**: Packages served from public PyPI, **filtered to source distributions only** (no wheel files).

### 3. Mixed Packages

Tests packages from both sources:

```bash
pip install flask numpy requests click -i http://127.0.0.1:8081/simple/
```

**Expected behavior**: 
- `flask` (private): Can have wheels
- `numpy` (public): Source distributions only
- `requests` (public): Source distributions only
- `click` (private): Can have wheels

## Filtering Behavior

The key feature being tested is that **packages from public PyPI are filtered to remove wheel files**, ensuring only source distributions are served. This is important for:

- **Security**: Source distributions are more transparent and auditable
- **Compatibility**: Source distributions work across more platforms
- **Compliance**: Some environments require source-only packages

## CI Integration

The E2E tests are integrated into the GitHub Actions CI workflow:

1. **Basic E2E Tests**: Run as Go tests with coverage
2. **Comprehensive E2E Tests**: Run the full container-based test suite

The comprehensive tests are skipped if Docker is not available in the CI environment.

## Troubleshooting

### Common Issues

1. **Port conflicts**: Ensure ports 8080 and 8081 are available
2. **Docker permissions**: Ensure Docker daemon is running and accessible
3. **Network issues**: The tests require internet access to download packages

### Debug Mode

To run with verbose output:

```bash
cd e2e
DEBUG=1 ./test_e2e.sh
```

### Manual Verification

To manually verify the filtering behavior:

```bash
# Check numpy (public) - should have no .whl files
curl http://127.0.0.1:8081/simple/numpy/

# Check flask (private) - can have .whl files
curl http://127.0.0.1:8081/simple/flask/
```

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Public PyPI   │    │   Tejedor       │    │   Private PyPI  │
│  (pypi.org)     │◄──►│    Proxy        │◄──►│   (Container)   │
│                 │    │                 │    │                 │
│ - numpy         │    │ - Routes        │    │ - flask         │
│ - pandas        │    │ - Filters       │    │ - click         │
│ - matplotlib    │    │ - Caches        │    │ - jinja2        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   pip install   │
                       │   -i proxy      │
                       └─────────────────┘
```

This architecture ensures that:
- Private packages are served unfiltered (can have wheels)
- Public packages are filtered (source distributions only)
- The proxy correctly routes requests to the appropriate source
- Real pip installs work end-to-end 