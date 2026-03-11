param(
    [string]$RootPath = ""
)

$ErrorActionPreference = "Stop"

if (-not $RootPath) {
    $RootPath = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

$iconPath = Join-Path $RootPath "assets\app.ico"
$manifestPath = Join-Path $RootPath "assets\app.manifest"
$outPath = Join-Path $RootPath "cmd\server\rsrc_windows_amd64.syso"

if (-not (Test-Path $iconPath)) {
    throw "未找到图标文件: $iconPath"
}
if (-not (Test-Path $manifestPath)) {
    throw "未找到 manifest 文件: $manifestPath"
}

$goExe = (Get-Command go.exe -ErrorAction SilentlyContinue).Source
if (-not $goExe) {
    $fallbackGo = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $fallbackGo) {
        $goExe = $fallbackGo
    } else {
        throw "未找到 go.exe，请先安装 Go 或将其加入 PATH"
    }
}

$goPath = (& $goExe env GOPATH).Trim()
if (-not $goPath) {
    throw "无法获取 GOPATH"
}

$toolPath = Join-Path $goPath "bin\rsrc.exe"
if (-not (Test-Path $toolPath)) {
    & $goExe install github.com/akavel/rsrc@latest
    if ($LASTEXITCODE -ne 0) {
        throw "安装 rsrc 失败"
    }
}

& $toolPath -arch amd64 -ico $iconPath -manifest $manifestPath -o $outPath
if ($LASTEXITCODE -ne 0) {
    throw "生成 Windows 资源失败"
}

Write-Host "已生成 Windows 资源: $outPath"
