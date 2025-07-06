# Helm API

A simple HTTP API server that provides Helm templating functionality without requiring a Kubernetes cluster connection. This service accepts GET requests with query parameters and returns rendered YAML manifests. Supports local charts, repository charts, and OCI registry charts.

## Features

- **Client-only templating**: Templates Helm charts without connecting to a Kubernetes cluster
- **RESTful API**: Simple HTTP GET endpoint with query parameters
- **Multiple chart sources**: Support for local charts, HTTP/HTTPS repositories, and OCI registries
- **Configurable**: Support for all major Helm template options including values, files, and Kubernetes versions
- **Container-ready**: Built with GoReleaser and ko for easy container deployment

## API Endpoint

### GET /template

Renders a Helm chart template and returns the resulting YAML manifests using query parameters.

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chart` | string | Yes | Path to the Helm chart, chart name, or OCI registry URL |
| `chart_version` | string | No | Version of the chart (works with repository charts) |
| `repository` | string | No | Chart repository URL (HTTP/HTTPS or OCI registry) |
| `release_name` | string | No | Name of the release (default: "release-name") |
| `namespace` | string | No | Target namespace (default: "default") |
| `kube_version` | string | No | Kubernetes version for capabilities (default: "v1.29.0") |
| `set` | array | No | Set values (equivalent to `--set`, can be used multiple times) |
| `value_files` | array | No | Specify values files (can be used multiple times) |
| `file_values` | array | No | Set values from files (can be used multiple times) |
| `json_values` | array | No | Set JSON values (can be used multiple times) |
| `show_only` | array | No | Only show specific template files (can be used multiple times) |
| `api_versions` | array | No | Additional API versions for capabilities (can be used multiple times) |
| `include_crds` | boolean | No | Include CRDs in output |
| `skip_tests` | boolean | No | Skip test manifests |
| `is_upgrade` | boolean | No | Set .Release.IsUpgrade instead of .Release.IsInstall |
| `validate` | boolean | No | Enable manifest validation |

#### Response

Returns the rendered YAML manifests with `Content-Type: application/x-yaml`.

## Usage Examples

### Basic template request

```bash
curl "http://localhost:8080/template?chart=nginx&release_name=my-nginx"
```

### With custom values

```bash
curl "http://localhost:8080/template?chart=./my-chart&set=replicaCount=3&set=image.tag=v2.0"
```

### Multiple parameters

```bash
curl "http://localhost:8080/template?chart=nginx&release_name=test&namespace=production&set=replicas=2&include_crds=true&kube_version=v1.28.0"
```

### Using array parameters

```bash
curl "http://localhost:8080/template?chart=./my-chart&set=key1=value1&set=key2=value2&show_only=deployment.yaml&show_only=service.yaml"
```

### With chart repository

```bash
curl "http://localhost:8080/template?chart=nginx&repository=https://charts.bitnami.com/bitnami&chart_version=15.0.0&release_name=my-nginx"
```

### With OCI registry

```bash
curl "http://localhost:8080/template?chart=oci://registry-1.docker.io/bitnamicharts/nginx&chart_version=15.0.0&release_name=oci-nginx"
```

### Health Check

```bash
curl "http://localhost:8080/health"
```

## Installation

### Using Go

```bash
go install github.com/appkins-org/helm-api@latest
```

### Building from Source

```bash
git clone https://github.com/appkins-org/helm-api.git
cd helm-api
go build -o helm-api .
```

## Building and Running

### Local Development

```bash
# Install dependencies
go mod tidy

# Run the server
go run main.go

# The server will start on port 8080
```

### Using Docker

```bash
# Build the container
docker build -t helm-api .

# Run the container
docker run -p 8080:8080 helm-api
```

### Using GoReleaser with ko

```bash
# Build and publish container (requires Docker and ko)
goreleaser release --snapshot --clean

# Or just build containers
goreleaser build --snapshot --clean
```

## Configuration

The server can be configured using environment variables:

- `PORT`: Server port (default: 8080)
- `HELM_DEBUG`: Enable Helm debug logging (default: false)

### Chart Authentication

For private repositories or OCI registries, you may need to configure authentication:

- **HTTP/HTTPS repositories**: Use `helm repo add` to configure credentials before running the server
- **OCI registries**: Use `helm registry login` or configure Docker credentials
- **Local charts**: Ensure the chart path is accessible to the server process

## Requirements

- Go 1.21 or later
- Helm charts accessible to the server
- For repository charts: Network access to chart repositories
- For OCI charts: Network access to OCI registries and proper authentication if required
- For container builds: Docker and optionally ko

## Testing

The project includes comprehensive tests for both the HTTP server and the template processing functionality.

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test ./... -cover

# Run benchmarks
go test ./... -bench=.
```

### Test Coverage

- **HTTP Server Tests (`main_test.go`)**: Tests all HTTP endpoints including error cases, content-type validation, and mock HTTP server functionality
- **Template Package Tests (`internal/template/template_test.go`)**: Tests template request validation, Kubernetes version parsing, value merging, and helper functions
- **Integration Tests**: Includes a full test chart creation and processing test

### Manual Testing

You can test the API manually by starting the server and making HTTP requests:

```bash
# Start the server
go run main.go

# Test health endpoint
curl "http://localhost:8080/health"

# Test template endpoint
curl "http://localhost:8080/template?chart=nginx&release_name=test"
```

## Development

This project uses:

- **Helm SDK**: For chart templating functionality
- **Go standard library**: For HTTP server
- **GoReleaser**: For building and releasing
- **ko**: For building OCI containers

## License

MIT License
