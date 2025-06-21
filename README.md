# Tejedor

A Go application that acts as a proxy for PyPI (Python Package Index), implementing the [Simple Repository API](https://packaging.python.org/en/latest/specifications/simple-repository-api/). The proxy intelligently routes requests between a public PyPI index and a private index based on package availability.

> **Why "Tejedor"?** Tejedor means "weaver" in Spanish. This project weaves together packages from multiple indexes, creating a seamless experience by intelligently routing between public and private PyPI sources.

## Features

- **Dual Index Support**: Routes requests between public PyPI and a private index
- **Intelligent Routing**: 
  - Packages only in public PyPI → served from public index
  - Packages in both indexes → served from private index (priority)
  - Packages only in private index → served from private index
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
```

### Environment Variables

You can also configure the proxy using environment variables:

```bash
export PYPI_PROXY_PRIVATE_PYPI_URL="https://your-private-pypi.com/simple/"
export PYPI_PROXY_PORT="9090"
export PYPI_PROXY_CACHE_ENABLED="true"
export PYPI_PROXY_CACHE_SIZE="10000"
export PYPI_PROXY_CACHE_TTL_HOURS="6"
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

- **Package Index**: `http://localhost:8080/simple/`
- **Package Page**: `http://localhost:8080/simple/{package_name}/`
- **Package Files**: `http://localhost:8080/packages/{file_path}`

### Example Usage

1. **Install a package using pip**:
```bash
pip install --index-url http://localhost:8080/simple/ pycups
```

2. **Install a package that exists in both indexes**:
```bash
pip install --index-url http://localhost:8080/simple/ pydantic
```

3. **Check which index served a package**:
```bash
curl -I http://localhost:8080/simple/pycups/
# Look for X-PyPI-Source header in response
```

## Testing

### Unit Tests

Run unit tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

### Integration Tests

Run integration tests (requires network access):
```bash
go test ./integration/...
```

Run integration tests with cache testing:
```bash
go test -v ./integration/ -run TestProxyWithCache
```

### Test Examples

The integration tests include examples for:
- **pycups**: Package only in public PyPI (served from public)
- **pydantic**: Package in both indexes (served from private)

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