# Implementation Plan: TLS Support with Windows Certificate Store

## Context

The Windows DNS API currently runs HTTP-only on port 8080. For production deployments, TLS/HTTPS support is needed but currently requires placing the API behind a reverse proxy (IIS, nginx, Caddy) for TLS termination.

This implementation adds **native TLS support** with certificate loading from the Windows Certificate Store, eliminating the need for external reverse proxies in many deployment scenarios. Certificates will be identified by either **thumbprint** or **serial number**, providing flexibility for automation and certificate management.

### Why This Approach

- **Windows-native**: Uses Windows Certificate Store APIs via `golang.org/x/sys/windows` (already a dependency)
- **Secure**: Private keys remain in Windows certificate store, never exposed to application code
- **Service-compatible**: Works with both console mode and Windows service mode
- **Backwards compatible**: TLS disabled by default, existing HTTP deployments continue working unchanged
- **No external dependencies**: Uses only standard library crypto/tls and existing golang.org/x/sys

## Implementation Approach

### 1. Configuration Structure

**Add TLS configuration to `config.yaml`:**

```yaml
server:
  address: "0.0.0.0"
  port: 8080          # HTTP port (used when TLS disabled)
  https_port: 8443    # HTTPS port (used when TLS enabled)
  read_timeout: 10s
  write_timeout: 10s

tls:
  enabled: false                    # Enable/disable TLS
  certificate_store: "LocalMachine" # "LocalMachine" or "CurrentUser"
  store_name: "MY"                  # "MY"=Personal, "Root", "CA"

  # Certificate identification (specify ONE):
  thumbprint: ""                    # SHA1 thumbprint (40 hex chars)
  serial_number: ""                 # Serial number (hex string)

  min_version: "1.2"                # "1.0", "1.1", "1.2", "1.3"
```

**Why this structure:**
- `https_port`: Separate HTTPS port (8443) from HTTP port (8080) for clarity
- `certificate_store`: LocalMachine for service mode (accessible to LocalSystem), CurrentUser for console mode
- `store_name`: Standard Windows certificate store names (MY for personal certificates)
- Thumbprint OR serial number: Two common certificate identification methods
- `min_version`: Security policy control (default TLS 1.2)

### 2. Certificate Loading Module

**Create new package: `internal/certs/`**

This package handles all Windows certificate store operations using platform-specific build tags (following the existing `service_windows.go`/`service_other.go` pattern).

**Key components:**
- `loader.go`: Platform-agnostic interfaces and public API
- `loader_windows.go` (build tag: `//go:build windows`): Windows certificate store syscalls
- `loader_other.go` (build tag: `//go:build !windows`): Stub for cross-platform compilation
- `errors.go`: Typed errors (ErrCertificateNotFound, ErrNoPrivateKey, etc.)

**Certificate loading process:**
1. Open Windows certificate store via `CertOpenStore()` syscall
2. Search by thumbprint using `CertFindCertificateInStore()` with `CERT_FIND_HASH`
3. OR search by serial number using `CERT_FIND_SERIAL_NUMBER`
4. Extract certificate DER bytes and parse to `x509.Certificate`
5. Acquire private key handle via `CryptAcquireCertificatePrivateKey()`
6. Return `tls.Certificate` with certificate chain and private key reference

**Private key handling:**
- Windows keeps private keys secure in certificate store
- Private key handle obtained via CNG (Cryptography Next Generation) APIs
- Private key never exported, only referenced for signing operations
- Implements `crypto.PrivateKey` interface for Go's crypto/tls

### 3. HTTP/HTTPS Server Integration

**Modify `cmd/server/main.go`:**

Current flow:
```
startHTTPServer() → httpServer.ListenAndServe() (HTTP only)
```

New flow:
```
startHTTPServer()
  ↓
  if cfg.TLS.Enabled:
    → startHTTPSServer() → LoadCertificate() → httpServer.ListenAndServeTLS()
  else:
    → httpServer.ListenAndServe() (existing HTTP code)
```

**Changes:**
- Add `startHTTPSServer()` function that loads certificate and configures TLS
- Modify `startHTTPServer()` to branch based on `cfg.TLS.Enabled`
- Extract shutdown logic to `shutdownServer()` helper (DRY)
- Add `parseTLSVersion()` helper to convert string ("1.2") to `tls.VersionTLS12`

**TLS Configuration:**
- Load certificate from Windows store
- Validate certificate for server use (ServerAuth EKU, DigitalSignature key usage)
- Configure `tls.Config` with secure cipher suites
- Set minimum TLS version from config
- Enable server cipher suite preference

### 4. Configuration Validation

**Extend `internal/config/config.go`:**

**Add structs:**
```go
type TLSConfig struct {
    Enabled          bool   `yaml:"enabled"`
    CertificateStore string `yaml:"certificate_store"`
    StoreName        string `yaml:"store_name"`
    Thumbprint       string `yaml:"thumbprint"`
    SerialNumber     string `yaml:"serial_number"`
    MinVersion       string `yaml:"min_version"`
}

type ServerConfig struct {
    // ... existing fields ...
    HTTPSPort int `yaml:"https_port"`  // NEW
}

type Config struct {
    // ... existing fields ...
    TLS TLSConfig `yaml:"tls"`  // NEW
}
```

**Validation rules:**
- If `TLS.Enabled == true`:
  - Must specify either `Thumbprint` OR `SerialNumber` (not both, not neither)
  - Thumbprint must be exactly 40 hex characters (SHA1)
  - SerialNumber must be valid hex string
  - CertificateStore must be "LocalMachine" or "CurrentUser"
  - StoreName must be valid: "MY", "Root", "CA", "Trust", "Disallowed"
  - MinVersion must be: "1.0", "1.1", "1.2", "1.3"
- Warning if `CertificateStore == "CurrentUser"` (won't work in service mode)

**Defaults (in `applyDefaults()`):**
- `HTTPSPort`: 8443
- `CertificateStore`: "LocalMachine"
- `StoreName`: "MY"
- `MinVersion`: "1.2"

### 5. Service Mode Compatibility

**Critical consideration:**
- Windows services run as LocalSystem account by default
- LocalSystem can access **LocalMachine** certificate store ✅
- LocalSystem **cannot** access **CurrentUser** certificate store ❌

**Implementation:**
- Default to LocalMachine store (safest for production)
- Log warning if CurrentUser store used (may fail in service mode)
- Certificate must have private key permissions for LocalSystem account
- Document in installation guide: use `certlm.msc` → Manage Private Keys → Add SYSTEM

### 6. Error Handling

**Startup errors (fatal - prevent server start):**
- Certificate not found in store
- Private key inaccessible (permissions)
- Certificate validation failed (no ServerAuth EKU, expired, etc.)
- Certificate store access denied

**Logging strategy:**
```
INFO: TLS configuration detected (store, store_name, identifier_type)
DEBUG: Opening certificate store
INFO: Certificate found (subject, issuer, expiry)
INFO: Private key acquired successfully
INFO: TLS server configured (min_version, cipher_suites)
INFO: Server listening (HTTPS) on address
```

**User-friendly error messages:**
- "Certificate with thumbprint XXX not found in LocalMachine\\MY store"
- "Failed to acquire private key: access denied. Grant LocalSystem account permission via certlm.msc"
- "Certificate validation failed: missing Server Authentication extended key usage"

## Critical Files to Modify

### New Files

1. **`internal/certs/loader.go`** (≈100 lines)
   - Interfaces: `Loader`, `CertificateIdentifier`
   - Public API: `LoadCertificate()`, `ValidateCertificate()`
   - Platform-agnostic certificate loading

2. **`internal/certs/loader_windows.go`** (≈300 lines)
   - Build tag: `//go:build windows`
   - Windows certificate store syscalls
   - Certificate search by thumbprint/serial
   - Private key acquisition via CNG

3. **`internal/certs/loader_other.go`** (≈20 lines)
   - Build tag: `//go:build !windows`
   - Stub implementation for cross-platform compilation

4. **`internal/certs/errors.go`** (≈30 lines)
   - Typed errors: `ErrCertificateNotFound`, `ErrNoPrivateKey`, etc.

5. **`docs/tls-testing-guide.md`** (≈200 lines)
   - Certificate setup (self-signed for testing)
   - Testing steps (HTTPS connection, TLS versions)
   - Troubleshooting guide
   - Common issues (certificate not found, private key access denied)

### Modified Files

1. **`internal/config/config.go`**
   - **Lines 10-16**: Add `TLSConfig` to `Config` struct
   - **Lines 18-23**: Add `HTTPSPort` to `ServerConfig`
   - **New**: Add `TLSConfig` struct definition (after line 23)
   - **Lines 73-109**: Update `applyDefaults()` - add TLS defaults
   - **Lines 112-154**: Update `Validate()` - add TLS validation
   - **New**: Add `isHexString()` helper function

2. **`cmd/server/main.go`**
   - **Line 194**: Modify to branch on `cfg.TLS.Enabled`
   - **New**: Add `startHTTPSServer()` function (after line 215)
   - **New**: Add `shutdownServer()` helper (extract lines 204-214)
   - **New**: Add `parseTLSVersion()` helper
   - **Top imports**: Add `"windows-dns-api-go/internal/certs"`, `"crypto/tls"`

3. **`config.yaml.example`**
   - **After line 17**: Add complete `tls:` section with documentation
   - **Line 10**: Add `https_port: 8443`

4. **`README.md`**
   - **After "Configuration" section**: Add "TLS Configuration (Optional)" section
   - **Features list**: Add "Native TLS Support" bullet
   - **Security Notes**: Update with TLS recommendations

5. **`docs/windows-service.md`**
   - **After "Prerequisites"**: Add note about LocalMachine certificates for TLS
   - **Installation section**: Add certificate permission setup steps

## Implementation Sequence

### Phase 1: Configuration (No TLS yet)
1. Add `TLSConfig` struct to `internal/config/config.go`
2. Add `HTTPSPort` to `ServerConfig`
3. Update `applyDefaults()` with TLS defaults
4. Update `Validate()` with TLS validation rules
5. Add `isHexString()` helper
6. Update `config.yaml.example`
7. **Test**: Load config with TLS section (enabled: false)

### Phase 2: Certificate Loading Module
1. Create `internal/certs/` directory
2. Implement `loader.go` (interfaces)
3. Implement `errors.go`
4. Implement `loader_windows.go` (Windows certificate store)
5. Implement `loader_other.go` (stub)
6. **Test**: Unit tests for config validation
7. **Test**: Certificate loading on Windows (requires test certificate)

### Phase 3: Server Integration
1. Add imports to `cmd/server/main.go`
2. Implement `startHTTPSServer()` function
3. Implement `shutdownServer()` helper
4. Implement `parseTLSVersion()` helper
5. Modify `startHTTPServer()` to branch on TLS enabled
6. **Test**: HTTP mode still works (TLS disabled)
7. **Test**: HTTPS mode with test certificate

### Phase 4: Documentation & Testing
1. Create `docs/tls-testing-guide.md`
2. Update `README.md` with TLS section
3. Update `docs/windows-service.md` with TLS notes
4. **Test**: Self-signed certificate setup
5. **Test**: Service mode with LocalMachine certificate
6. **Test**: Error scenarios (cert not found, wrong thumbprint, etc.)

## Verification Steps

### 1. Configuration Validation
```bash
# Test config validation with TLS disabled (should work)
# Test config with both thumbprint and serial (should error)
# Test config with invalid thumbprint length (should error)
# Test config with invalid store name (should error)
```

### 2. Certificate Loading (Windows only)
```powershell
# Create test certificate
$cert = New-SelfSignedCertificate `
    -Subject "CN=dns-api.local" `
    -DnsName "localhost" `
    -KeyAlgorithm RSA `
    -KeyLength 2048 `
    -CertStoreLocation "Cert:\LocalMachine\My" `
    -KeyUsage DigitalSignature, KeyEncipherment `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.1") `
    -NotAfter (Get-Date).AddYears(1)

# Get thumbprint
$cert.Thumbprint

# Update config.yaml with thumbprint
# Set tls.enabled = true
```

### 3. HTTPS Server Testing
```bash
# Start server - should see logs:
# - "Loading TLS certificate from Windows certificate store"
# - "Certificate found in store"
# - "TLS server configured"
# - "Server listening (HTTPS) on 0.0.0.0:8443"

# Test HTTPS connection
curl -k https://localhost:8443/api/v1/health

# Test TLS version (should succeed for 1.2)
openssl s_client -connect localhost:8443 -tls1_2

# Test TLS version (should fail if min_version is 1.2)
openssl s_client -connect localhost:8443 -tls1_1
```

### 4. Service Mode Testing
```powershell
# Ensure certificate is in LocalMachine\My store
# Grant LocalSystem permission to private key:
# - Open certlm.msc
# - Find certificate
# - Right-click → All Tasks → Manage Private Keys
# - Add "SYSTEM" account with Read permission

# Install service
.\scripts\install-service.ps1

# Start service
Start-Service WindowsDNSAPI

# Check service status
Get-Service WindowsDNSAPI

# Test HTTPS endpoint
curl -k https://localhost:8443/api/v1/health

# Check logs
Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" -Tail 20
```

### 5. Error Scenario Testing
```bash
# Test 1: Wrong thumbprint
# Expected: Startup fails with "certificate not found"

# Test 2: Certificate without private key
# Expected: Startup fails with "failed to acquire private key"

# Test 3: Certificate without ServerAuth EKU
# Expected: Startup fails with "certificate validation failed"

# Test 4: CurrentUser store in service mode
# Expected: Startup fails with "access denied" or "certificate not found"
```

### 6. Backwards Compatibility
```bash
# Test existing config.yaml without TLS section
# Expected: Server starts in HTTP mode on port 8080

# Test config.yaml with tls.enabled = false
# Expected: Server starts in HTTP mode on port 8080
```

## Key Design Decisions

### 1. Certificate Store: LocalMachine vs CurrentUser
**Decision**: Default to **LocalMachine**
- **Reason**: Compatible with Windows service mode (LocalSystem account)
- **Trade-off**: CurrentUser still supported for console mode development

### 2. Certificate Identification: Thumbprint vs Serial Number
**Decision**: Support **both** (user chooses one)
- **Thumbprint**: Most common, used by PowerShell, visible in certmgr.msc
- **Serial Number**: Alternative for automation scenarios
- **Why not Subject/Issuer**: Less precise, multiple certificates may match

### 3. Implementation: Direct Syscalls vs External Library
**Decision**: Direct **syscalls** via `golang.org/x/sys/windows`
- **Reason**: No new dependencies, maximum control, follows existing patterns
- **Trade-off**: More code to maintain vs using github.com/google/certtostore

### 4. HTTP/HTTPS Mode: Single Port vs Dual Port
**Decision**: **Single port** (HTTP OR HTTPS, not both)
- **Reason**: Simpler configuration and code
- **Trade-off**: Can't serve both HTTP and HTTPS simultaneously

### 5. TLS Disabled by Default
**Decision**: `enabled: false` by default
- **Reason**: Backwards compatibility, no breaking changes
- **Trade-off**: Users must explicitly enable TLS

## Dependencies

**No new dependencies required!**

Existing dependencies sufficient:
- `golang.org/x/sys v0.41.0` - Already in go.mod (used for Windows service)
- `crypto/tls` - Standard library
- `crypto/x509` - Standard library

## Security Considerations

1. **Private Key Security**: Private keys remain in Windows certificate store, never exported
2. **Certificate Validation**: Validates ServerAuth EKU and DigitalSignature key usage at startup
3. **TLS Version**: Defaults to TLS 1.2 minimum (industry standard)
4. **Cipher Suites**: Uses secure modern cipher suites (ECDHE-based, AEAD)
5. **Service Account**: LocalSystem has minimal permissions (by design)
6. **Certificate Permissions**: Document requirement to grant SYSTEM account read access to private key

## Potential Issues & Mitigations

| Issue | Mitigation |
|-------|------------|
| Certificate not accessible in service mode | Default to LocalMachine store, document private key permissions |
| Certificate expired | Validate at startup, log expiry date, fail fast with clear error |
| Private key permissions | Document in troubleshooting guide, provide clear error message |
| Multiple certificates with same serial | Use thumbprint (more precise) by default |
| Certificate without ServerAuth EKU | Validate at startup, fail with clear error message |

## Success Criteria

- ✅ Server starts with HTTPS when TLS enabled
- ✅ Certificate loaded from Windows certificate store by thumbprint
- ✅ Certificate loaded from Windows certificate store by serial number
- ✅ Works in both console mode and Windows service mode
- ✅ Backwards compatible (existing HTTP deployments unchanged)
- ✅ Clear error messages for certificate issues
- ✅ Complete documentation for setup and troubleshooting
- ✅ No new external dependencies added
