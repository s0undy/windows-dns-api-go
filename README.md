# Windows DNS API (Go)

A REST API in Go for managing DNS records on Microsoft Windows DNS Server. The API executes PowerShell cmdlets locally, authenticates via API keys, and uses YAML for configuration.

## Features

- **A Record Management**: Full CRUD operations for A records
- **API Key Authentication**: Secure access via `X-API-Key` header
- **PowerShell Integration**: Direct execution of DNS cmdlets
- **Structured Logging**: JSON or text format with configurable levels
- **Graceful Shutdown**: Safe termination with connection draining
- **Extensible Design**: Provider/Registry pattern for easy addition of new record types

## Requirements

- Go 1.22+ (for `net/http` method-based routing)
- Windows Server with DNS role installed
- PowerShell with DNS management cmdlets

## Installation

1. Clone the repository
2. Copy `config.yaml.example` to `config.yaml`
3. Edit `config.yaml` with your settings (zone, API keys, etc.)
4. Build the application:

```bash
make build
```

Or build for Windows specifically:

```bash
make build-windows
```

## Configuration

Create a `config.yaml` file based on the example:

```yaml
server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

dns:
  server_name: "."              # "." = local server
  default_zone: "example.com"

powershell:
  timeout: 30s
  executable: "powershell.exe"

logging:
  level: "info"                 # debug | info | warn | error
  format: "json"               # json | text

api_keys:
  - name: "admin"
    key: "your-secret-key-here"
```

## Running

```bash
# Run directly
make run

# Or run the compiled binary
./bin/windows-dns-api-server

# Or specify config path
CONFIG_PATH=/path/to/config.yaml ./bin/windows-dns-api-server
```

## Windows Service Installation

For production deployments, install the API as a Windows service that starts automatically.

See the **[Windows Service Installation Guide](docs/windows-service.md)** for detailed instructions including:
- Automated installation scripts
- Service management commands
- Troubleshooting steps
- Security configuration

Quick start:

```powershell
# 1. Copy files to installation directory
New-Item -Path "C:\Program Files\windows-dns-api-go" -ItemType Directory -Force
Copy-Item -Path ".\windows-dns-api-server.exe" -Destination "C:\Program Files\windows-dns-api-go\"
Copy-Item -Path ".\config.yaml" -Destination "C:\Program Files\windows-dns-api-go\"

# 2. Install and start service
sc.exe create WindowsDNSAPI binPath= "C:\Program Files\windows-dns-api-go\windows-dns-api-server.exe" start= auto DisplayName= "Windows DNS API"
sc.exe description WindowsDNSAPI "REST API for managing Windows DNS Server records"
sc.exe failure WindowsDNSAPI reset= 86400 actions= restart/5000/restart/5000/restart/5000
sc.exe start WindowsDNSAPI
```

## API Documentation

Interactive API documentation is available at `/docs` when the server is running:

```
http://localhost:8080/docs
```

The documentation is powered by [Scalar](https://scalar.com/) and provides:
- Interactive API explorer with "Try It Out" functionality
- Complete request/response examples
- Authentication testing (add your API key directly in the UI)
- Full schema documentation for all endpoints

## API Endpoints

### Health Check (No Authentication)

```bash
GET /api/v1/health
```

### A Record Operations (Require Authentication)

All endpoints require the `X-API-Key` header.

#### List all A records

```bash
curl -H "X-API-Key: your-secret-key-here" \
  "http://localhost:8080/api/v1/records/a?zone=example.com"
```

#### Get a specific A record

```bash
curl -H "X-API-Key: your-secret-key-here" \
  "http://localhost:8080/api/v1/records/a/www?zone=example.com"
```

#### Create an A record

```bash
curl -X POST \
  -H "X-API-Key: your-secret-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "www",
    "zone": "example.com",
    "ipv4_address": "192.168.1.100",
    "ttl": 3600
  }' \
  "http://localhost:8080/api/v1/records/a"
```

#### Update an A record

```bash
curl -X PUT \
  -H "X-API-Key: your-secret-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "zone": "example.com",
    "ipv4_address": "192.168.1.101",
    "ttl": 7200
  }' \
  "http://localhost:8080/api/v1/records/a/www"
```

#### Delete an A record

```bash
curl -X DELETE \
  -H "X-API-Key: your-secret-key-here" \
  "http://localhost:8080/api/v1/records/a/www?zone=example.com&value=192.168.1.101"
```

Note: The `value` parameter (IP address) is required for deletion because a hostname can have multiple A records.

## Development

### Build

```bash
make build
```

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Run Checks

```bash
make check  # Runs fmt, vet, and test
```

### Clean Build Artifacts

```bash
make clean
```

## Project Structure

```
windows-dns-api-go/
├── cmd/server/main.go              # Entry point
├── internal/
│   ├── api/                        # HTTP handlers and routing
│   │   ├── handler.go              # Shared handler struct
│   │   ├── arecord_handler.go      # A record CRUD handlers
│   │   ├── health_handler.go       # Health check handler
│   │   ├── request.go              # Request helpers
│   │   ├── response.go             # Response helpers
│   │   └── routes.go               # Route registration
│   ├── config/                     # Configuration loading
│   │   └── config.go
│   ├── dns/                        # DNS record management
│   │   ├── record.go               # Record types
│   │   ├── errors.go               # DNS errors
│   │   ├── provider.go             # Provider interfaces
│   │   ├── a_provider.go           # A record provider
│   │   └── registry.go             # Provider registry
│   ├── middleware/                 # HTTP middleware
│   │   ├── auth.go                 # Authentication
│   │   ├── logging.go              # Request logging
│   │   └── recover.go              # Panic recovery
│   ├── powershell/                 # PowerShell execution
│   │   └── executor.go
│   └── validate/                   # Input validation
│       └── validate.go
├── config.yaml.example
├── go.mod
└── Makefile
```

## Security Notes

- API keys should be kept secret and rotated regularly
- Run the API server with minimal required Windows privileges
- Consider using TLS/HTTPS in production (place behind a reverse proxy)
- Input validation prevents command injection attacks
- All user input is validated against strict patterns before PowerShell execution

## Adding New Record Types

To add support for new DNS record types (CNAME, AAAA, MX, TXT, etc.):

1. Define the record type in `internal/dns/record.go`
2. Create a new provider file (e.g., `internal/dns/cname_provider.go`)
3. Create handler file (e.g., `internal/api/cname_handler.go`)
4. Register the provider in `cmd/server/main.go`
5. Add routes in `internal/api/routes.go`

No changes to existing code are required!

## License

MIT
