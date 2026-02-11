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
