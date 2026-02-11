# Windows Service Installation Guide

This guide explains how to install the Windows DNS API as a Windows service that starts automatically with the system.

## Prerequisites

- Windows Server 2016 or later
- Administrator privileges
- Windows DNS API binary (`windows-dns-api-server.exe`)
- Valid `config.yaml` configuration file
- Windows DNS Server role installed

## Installation Directory Structure

The recommended installation directory is:
```
C:\Program Files\windows-dns-api-go\
├── windows-dns-api-server.exe
├── config.yaml
└── windows-dns-api.log (created automatically)
```

## Manual Installation Steps

### 1. Prepare Installation Directory

Open PowerShell as Administrator:

```powershell
New-Item -Path "C:\Program Files\windows-dns-api-go" -ItemType Directory -Force
```

### 2. Copy Files

Copy your compiled binary and configuration file:

```powershell
Copy-Item -Path ".\windows-dns-api-server.exe" -Destination "C:\Program Files\windows-dns-api-go\"
Copy-Item -Path ".\config.yaml" -Destination "C:\Program Files\windows-dns-api-go\"
```

### 3. Configure Logging

The application will automatically write logs to the installation directory. Edit `C:\Program Files\windows-dns-api-go\config.yaml` to verify logging configuration:

```yaml
logging:
  level: "info"
  format: "json"
  file_path: ""  # Empty = default to executable directory
  max_size_mb: 100
  rotate_days: 30
```

With `file_path: ""`, logs will automatically be written to:
```
C:\Program Files\windows-dns-api-go\windows-dns-api.log
```

### 4. Install Service

Create the Windows service using `sc.exe`:

```powershell
sc.exe create WindowsDNSAPI `
    binPath= "C:\Program Files\windows-dns-api-go\windows-dns-api-server.exe" `
    start= auto `
    DisplayName= "Windows DNS API" `
    obj= "LocalSystem"
```

### 5. Configure Service Description

```powershell
sc.exe description WindowsDNSAPI "REST API for managing Windows DNS Server records"
```

### 6. Configure Recovery Options

Configure the service to restart automatically on failure:

```powershell
sc.exe failure WindowsDNSAPI reset= 86400 actions= restart/5000/restart/5000/restart/5000
```

This configures the service to:
- Restart after 5 seconds on first failure
- Restart after 5 seconds on second failure
- Restart after 5 seconds on third failure
- Reset failure count after 24 hours (86400 seconds)

### 7. Start the Service

```powershell
sc.exe start WindowsDNSAPI
```

### 8. Verify Service is Running

```powershell
Get-Service -Name WindowsDNSAPI
```

You should see:
```
Status   Name               DisplayName
------   ----               -----------
Running  WindowsDNSAPI      Windows DNS API
```

## Automated Installation Script

Save this as `install-service.ps1` for automated installation:

```powershell
#Requires -RunAsAdministrator

<#
.SYNOPSIS
    Installs Windows DNS API as a Windows Service
.DESCRIPTION
    This script automates the installation of Windows DNS API as a Windows service,
    including file copying, permission setup, and service configuration.
.PARAMETER InstallPath
    Installation directory path (default: C:\Program Files\windows-dns-api-go)
.PARAMETER ServiceName
    Service name (default: WindowsDNSAPI)
.PARAMETER BinaryPath
    Path to the windows-dns-api-server.exe file
.PARAMETER ConfigPath
    Path to the config.yaml file
.EXAMPLE
    .\install-service.ps1
.EXAMPLE
    .\install-service.ps1 -InstallPath "D:\Services\dns-api" -BinaryPath ".\bin\windows-dns-api-server.exe"
#>

param(
    [string]$InstallPath = "C:\Program Files\windows-dns-api-go",
    [string]$ServiceName = "WindowsDNSAPI",
    [string]$BinaryPath = ".\windows-dns-api-server.exe",
    [string]$ConfigPath = ".\config.yaml"
)

# Function to write colored output
function Write-Step {
    param([string]$Message)
    Write-Host "[STEP] $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Failure {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Check if running as administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Failure "This script must be run as Administrator"
    exit 1
}

# Check if service already exists
Write-Step "Checking for existing service..."
$existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Failure "Service '$ServiceName' already exists. Please uninstall first using uninstall-service.ps1"
    exit 1
}
Write-Success "No existing service found"

# Check if binary exists
Write-Step "Checking for binary..."
if (-not (Test-Path $BinaryPath)) {
    Write-Failure "Binary not found at: $BinaryPath"
    exit 1
}
Write-Success "Binary found at: $BinaryPath"

# Check if config exists
Write-Step "Checking for configuration..."
if (-not (Test-Path $ConfigPath)) {
    Write-Failure "Config not found at: $ConfigPath"
    exit 1
}
Write-Success "Configuration found at: $ConfigPath"

# Create installation directory
Write-Step "Creating installation directory..."
New-Item -Path $InstallPath -ItemType Directory -Force | Out-Null
Write-Success "Installation directory created: $InstallPath"

# Copy files
Write-Step "Copying files..."
Copy-Item -Path $BinaryPath -Destination $InstallPath -Force
Copy-Item -Path $ConfigPath -Destination $InstallPath -Force
Write-Success "Files copied successfully"

# Set file permissions
Write-Step "Setting file permissions..."
$acl = Get-Acl $InstallPath
$acl.SetAccessRuleProtection($true, $false)

$adminRule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "BUILTIN\Administrators", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
)
$acl.SetAccessRule($adminRule)

$systemRule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT AUTHORITY\SYSTEM", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
)
$acl.SetAccessRule($systemRule)

Set-Acl $InstallPath $acl
Write-Success "File permissions set (Administrators and SYSTEM only)"

# Build full executable path
$executablePath = Join-Path $InstallPath (Split-Path $BinaryPath -Leaf)

# Install service
Write-Step "Installing Windows service..."
$result = sc.exe create $ServiceName `
    binPath= "`"$executablePath`"" `
    start= auto `
    DisplayName= "Windows DNS API" `
    obj= "LocalSystem"

if ($LASTEXITCODE -ne 0) {
    Write-Failure "Failed to create service: $result"
    exit 1
}
Write-Success "Service created"

# Set description
Write-Step "Setting service description..."
sc.exe description $ServiceName "REST API for managing Windows DNS Server records" | Out-Null
Write-Success "Service description set"

# Configure recovery
Write-Step "Configuring recovery options..."
sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null
Write-Success "Recovery options configured (auto-restart on failure)"

# Start service
Write-Step "Starting service..."
sc.exe start $ServiceName | Out-Null

Start-Sleep -Seconds 2

$service = Get-Service -Name $ServiceName
if ($service.Status -eq "Running") {
    Write-Success "Service started successfully"
} else {
    Write-Failure "Service failed to start. Status: $($service.Status)"
    Write-Host "Check logs at: $InstallPath\windows-dns-api.log" -ForegroundColor Yellow
    exit 1
}

# Display summary
Write-Host "`n========================================" -ForegroundColor Yellow
Write-Host "Installation Complete!" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Yellow
Write-Host "Service Name:    $ServiceName"
Write-Host "Install Path:    $InstallPath"
Write-Host "Status:          $($service.Status)"
Write-Host "Logs:            $InstallPath\windows-dns-api.log"
Write-Host "`nManagement Commands:"
Write-Host "  Status:   Get-Service -Name $ServiceName"
Write-Host "  Start:    Start-Service -Name $ServiceName"
Write-Host "  Stop:     Stop-Service -Name $ServiceName"
Write-Host "  Restart:  Restart-Service -Name $ServiceName"
Write-Host "`nAPI Documentation: http://localhost:8080/docs"
Write-Host "========================================`n" -ForegroundColor Yellow
```

### Running the Installation Script

```powershell
.\install-service.ps1
```

Or with custom parameters:

```powershell
.\install-service.ps1 -InstallPath "D:\Services\dns-api" -BinaryPath ".\bin\windows-dns-api-server.exe" -ConfigPath ".\config.yaml"
```

## Service Management

### Check Service Status

```powershell
Get-Service -Name WindowsDNSAPI
```

Or with detailed information:

```powershell
Get-Service -Name WindowsDNSAPI | Format-List *
```

Using sc.exe:

```powershell
sc.exe query WindowsDNSAPI
```

### Start Service

```powershell
Start-Service -Name WindowsDNSAPI
```

Or:

```powershell
sc.exe start WindowsDNSAPI
```

### Stop Service

```powershell
Stop-Service -Name WindowsDNSAPI
```

Or:

```powershell
sc.exe stop WindowsDNSAPI
```

### Restart Service

```powershell
Restart-Service -Name WindowsDNSAPI
```

Or:

```powershell
sc.exe stop WindowsDNSAPI
Start-Sleep -Seconds 2
sc.exe start WindowsDNSAPI
```

### View Service Logs

View the log file:

```powershell
Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" -Tail 50
```

For real-time log monitoring:

```powershell
Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" -Wait -Tail 50
```

View only errors:

```powershell
Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" | Select-String -Pattern "error"
```

## Uninstallation

### Uninstallation Script

Save this as `uninstall-service.ps1`:

```powershell
#Requires -RunAsAdministrator

<#
.SYNOPSIS
    Uninstalls Windows DNS API service
.DESCRIPTION
    Stops and removes the Windows DNS API service. Optionally removes installation files.
.PARAMETER ServiceName
    Service name (default: WindowsDNSAPI)
.PARAMETER RemoveFiles
    Remove installation files (default: false)
.PARAMETER InstallPath
    Installation directory path (default: C:\Program Files\windows-dns-api-go)
.EXAMPLE
    .\uninstall-service.ps1
.EXAMPLE
    .\uninstall-service.ps1 -RemoveFiles $true
#>

param(
    [string]$ServiceName = "WindowsDNSAPI",
    [bool]$RemoveFiles = $false,
    [string]$InstallPath = "C:\Program Files\windows-dns-api-go"
)

function Write-Step {
    param([string]$Message)
    Write-Host "[STEP] $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Failure {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Check if running as administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Failure "This script must be run as Administrator"
    exit 1
}

# Check if service exists
Write-Step "Checking for service..."
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $service) {
    Write-Failure "Service '$ServiceName' not found"
    exit 1
}
Write-Success "Service found"

# Stop service if running
if ($service.Status -eq "Running") {
    Write-Step "Stopping service..."
    sc.exe stop $ServiceName | Out-Null
    Start-Sleep -Seconds 3
    Write-Success "Service stopped"
}

# Delete service
Write-Step "Removing service..."
sc.exe delete $ServiceName | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Success "Service removed successfully"
} else {
    Write-Failure "Failed to remove service"
    exit 1
}

# Remove files if requested
if ($RemoveFiles) {
    Write-Step "Removing installation files..."
    if (Test-Path $InstallPath) {
        Remove-Item -Path $InstallPath -Recurse -Force
        Write-Success "Installation files removed from: $InstallPath"
    } else {
        Write-Host "Installation path not found: $InstallPath" -ForegroundColor Yellow
    }
}

Write-Host "`n========================================" -ForegroundColor Yellow
Write-Host "Uninstallation Complete!" -ForegroundColor Yellow
Write-Host "========================================`n" -ForegroundColor Yellow
```

### Manual Uninstallation

```powershell
# Stop the service
sc.exe stop WindowsDNSAPI

# Delete the service
sc.exe delete WindowsDNSAPI
```

### Remove Files (Optional)

**Warning**: This will delete all files including logs and configuration.

```powershell
Remove-Item -Path "C:\Program Files\windows-dns-api-go" -Recurse -Force
```

## Troubleshooting

### Service Won't Start

1. **Check if the executable exists:**
   ```powershell
   Test-Path "C:\Program Files\windows-dns-api-go\windows-dns-api-server.exe"
   ```

2. **Check if config file exists and is valid:**
   ```powershell
   Test-Path "C:\Program Files\windows-dns-api-go\config.yaml"
   Get-Content "C:\Program Files\windows-dns-api-go\config.yaml"
   ```

3. **Try running the executable manually to see error messages:**
   ```powershell
   cd "C:\Program Files\windows-dns-api-go"
   .\windows-dns-api-server.exe
   ```
   Press Ctrl+C to stop after verifying it starts.

4. **Check the application logs:**
   ```powershell
   Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" -Tail 100
   ```

5. **Check Windows Event Viewer:**
   - Open Event Viewer (`eventvwr.msc`)
   - Navigate to Windows Logs > Application
   - Look for errors from source "WindowsDNSAPI"

6. **Verify permissions:**
   ```powershell
   icacls "C:\Program Files\windows-dns-api-go"
   ```

### Service Crashes or Stops Unexpectedly

1. **Check application logs for errors:**
   ```powershell
   Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" | Select-String -Pattern "error","panic","fatal"
   ```

2. **Verify PowerShell access:**
   The service needs to execute PowerShell DNS cmdlets. Test PowerShell access:
   ```powershell
   Get-DnsServerResourceRecord -ZoneName "your-zone.com" -RRType A
   ```

3. **Check DNS Server status:**
   ```powershell
   Get-Service -Name DNS
   ```

4. **Review recovery actions:**
   ```powershell
   sc.exe qfailure WindowsDNSAPI
   ```

### Port Already in Use

If port 8080 is already in use:

1. **Stop the service:**
   ```powershell
   Stop-Service -Name WindowsDNSAPI
   ```

2. **Edit the configuration:**
   ```powershell
   notepad "C:\Program Files\windows-dns-api-go\config.yaml"
   ```
   Change the port:
   ```yaml
   server:
     port: 8081  # Change to available port
   ```

3. **Restart the service:**
   ```powershell
   Start-Service -Name WindowsDNSAPI
   ```

4. **Find what's using a port:**
   ```powershell
   Get-NetTCPConnection -LocalPort 8080 | Select-Object -Property LocalPort, OwningProcess
   Get-Process -Id <OwningProcess>
   ```

### Authentication Issues

1. **Verify API keys in config:**
   ```powershell
   Get-Content "C:\Program Files\windows-dns-api-go\config.yaml" | Select-String -Pattern "api_keys" -Context 0,5
   ```

2. **Test with curl:**
   ```powershell
   curl.exe -H "X-API-Key: your-secret-key-here" http://localhost:8080/api/v1/health
   ```

3. **Check authentication logs:**
   ```powershell
   Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" | Select-String -Pattern "auth"
   ```

### Log Files Not Created

1. **Check file permissions:**
   ```powershell
   icacls "C:\Program Files\windows-dns-api-go"
   ```

2. **Verify service account has write access:**
   The LocalSystem account should have full control by default.

3. **Check disk space:**
   ```powershell
   Get-PSDrive C | Select-Object Used,Free
   ```

## Security Considerations

### File Permissions

Restrict access to the installation directory to prevent unauthorized access:

```powershell
# Remove inherited permissions
$acl = Get-Acl "C:\Program Files\windows-dns-api-go"
$acl.SetAccessRuleProtection($true, $false)
Set-Acl "C:\Program Files\windows-dns-api-go" $acl

# Grant Administrators full control
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "BUILTIN\Administrators", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
)
$acl.SetAccessRule($rule)

# Grant SYSTEM full control
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT AUTHORITY\SYSTEM", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
)
$acl.SetAccessRule($rule)

# Apply permissions
Set-Acl "C:\Program Files\windows-dns-api-go" $acl
```

### Config File Security

The config file contains API keys and should be protected:

```powershell
$configPath = "C:\Program Files\windows-dns-api-go\config.yaml"
$acl = Get-Acl $configPath
$acl.SetAccessRuleProtection($true, $false)

# Only Administrators and SYSTEM can read
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "BUILTIN\Administrators", "FullControl", "Allow"
)
$acl.SetAccessRule($rule)

$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT AUTHORITY\SYSTEM", "FullControl", "Allow"
)
$acl.SetAccessRule($rule)

Set-Acl $configPath $acl
```

### Firewall Configuration

If accessing the API from other machines, configure Windows Firewall:

```powershell
New-NetFirewallRule -DisplayName "Windows DNS API" `
    -Direction Inbound `
    -Protocol TCP `
    -LocalPort 8080 `
    -Action Allow `
    -Profile Domain,Private `
    -Description "Allow inbound connections to Windows DNS API"
```

To restrict to specific IP addresses:

```powershell
New-NetFirewallRule -DisplayName "Windows DNS API (Restricted)" `
    -Direction Inbound `
    -Protocol TCP `
    -LocalPort 8080 `
    -Action Allow `
    -RemoteAddress 192.168.1.0/24 `
    -Profile Domain,Private
```

### Running with Dedicated Service Account (Advanced)

For better security isolation, create a dedicated service account:

```powershell
# Create user account
$password = ConvertTo-SecureString "ComplexPassword123!" -AsPlainText -Force
New-LocalUser -Name "DNSAPIService" `
    -Password $password `
    -Description "Windows DNS API Service Account" `
    -PasswordNeverExpires $true `
    -AccountNeverExpires

# Add to DNS Admins group (required for DNS management)
Add-LocalGroupMember -Group "DNS Admins" -Member "DNSAPIService"

# Grant "Log on as a service" right
# Note: This requires using Local Security Policy (secpol.msc) or Group Policy
# Navigate to: Local Policies > User Rights Assignment > Log on as a service

# Update service to use the account
sc.exe config WindowsDNSAPI obj= ".\DNSAPIService" password= "ComplexPassword123!"

# Restart service
Restart-Service -Name WindowsDNSAPI
```

### TLS/HTTPS Configuration

For production use, consider placing the API behind a reverse proxy (IIS, nginx, Caddy) that handles TLS termination:

- **IIS**: Use Application Request Routing (ARR) module
- **nginx**: Standard reverse proxy configuration
- **Caddy**: Automatic HTTPS with Let's Encrypt

### API Key Best Practices

1. **Use strong, random API keys** (at least 32 characters)
2. **Rotate keys regularly** (every 90 days recommended)
3. **Use different keys for different clients/applications**
4. **Never commit API keys to version control**
5. **Monitor API access logs** for suspicious activity

Generate secure API keys:

```powershell
# Generate a random 32-byte API key
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$apiKey = [Convert]::ToBase64String($bytes)
Write-Host "New API Key: $apiKey"
```

## Updating the Service

To update to a new version:

1. **Stop the service:**
   ```powershell
   Stop-Service -Name WindowsDNSAPI
   ```

2. **Backup current installation:**
   ```powershell
   Copy-Item -Path "C:\Program Files\windows-dns-api-go" `
             -Destination "C:\Program Files\windows-dns-api-go-backup-$(Get-Date -Format 'yyyy-MM-dd')" `
             -Recurse
   ```

3. **Replace the binary:**
   ```powershell
   Copy-Item -Path ".\windows-dns-api-server.exe" `
             -Destination "C:\Program Files\windows-dns-api-go\" `
             -Force
   ```

4. **Review config changes** (if any in new version):
   ```powershell
   # Compare with example config from new version
   notepad "C:\Program Files\windows-dns-api-go\config.yaml"
   ```

5. **Start the service:**
   ```powershell
   Start-Service -Name WindowsDNSAPI
   ```

6. **Verify the update:**
   ```powershell
   Get-Service -Name WindowsDNSAPI
   Get-Content "C:\Program Files\windows-dns-api-go\windows-dns-api.log" -Tail 20
   ```

## Monitoring and Maintenance

### Regular Health Checks

Create a scheduled task to monitor the API:

```powershell
# Create monitoring script
$monitorScript = @"
`$response = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/health' -UseBasicParsing -ErrorAction SilentlyContinue
if (`$response.StatusCode -ne 200) {
    Write-EventLog -LogName Application -Source 'WindowsDNSAPI' -EventId 1001 -EntryType Error -Message 'Health check failed'
}
"@

Set-Content -Path "C:\Program Files\windows-dns-api-go\monitor.ps1" -Value $monitorScript

# Create scheduled task
$action = New-ScheduledTaskAction -Execute 'PowerShell.exe' -Argument '-File "C:\Program Files\windows-dns-api-go\monitor.ps1"'
$trigger = New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 5)
Register-ScheduledTask -TaskName "WindowsDNSAPI-HealthCheck" -Action $action -Trigger $trigger -RunLevel Highest
```

### Log Rotation Monitoring

Check log file sizes:

```powershell
Get-ChildItem "C:\Program Files\windows-dns-api-go\windows-dns-api*.log" |
    Select-Object Name, Length, LastWriteTime |
    Format-Table -AutoSize
```

### Performance Monitoring

Monitor service resource usage:

```powershell
$process = Get-Process | Where-Object {$_.ProcessName -eq "windows-dns-api-server"}
$process | Select-Object ProcessName, CPU, WorkingSet, Handles
```

## References

- [Windows Service Control (sc.exe)](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/sc-create)
- [Windows Services Best Practices](https://learn.microsoft.com/en-us/dotnet/framework/windows-services/introduction-to-windows-service-applications)
- [PowerShell DNS Cmdlets](https://learn.microsoft.com/en-us/powershell/module/dnsserver/)
