# Tejedor

A Go application that acts as a proxy for PyPI (Python Package Index), implementing the [Simple Repository API](https://packaging.python.org/en/latest/specifications/simple-repository-api/). The proxy intelligently routes requests between a public PyPI index and a private index based on package availability.

> **Why "Tejedor"?** Tejedor means "weaver" in Spanish. This project weaves together packages from multiple indexes, creating a seamless experience by intelligently routing between public and private PyPI sources.

## Features

- **Dual Index Support**: Routes requests between public PyPI and a private index
- **Intelligent Routing**:
  - Packages only in public PyPI → served from public index
  - Packages in both indexes → served from private index (priority)
  - Packages only in private index → served from private index
  - Packages in `public_only_packages` list → always served from public index (even if they exist in private index)
- **Security Protection**: Binary wheels are only served from private repositories, while only source distributions (sdists) are allowed from public PyPI, protecting users from compromised builds
- **LRU Caching**: Configurable cache with TTL for package existence information
- **Response Headers**: Includes `X-PyPI-Source` header indicating which index served the content
- **Simple Repository API**: Full compatibility with PyPI's simple repository API

## New Features (2024)

### HTML/Version List Caching
- The proxy now caches the full HTML content of `/simple/{package}/` responses (the version list page) for both public and private indexes.
- This reduces backend calls and improves performance for repeated requests to the same package page.
- The cache is LRU with TTL, and can be configured or disabled via config.
- Both package existence and HTML content are cached independently.

### Health Endpoint
- A new health endpoint is available at `/health`.
- Returns JSON with cache statistics, including counts for public/private package existence and public/private HTML page caches.
- Example:
  ```json
  {
    "status": "healthy",
    "cache": {
      "enabled": true,
      "public_packages": 123,
      "private_packages": 45,
      "public_pages": 67,
      "private_pages": 12
    }
  }
  ```

### Public-Only Packages
- Configure specific packages to always be served from the public PyPI index, even if they exist in your private index.
- This is useful for update workflows where you want to check the public index for newer versions of certain packages.
- Add packages to the `public_only_packages` list in your configuration:
  ```yaml
  public_only_packages:
    - requests
    - pydantic
    - fastapi
  ```
- When a package is in this list, it will always be served from the public index, regardless of whether it exists in your private index.
- If a public-only package doesn't exist in the public index, the request will return a 404 error.

## Code Quality

This project maintains high code quality standards with:
- **Go Linting**: Uses `golangci-lint` with comprehensive rules including `govet`, `errcheck`, `staticcheck`, and more
- **Test Coverage**: Maintains >80% test coverage across all packages
- **Race Detection**: All tests run with `-race` flag to detect race conditions
- **Security Scanning**: Uses `gosec` for security vulnerability detection

## Hermeto Integration

Tejedor integrates with Hermeto (Cachi2) through a Tekton task that provides PyPI proxy functionality for Python dependency prefetching.

### Tekton Task: `prefetch-dependencies-tejedor`

The task automatically detects Python pip dependencies and starts Tejedor as a sidecar container to proxy PyPI requests.

**Features:**
- Automatic Python dependency detection
- Tejedor sidecar with configurable private PyPI URL
- Optional proxy server support
- Wheel download support for private packages
- Source-only filtering for public packages

**Usage:**
```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: prefetch-dependencies-tejedor
spec:
  params:
    - name: input
      value: "pip"
    - name: private-pypi-url
      value: "https://your-private-pypi.com/simple/"
    - name: proxy-server
      value: "http://proxy.company.com:8080"  # optional
```

**Documentation:**
- [Task Documentation](task/prefetch-dependencies-tejedor/0.2/README.md)
- [Local Testing Guide](task/prefetch-dependencies-tejedor/0.2/LOCAL_TESTING.md)
- [Test Example](task/prefetch-dependencies-tejedor/0.2/test-example.yaml)

## Quick Start

### Prerequisites

- Go 1.21 or later
- Access to a private PyPI index

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd python-index-proxy
```

2. Install dependencies:
```bash
go mod download
```

3. Create a configuration file:
```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your private PyPI URL
```

4. Build the application:
```bash
go build -o pypi-proxy
```

5. Run the proxy:
```bash
./pypi-proxy
```

The proxy will start on port 8080 by default.

### Command Line Flags

You can also configure the proxy using command line flags:

```bash
./pypi-proxy --private-pypi-url="https://your-private-pypi.com/simple/" --port=9090 --cache-enabled=false
```

Available flags:
- `--private-pypi-url`: URL of the private PyPI server (required)
- `--public-pypi-url`: URL of the public PyPI server (default: https://pypi.org/simple/)
- `--port`: Port to listen on (default: 8080)
- `--cache-enabled`: Enable caching (default: true)
- `--cache-size`: Cache size in entries (default: 20000)
- `--cache-ttl-hours`: Cache TTL in hours (default: 12)
- `--config`: Path to configuration file

## Configuration

### Configuration File

Create a `config.yaml` file in the project root:

```yaml
public_pypi_url: "https://pypi.org/simple/"
private_pypi_url: "https://your-private-pypi.com/simple/"
port: 8080
cache_enabled: true
cache_size: 20000
cache_ttl_hours: 12
public_only_packages:
  - requests
  - pydantic
  - fastapi
```

### Environment Variables

You can also configure the proxy using environment variables:

```bash
export PYPI_PROXY_PRIVATE_PYPI_URL="https://your-private-pypi.com/simple/"
export PYPI_PROXY_PORT="9090"
export PYPI_PROXY_CACHE_ENABLED="true"
export PYPI_PROXY_CACHE_SIZE="10000"
export PYPI_PROXY_CACHE_TTL_HOURS="6"
export PYPI_PROXY_PUBLIC_ONLY_PACKAGES="requests,pydantic,fastapi"
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `public_pypi_url` | string | `https://pypi.org/simple/` | URL of the public PyPI index |
| `private_pypi_url` | string | (required) | URL of your private PyPI index |
| `port` | int | `8080` | Port to run the proxy server on |
| `cache_enabled` | bool | `true` | Enable/disable caching |
| `cache_size` | int | `20000` | Maximum number of cache entries |
| `cache_ttl_hours` | int | `12` | Cache TTL in hours |
| `public_only_packages` | []string | `[]` | List of packages that should always be served from the public index |

## Usage

### Command Line Options

```bash
./pypi-proxy [options]

Options:
  -config string
        Path to configuration file (default searches for config.yaml)
```

### Using the Proxy

Once running, the proxy exposes the Simple Repository API at the configured port:

- **Package Index**: `http://127.0.0.1:8080/simple/`
- **Package Page**: `http://127.0.0.1:8080/simple/{package_name}/`
- **Package Files**: `http://127.0.0.1:8080/packages/{file_path}`

### Example Usage

1. **Install a package using pip**:
```bash
pip install --index-url http://127.0.0.1:8080/simple/ pycups
```

2. **Install a package that exists in both indexes**:
```bash
pip install --index-url http://127.0.0.1:8080/simple/ pydantic
```

3. **Check which index served a package**:
```bash
curl -I http://127.0.0.1:8080/simple/pycups/
# Look for X-PyPI-Source header in response
```

## Testing

The application includes comprehensive testing with multiple test types:

### Testcontainers Integration

The project now uses [testcontainers-go](https://golang.testcontainers.org/) for end-to-end testing, providing:

- **Automated Container Management**: Containers are automatically started and stopped for each test
- **Podman Support**: Full compatibility with Podman container runtime
- **Isolated Test Environment**: Each test gets its own clean container environment
- **Automatic Cleanup**: Containers are automatically removed after tests complete
- **Health Checks**: Tests wait for services to be ready before running

#### Testcontainers Benefits

1. **Simplified Testing**: No more complex shell scripts or Docker Compose dependencies
2. **Better Isolation**: Each test runs in its own container environment
3. **Improved CI/CD**: More reliable and portable test execution
4. **Developer Experience**: Cleaner, more maintainable test code

#### Running Testcontainers Tests

```bash
# Build test images and run testcontainers tests
make test-e2e-testcontainers

# Or manually build images and run tests
podman build -t tejedor:test -f e2e/Dockerfile.tejedor .
podman build -t tejedor-test-pypi:latest -f e2e/Dockerfile .
go test -v ./e2e -run Test.*Testcontainers
```

### Unit Tests
- **Cache Tests**: Test cache functionality including creation, operations, expiration, and statistics
- **Config Tests**: Test configuration management, environment variables, and file loading
- **PyPI Client Tests**: Test HTTP client functionality, package existence, and file handling
- **Proxy Tests**: Test main proxy functionality, HTTP handlers, and error scenarios

### Integration Tests
- **Local PyPI Server**: Tests use a local mock PyPI server instead of external dependencies
- **Real Public PyPI**: Tests against the actual public PyPI index for realistic validation
- **Cache Integration**: Tests caching behavior with real network calls
- **File Handling**: Tests file proxying functionality
- **Error Handling**: Tests with invalid URLs and network failures
- **Request Validation**: Tests invalid request handling
- **Source Filtering**: Tests public vs private index behavior

### Test Features
- **No External Dependencies**: Integration tests use a local PyPI server, eliminating dependency on external private indexes
- **Comprehensive Coverage**: Tests all major functionality including caching, file handling, and error scenarios
- **Real Integration**: Tests against actual public PyPI index for realistic validation
- **Race Detection**: Built-in race condition testing
- **Coverage Reporting**: Built-in coverage analysis

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run only unit tests
go test ./cache/... ./config/... ./pypi/... ./proxy/...

# Run only integration tests
go test ./integration/...

# Run tests with race detection
go test -race ./...

# Run testcontainers-based e2e tests
make test-e2e-testcontainers
```

## Development

### Project Structure

```
python-index-proxy/
├── main.go              # Application entry point
├── config/              # Configuration management
│   ├── config.go
│   └── config_test.go
├── cache/               # LRU cache implementation
│   ├── cache.go
│   └── cache_test.go
├── pypi/                # PyPI client and constants
│   ├── client.go
│   └── client_test.go
├── proxy/               # Main proxy logic
│   └── proxy.go
├── integration/         # Integration tests
│   └── integration_test.go
├── config.yaml          # Configuration file
├── go.mod              # Go module file
└── README.md           # This file
```

### Building

Build for current platform:
```bash
go build -o pypi-proxy
```

Build for specific platform:
```bash
GOOS=linux GOARCH=amd64 go build -o pypi-proxy-linux
GOOS=darwin GOARCH=amd64 go build -o pypi-proxy-darwin
GOOS=windows GOARCH=amd64 go build -o pypi-proxy-windows.exe
```

## Response Headers

The proxy adds the following response headers:

- `X-PyPI-Source`: Indicates which index served the content
  - `public`: Content served from public PyPI
  - `private`: Content served from private PyPI
  - `proxy`: Content served by the proxy itself (index page)

## Caching

The proxy uses an LRU cache to store package existence information:

- **Cache Size**: Configurable (default: 20,000 entries)
- **TTL**: Configurable (default: 12 hours)
- **Disable**: Set `cache_enabled: false` for integration tests

Cache statistics are logged when the server starts.

## Troubleshooting

### Common Issues

1. **Private PyPI URL not configured**:
   ```
   Error: private_pypi_url is required
   ```
   Solution: Set the `private_pypi_url` in config.yaml or environment variable.

2. **Port already in use**:
   ```
   Error: listen tcp :8080: bind: address already in use
   ```
   Solution: Change the port in configuration or stop the service using port 8080.

3. **Network connectivity issues**:
   ```
   Error: error checking public index: Get "https://pypi.org/simple/...": dial tcp: i/o timeout
   ```
   Solution: Check network connectivity and firewall settings.

4. **File download errors (404 Not Found)**:
   ```
   ERROR: HTTP error 404 while getting http://127.0.0.1:8080/package-version.whl
   ```
   Solution: This issue has been fixed in recent versions. The proxy now correctly handles direct file requests for wheel files and other package distributions. Make sure you're using the latest version of the proxy.

### Logs

The proxy logs all HTTP requests and startup information. Check the console output for:
- Server startup messages
- Configuration details
- Request logs
- Error messages

### Debug Mode

For debugging, you can run with verbose logging:
```bash
go run main.go -config config.yaml
```

## Security Considerations

- The `config.yaml` file contains sensitive URLs and should not be committed to version control
- The proxy forwards all headers from upstream responses
- Consider using HTTPS for the proxy in production environments
- The private PyPI URL should be kept secure and not exposed publicly

### Binary Wheel Protection

One of the key security features of this proxy is its protection against compromised binary wheels:

- **Private Repository Binary Wheels**: Binary wheels (`.whl` files) are only served from your private PyPI repository, ensuring you have full control over the build process and can verify the integrity of compiled packages
- **Public PyPI Source Distributions Only**: From public PyPI, only source distributions (`.tar.gz` files) are allowed, which must be compiled locally during installation
- **Compromise Prevention**: This approach prevents users from installing potentially compromised pre-compiled binaries from public sources, as all binary wheels come from your trusted private repository
- **Build Transparency**: By requiring local compilation of public packages, users can inspect the source code and build process, providing transparency and reducing the attack surface

This security model ensures that your users are protected from supply chain attacks while still maintaining access to the vast ecosystem of Python packages available on public PyPI.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

Copyright (C) 2024 Brian Cook <bcook@redhat.com>

## Development: Linting and Formatting

To ensure your code matches CI checks, use the provided Makefile targets:

- `make fmt` — Formats code using gofumpt (same as CI expects)
- `make lint` — Runs golangci-lint (same as CI)
- `make security` — Runs gosec security scan (same as CI)

### Setup

**Important**: Use the same Go version and tool versions as CI to avoid discrepancies.

**Go Version**: Use Go 1.21 (same as CI)

**Install tools with specific versions**:

```bash
# Install Go 1.21 if you don't have it
# go install golang.org/dl/go1.21@latest
# go1.21 download

# Install tools with versions matching CI
go install mvdan.cc/gofumpt@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

### Recommended workflow

1. Run `make fmt` before committing to auto-format your code.
2. Run `make lint` to catch lint issues before pushing.
3. Run `make security` to check for security issues.

**Note**: The Makefile uses full paths to installed binaries to ensure consistency. If you get "command not found" errors, make sure you've installed the tools using the commands above.

This will help minimize discrepancies between local and CI runs.