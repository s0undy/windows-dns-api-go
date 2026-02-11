# Windows DNS API (Go)

A REST API in Go for managing DNS records on Microsoft Windows DNS Server. The API executes PowerShell cmdlets locally, authenticates via API keys, and uses YAML for configuration.

## Features

- **A Record Management**: Full CRUD operations for A records
- **API Key Authentication**: Secure access via `X-API-Key` header
- **PowerShell Integration**: Direct execution of DNS cmdlets
- **File Logging with Rotation**: Automatic log rotation (size + time-based), dual output (console + file)
- **Windows Service Support**: Native Windows service integration with SCM, auto-restart on failure
- **Interactive API Documentation**: Scalar-powered docs at `/docs` with "Try It Out" functionality
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
  file_path: ""                # Empty = default to exe directory + "windows-dns-api.log"
  max_size_mb: 100            # Max log file size before rotation (0 = no size-based rotation)
  rotate_days: 30             # Rotate every N days (0 = disabled). Default: 30
                               # Note: All rotated logs are kept permanently (never deleted)

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

### Automated Installation

Use the provided PowerShell script for one-command installation:

```powershell
# Basic installation (recommended)
.\scripts\install-service.ps1

# Custom installation path
.\scripts\install-service.ps1 -InstallPath "D:\Services\dns-api" -BinaryPath ".\bin\windows-dns-api-server.exe"
```

The script automatically:
- ✅ Creates installation directory at `C:\Program Files\windows-dns-api-go`
- ✅ Copies binary and configuration files
- ✅ Sets secure file permissions (Administrators + SYSTEM only)
- ✅ Creates Windows service with LocalSystem account
- ✅ Configures auto-restart on failure
- ✅ Starts the service and verifies it's running

**Uninstallation:**
```powershell
.\scripts\uninstall-service.ps1
```

See the **[Windows Service Installation Guide](docs/windows-service.md)** for:
- Detailed installation instructions
- Service management commands (start, stop, restart, status)
- Troubleshooting guide
- Security configuration options
- Manual installation steps

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
├── cmd/server/
│   ├── main.go                     # Entry point
│   ├── service_windows.go          # Windows service handler (Windows only)
│   └── service_other.go            # Service stub (non-Windows platforms)
├── internal/
│   ├── api/                        # HTTP handlers and routing
│   │   ├── handler.go              # Shared handler struct
│   │   ├── arecord_handler.go      # A record CRUD handlers
│   │   ├── health_handler.go       # Health check handler
│   │   ├── docs_handler.go         # Scalar API documentation handler
│   │   ├── openapi.yaml            # OpenAPI 3.1 specification (embedded)
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
├── scripts/
│   ├── install-service.ps1         # Automated service installation
│   └── uninstall-service.ps1       # Automated service removal
├── docs/
│   └── windows-service.md          # Windows service installation guide
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
