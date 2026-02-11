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
