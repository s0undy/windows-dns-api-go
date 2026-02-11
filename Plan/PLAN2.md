# Implementation Plan: File Logging, API Documentation, and Windows Service

## Context

The Windows DNS API is a REST API in Go that manages DNS records on Microsoft Windows DNS Server. It currently logs to stdout only, has no interactive API documentation, and lacks production deployment documentation for Windows Service installation.

This plan adds three production-ready features:
1. **File-based logging** with automatic rotation alongside console output
2. **Scalar API documentation** served at `/docs` with embedded OpenAPI spec
3. **Windows Service installation documentation** with automated scripts

## Feature 1: File Logging with Rotation

### Overview
Add dual output logging (console + file) using `io.MultiWriter` and automatic log rotation using the industry-standard `lumberjack` library. Default log location: same directory as executable.

### Files to Modify

**[internal/config/config.go](internal/config/config.go)** - Add log file configuration
- Extend `LoggingConfig` struct (lines 36-39) with:
  - `FilePath string` - Path to log file (empty = default to exe directory)
  - `MaxSize int` - Max size in MB before rotation (default: 100)
  - `RotateDays int` - Rotate every N days (default: 30, 0 = disabled)
- Update `applyDefaults()` function (lines 95-100) to set defaults for new fields:
  - `MaxSize`: 100
  - `RotateDays`: 30
- No validation changes needed (file path can be empty for default)

**[cmd/server/main.go](cmd/server/main.go)** - Implement file logging
- Add imports: `io`, `path/filepath`, `gopkg.in/natefinch/lumberjack.v2`
- Modify `setupLogger()` function (lines 100-127):
  - Determine log file path using `os.Executable()` if config is empty
  - Create `lumberjack.Logger` instance with rotation settings:
    - `MaxSize`: From config (default 100MB) - rotates if file exceeds this size
    - `MaxAge`: 0 (never delete old logs)
    - `MaxBackups`: 0 (keep all rotated log files)
    - `LocalTime`: true (use local time for backup filenames)
  - Use `io.MultiWriter(os.Stdout, logFile)` for dual output
  - Pass multiwriter to existing `slog.NewJSONHandler()` or `slog.NewTextHandler()`
  - Log the final log file path for confirmation
- Add `startLogRotationTimer()` function after `setupLogger()`:
  - Takes `*lumberjack.Logger` and rotation interval (days) as parameters
  - Starts a goroutine with `time.NewTicker(interval * 24 * time.Hour)`
  - On each tick, calls `logFile.Rotate()` to force rotation
  - Runs until program exits (no cleanup needed - goroutine dies with main)
- In `main()` function after logger setup:
  - Call `startLogRotationTimer(logFile, cfg.Logging.RotateDays)` if `RotateDays > 0`

**[config.yaml.example](config.yaml.example)** - Document new options
- Add to logging section (lines 15-17):
  ```yaml
  logging:
    level: "info"
    format: "json"
    file_path: ""           # Empty = default to exe directory + "windows-dns-api.log"
    max_size_mb: 100       # Max log file size before rotation (0 = no size-based rotation)
    rotate_days: 30        # Rotate every N days (0 = disabled, default: 30)
  ```

  **Note**: All rotated log files are kept permanently (never deleted). Rotation happens when:
  - File size exceeds `max_size_mb` (if > 0), OR
  - Every `rotate_days` days (if > 0)

  Default: Rotates every 30 days OR when file reaches 100MB (whichever comes first)

**[go.mod](go.mod)** - Add lumberjack dependency
- Add: `gopkg.in/natefinch/lumberjack.v2 v2.2.1`
- Run `go mod tidy` after changes

### Default Behavior
- If `file_path` is empty or not specified: logs to `<exe-directory>/windows-dns-api.log`
- Falls back to current directory if `os.Executable()` fails
- Both console and file receive identical log output in same format
- Automatic rotation triggered by:
  - **Size**: When file reaches 100MB (configurable via `max_size_mb`)
  - **Time**: Every 30 days (configurable via `rotate_days`)
- **All rotated log files are kept permanently** - never deleted
- Rotated files are timestamped: `windows-dns-api-2026-02-11T15-04-05.123.log`
- Time-based rotation runs in background goroutine using `time.Ticker`

### Verification Steps
1. Build with `make build-windows`
2. Run without `file_path` config - verify log file created next to exe
3. Make API requests - verify logs appear in both console and file
4. Verify log format matches between console and file
5. Test with explicit `file_path` in config
6. Generate large logs to test size-based rotation (>100MB) - verify rotation occurs
7. Verify rotated files are kept (not deleted)
8. Test time-based rotation by setting `rotate_days: 0.001` (~1.4 minutes) for testing
9. Confirm rotation occurs after the interval
10. Verify all old log files remain accessible with timestamps in filenames

---

## Feature 2: Scalar API Documentation

### Overview
Embed an OpenAPI 3.1 specification and serve interactive Scalar documentation at `/docs` (no authentication required). Uses Go's `embed` package to bundle the spec in the binary.

### Files to Create

**[internal/api/openapi.yaml](internal/api/openapi.yaml)** - OpenAPI 3.1 specification
- Complete spec covering all 6 endpoints:
  - `GET /api/v1/health` (no auth)
  - `GET /api/v1/records/a` (list)
  - `GET /api/v1/records/a/{name}` (get)
  - `POST /api/v1/records/a` (create)
  - `PUT /api/v1/records/a/{name}` (update)
  - `DELETE /api/v1/records/a/{name}` (delete)
- Define schemas: `ARecord`, `CreateARecordRequest`, `UpdateARecordRequest`, `ErrorResponse`
- Document authentication: `X-API-Key` header using `apiKey` security scheme
- Include examples for all request/response bodies
- Document all query parameters (zone, value)
- Specify error codes: `bad_request`, `unauthorized`, `not_found`, `conflict`, `internal_error`

**[internal/api/docs_handler.go](internal/api/docs_handler.go)** - Scalar documentation handler
- Add `//go:embed openapi.yaml` directive to embed spec
- Implement `DocsHandler` method on `Handler` struct
- Serve HTML page with Scalar CDN script
- Inject OpenAPI spec into Scalar configuration
- Use existing error handling pattern (`WriteInternalError`)

### Files to Modify

**[internal/api/routes.go](internal/api/routes.go)** - Register docs endpoint
- Add after health endpoint (line ~15):
  ```go
  mux.HandleFunc("GET /docs", h.DocsHandler)
  ```
- No authentication required (public documentation)

**[README.md](README.md)** - Document the docs endpoint
- Add section after "API Endpoints" (line ~78):
  - Link to `/docs` endpoint
  - Mention Scalar features (interactive explorer, auth testing)

### Technical Approach
- **Embedding**: OpenAPI spec embedded at compile time via `//go:embed`
- **Scalar rendering**: Served via CDN (https://cdn.jsdelivr.net/npm/@scalar/api-reference)
- **Configuration**: Spec content injected as JSON into Scalar's configuration object
- **Authentication**: Scalar's "Try It Out" supports adding `X-API-Key` header

### Verification Steps
1. Build project with embedded spec
2. Navigate to `http://localhost:8080/docs`
3. Verify Scalar UI renders all 6 endpoints
4. Test "Try It Out" with valid API key - should succeed
5. Test without API key - should receive 401 for protected endpoints
6. Verify all request/response schemas display correctly
7. Test health endpoint (no auth) via Scalar

---

## Feature 3: Windows Service Installation Documentation

### Overview
Create comprehensive documentation for installing the API as a Windows service at `C:\Program Files\windows-dns-api-go` with config and logs co-located. Uses PowerShell and native `sc.exe` with automated installation scripts.

### Files to Create

**[docs/windows-service.md](docs/windows-service.md)** - Complete installation guide

**Content structure:**
1. **Prerequisites**: Windows Server 2016+, admin privileges, DNS role
2. **Directory structure**: `C:\Program Files\windows-dns-api-go` with exe, config, logs
3. **Manual Installation Steps**:
   - Create installation directory
   - Copy binary and config files
   - Install service using sc.exe
   - Configure service recovery options
   - Start the service
4. **Automated Installation**: PowerShell script `install-service.ps1`
   - Parameter-driven (install path, service name, binary/config paths)
   - Validation (admin check, file existence, no existing service)
   - File copying and permission setting
   - Service installation using sc.exe
   - Recovery configuration (restart on failure)
   - Service startup
   - Success summary with management commands
5. **Service management**: Start, stop, restart, status commands
6. **Log viewing**: PowerShell commands to tail logs
7. **Uninstallation**: Stop and remove service with sc.exe
8. **Troubleshooting**:
   - Service won't start (permissions, missing files, port conflicts)
   - Service crashes (log inspection, DNS permissions)
   - Authentication issues
9. **Security considerations**:
   - File permissions (restrict to Administrators + SYSTEM)
   - Config file security (contains API keys)
   - Firewall configuration
   - Dedicated service account setup (optional)

**Key installation script features:**
- Service name: `WindowsDNSAPI`
- Display name: `Windows DNS API`
- Auto-start configuration (SERVICE_AUTO_START)
- Restart on failure (5 second delay, 3 retry attempts)
- Runs as LocalSystem (or optional dedicated account)
- Uses sc.exe for service creation and configuration

### Files to Modify

**[README.md](README.md)** - Link to service documentation
- Add section after "Running" (line ~77):
  - Brief overview of Windows service deployment
  - Link to detailed guide: `docs/windows-service.md`
  - Quick start example using `sc.exe`

### Configuration Notes
With `file_path: ""` in config, logs automatically write to:
```
C:\Program Files\windows-dns-api-go\windows-dns-api.log
```

This matches the required installation structure without config changes.

### Verification Steps
1. Test `install-service.ps1` on Windows Server
2. Verify service starts automatically after server reboot
3. Kill service process and verify auto-restart (recovery action)
4. Test uninstallation procedure
5. Verify logs write to correct location (`C:\Program Files\windows-dns-api-go\windows-dns-api.log`)
6. Validate PowerShell script runs without errors as Administrator
7. Test with non-admin account (should fail appropriately with clear error)
8. Verify service management commands (start, stop, restart, status)

---

## Implementation Order

### Phase 1: File Logging
1. Modify `internal/config/config.go` - add log file fields
2. Update `config.yaml.example` - document new options
3. Add lumberjack to `go.mod` and run `go mod tidy`
4. Modify `cmd/server/main.go` - implement dual logging
5. Build and test locally

### Phase 2: API Documentation
6. Create `internal/api/openapi.yaml` - full API specification
7. Create `internal/api/docs_handler.go` - Scalar handler
8. Modify `internal/api/routes.go` - register `/docs` endpoint
9. Update `README.md` - document API docs
10. Build and test documentation UI

### Phase 3: Service Documentation
11. Create `docs/` directory
12. Create `docs/windows-service.md` - complete guide with scripts
13. Update `README.md` - link to service docs

### Phase 4: Final Testing
14. Build Windows binary: `make build-windows`
15. Test file logging with default and explicit paths
16. Test Scalar docs UI and "Try It Out" functionality
17. Test service installation scripts on Windows Server (if available)

---

## Critical Files Reference

**Logging implementation:**
- [cmd/server/main.go:100-127](cmd/server/main.go) - `setupLogger()` function to modify
- [internal/config/config.go:36-39](internal/config/config.go) - `LoggingConfig` struct
- [internal/config/config.go:95-100](internal/config/config.go) - `applyDefaults()` function

**API structure (for OpenAPI spec):**
- [internal/api/routes.go](internal/api/routes.go) - All route definitions
- [internal/api/request.go](internal/api/request.go) - Request body structures
- [internal/api/response.go](internal/api/response.go) - Response helpers and error codes
- [internal/api/arecord_handler.go](internal/api/arecord_handler.go) - Handler logic
- [internal/dns/record.go](internal/dns/record.go) - `ARecord` struct definition

**Existing patterns to follow:**
- Handler method pattern: See `health_handler.go` for simple handler structure
- Error response pattern: Use `WriteError()`, `WriteInternalError()` from `response.go`
- Route registration: Follow pattern in `routes.go` with/without auth middleware

---

## Dependencies

**New dependency:**
- `gopkg.in/natefinch/lumberjack.v2` v2.2.1 - Log rotation

**External resources (not dependencies):**
- Scalar API Reference CDN: `https://cdn.jsdelivr.net/npm/@scalar/api-reference`
- NSSM: `https://nssm.cc/download` (user downloads separately)

---

## Notes

- **No breaking changes**: All features are additive
- **Backward compatible**: Existing configs continue to work (new fields are optional)
- **Minimal external dependencies**: Only lumberjack added; Scalar loaded via CDN
- **Cross-platform safe**: File logging works on both Windows and Linux (development)
- **Security conscious**: Service documentation includes permission hardening steps
- **Production ready**: Log rotation prevents disk space issues; service auto-recovery handles crashes
