param (
    [string]$Action = "all",
    [string]$RootPath = "$PSScriptRoot",
    [string]$ParentPath = ""
)

$ErrorActionPreference = "Stop"

if (-not $RootPath) {
    $RootPath = (Get-Item .).FullName
}

if (-not $ParentPath) {
    $ParentPath = (Get-Item (Join-Path $RootPath "..")).FullName
}

# Root and output
$BinPath = Join-Path $RootPath "bin"
$ExePath = Join-Path $BinPath "OpenSysKit.exe"

# Parent path files
$CertPath = Join-Path $ParentPath "WinDriverLoader.pfx"
$CertPassPath = Join-Path $ParentPath "password.txt"

function Get-SignTool {
    $paths = @(
        "C:\Program Files (x86)\Windows Kits\10\bin\10.0.*.0\x64\signtool.exe",
        "C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe",
        "C:\Program Files (x86)\Windows Kits\8.1\bin\x64\signtool.exe"
    )
    foreach ($p in $paths) {
        $resolved = Get-ChildItem $p -ErrorAction SilentlyContinue | Select-Object -Last 1
        if ($resolved) {
            return $resolved.FullName
        }
    }
    return $null
}

function Build {
    if (-not (Test-Path $BinPath)) {
        New-Item -ItemType Directory -Force -Path $BinPath | Out-Null
    }

    # 提取版本信息
    $tag = git describe --tags --always --dirty 2>$null
    if (-not $tag) { $tag = "dev" }
    $time = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

    $ldflags = "-s -w -X 'main.version=$tag' -X 'main.buildTime=$time'"

    Write-Host ">>> Compiling Go backend ($tag)..." -ForegroundColor Cyan
    go build -ldflags $ldflags -trimpath -o $ExePath ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Build success: $ExePath" -ForegroundColor Green
}

function Sign {
    if (-not (Test-Path $ExePath)) {
        Write-Host "Error: Cannot find $ExePath, build first." -ForegroundColor Red
        exit 1
    }

    if (-not (Test-Path $CertPath)) {
        Write-Host "Skip signing: Certificate not found ($CertPath)" -ForegroundColor Yellow
        return
    }

    $signtool = Get-SignTool
    if (-not $signtool) {
        Write-Host "Warning: signtool.exe not found, skipping sign" -ForegroundColor Yellow
        return
    }

    $pass = Get-Content $CertPassPath -Raw
    $pass = $pass.Trim()

    Write-Host ">>> Signing $ExePath..." -ForegroundColor Cyan
    & $signtool sign /fd sha256 /f $CertPath /p $pass $ExePath

    if ($LASTEXITCODE -eq 0) {
        Write-Host "Sign success!" -ForegroundColor Green
    } else {
        Write-Host "Sign failed, code: $LASTEXITCODE" -ForegroundColor Red
    }
}

switch ($Action) {
    "build" {
        Build
    }
    "sign" {
        Sign
    }
    "all" {
        Build
        Sign
    }
    "clean" {
        if (Test-Path $BinPath) {
            Remove-Item -Recurse -Force $BinPath
            Write-Host "Cleaned $BinPath" -ForegroundColor Green
        }
    }
    default {
        Write-Host "Unknown action: $Action. Available actions: build, sign, all, clean" -ForegroundColor Red
    }
}
