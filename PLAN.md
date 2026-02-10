# Windows DNS API (Go) — Implementation Plan

## Context

Build a REST API in Go that manages DNS records on a Microsoft Windows DNS Server. The API runs directly on the DNS server, executes PowerShell cmdlets locally via `os/exec`, authenticates via API keys, and uses YAML for configuration. No database needed — DNS data lives in Windows DNS, API keys live in the config file.

Starting with A records, but architected for easy addition of CNAME, AAAA, MX, TXT, etc.

---

## Project Structure

```
windows-dns-api-go/
├── cmd/server/main.go              # Entry point, wiring, graceful shutdown
├── internal/
│   ├── api/
│   │   ├── handler.go              # Shared handler struct (registry, config, logger)
│   │   ├── arecord_handler.go      # A record CRUD handlers
│   │   ├── health_handler.go       # GET /api/v1/health (no auth)
│   │   ├── request.go              # Request body types + JSON decoder
│   │   ├── response.go             # JSON response envelope + helpers
│   │   └── routes.go               # Route registration on mux
│   ├── config/
│   │   └── config.go               # YAML config struct + Load()
│   ├── dns/
│   │   ├── record.go               # Record interface, BaseRecord, ARecord
│   │   ├── errors.go               # Sentinel errors (NotFound, Exists, Validation)
│   │   ├── provider.go             # RecordProvider + CommandExecutor interfaces
│   │   ├── a_provider.go           # A record provider (PowerShell CRUD)
│   │   └── registry.go             # Maps RecordType → RecordProvider
│   ├── middleware/
│   │   ├── auth.go                 # X-API-Key header validation
│   │   ├── logging.go              # Request/response logging
│   │   └── recover.go              # Panic recovery
│   ├── powershell/
│   │   └── executor.go             # PowerShell execution via os/exec
│   └── validate/
│       └── validate.go             # Hostname, IPv4, zone, TTL validation
├── config.yaml.example
├── go.mod                          # Only external dep: gopkg.in/yaml.v3
└── Makefile
```

## Dependencies

- **Go 1.22+** (for `net/http` method-based routing with `r.PathValue()`)
- **gopkg.in/yaml.v3** — config parsing
- **Standard library only** for everything else: `log/slog`, `net/http`, `os/exec`, `encoding/json`

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/health` | No | Health check |
| GET | `/api/v1/records/a?zone=X` | Yes | List all A records |
| GET | `/api/v1/records/a/{name}?zone=X` | Yes | Get A record by name |
| POST | `/api/v1/records/a` | Yes | Create A record |
| PUT | `/api/v1/records/a/{name}` | Yes | Update A record |
| DELETE | `/api/v1/records/a/{name}?zone=X&value=IP` | Yes | Delete A record |

- Auth: `X-API-Key` header with key from config
- Zone defaults to `config.dns.default_zone` if omitted
- DELETE requires `value` query param (IP) because a hostname can have multiple A records

## Key Design Decisions

### PowerShell Interaction
- Execute locally via `os/exec` with `-NoProfile -NonInteractive -Command`
- Use `context.WithTimeout` per command (configurable, default 30s)
- Flatten `RecordData` via `Select-Object` calculated properties before `ConvertTo-Json` to avoid CIM metadata bloat
- Handle PowerShell's single-object-vs-array quirk (one result = JSON object, multiple = array)
- **Update strategy**: Remove old + Add new (simpler than `Set-DnsServerResourceRecord` which requires CIM object cloning)

### Extensibility (Provider/Registry Pattern)
- `RecordProvider` interface defines CRUD contract per record type
- `Registry` maps `RecordType → RecordProvider`
- Adding a new record type = new provider file + new handler file + register in `main.go` + add routes. No existing code changes.

### Input Validation (Command Injection Prevention)
- All user input validated against strict patterns before interpolation into PS commands
- Hostname: RFC 952/1123 regex, max 63 chars
- IPv4: `net.ParseIP` + `.To4()` check
- Zone: split on dots, validate each label
- TTL: 0–604800 range

### Authentication
- API key via `X-API-Key` header
- Keys defined in `config.yaml` with a name (for audit logging)
- Key lookup via pre-built `map[string]struct{}` for O(1) checks
- Auth middleware applied per-route (health endpoint excluded)

### Middleware Stack Order
`Recover → Logging → (Auth per route) → Handler`

### Graceful Shutdown
- Listen for SIGINT/SIGTERM, call `server.Shutdown()` with 15s timeout

## Config Format (config.yaml.example)

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

## Implementation Order

### Phase 1: Foundation
1. `go.mod` — init module, add yaml.v3
2. `internal/config/config.go` — config loading, defaults, validation
3. `config.yaml.example`
4. `internal/validate/validate.go` — input validation

### Phase 2: PowerShell + DNS Layer
5. `internal/powershell/executor.go` — PS execution engine
6. `internal/dns/record.go` — Record interface, BaseRecord, ARecord
7. `internal/dns/errors.go` — sentinel errors
8. `internal/dns/provider.go` — RecordProvider + CommandExecutor interfaces
9. `internal/dns/a_provider.go` — A record CRUD
10. `internal/dns/registry.go` — provider registry

### Phase 3: HTTP Layer
11. `internal/api/response.go` + `internal/api/request.go` — JSON helpers
12. `internal/middleware/auth.go` — API key auth
13. `internal/middleware/logging.go` — request logging
14. `internal/middleware/recover.go` — panic recovery
15. `internal/api/handler.go` — shared handler struct
16. `internal/api/health_handler.go` — health endpoint
17. `internal/api/arecord_handler.go` — A record handlers
18. `internal/api/routes.go` — route registration

### Phase 4: Wiring + Polish
19. `cmd/server/main.go` — entry point, dependency wiring
20. `Makefile` — build/test/run targets

## Verification

1. **Build**: `go build ./cmd/server/` compiles without errors
2. **Config loading**: Run with example config, verify startup log output
3. **Health check**: `curl http://localhost:8080/api/v1/health` returns `{"data":{"status":"ok"}}`
4. **Auth rejection**: `curl http://localhost:8080/api/v1/records/a` without key returns 401
5. **Auth success**: Same request with `-H "X-API-Key: ..."` returns 200
6. **CRUD on Windows DNS Server**: Test create/read/update/delete A records against a real Windows DNS zone
