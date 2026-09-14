$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$frontend = Join-Path $root "frontend"
$backend = Join-Path $root "backend"
$embed = Join-Path $backend "internal\web\dist"
$release = Join-Path $root "release"
$placeholder = Join-Path $embed "index.html"
$backup = Join-Path ([System.IO.Path]::GetTempPath()) ("frameflow-index-" + [guid]::NewGuid().ToString() + ".html")

Push-Location $frontend
try {
  if (-not (Test-Path (Join-Path $frontend "node_modules"))) {
    npm ci
    if ($LASTEXITCODE -ne 0) { throw "npm ci failed with exit code $LASTEXITCODE. Stop running dev servers and retry." }
  }
  npm run build
  if ($LASTEXITCODE -ne 0) { throw "npm run build failed with exit code $LASTEXITCODE." }
  New-Item -ItemType Directory -Force -Path $embed, $release | Out-Null
  Copy-Item $placeholder $backup -Force
  Get-ChildItem $embed -Force | Where-Object { $_.Name -ne "index.html" } | Remove-Item -Recurse -Force
  Copy-Item (Join-Path $frontend "dist\*") $embed -Recurse -Force
  Push-Location $backend
  try {
    go build -trimpath -ldflags "-H=windowsgui -s -w -X main.openOnStart=1" -o (Join-Path $release "FrameFlow.exe") ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE." }
  } finally { Pop-Location }
} finally {
  if (Test-Path $backup) { Copy-Item $backup $placeholder -Force; Remove-Item $backup -Force }
  Pop-Location
}
Write-Host "Created $release\FrameFlow.exe"
