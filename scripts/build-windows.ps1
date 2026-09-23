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
  $resolvedEmbed = [System.IO.Path]::GetFullPath($embed)
  $resolvedBackend = [System.IO.Path]::GetFullPath($backend).TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
  if (-not $resolvedEmbed.StartsWith($resolvedBackend, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Embedded frontend path is outside the backend workspace"
  }
  Copy-Item $placeholder $backup -Force
  Get-ChildItem $embed -Force | Where-Object { $_.Name -ne "index.html" } | Remove-Item -Recurse -Force
  Copy-Item (Join-Path $frontend "dist\*") $embed -Recurse -Force
  Push-Location $backend
  try {
    go build -trimpath -ldflags "-s -w -X main.openOnStart=1" -o (Join-Path $release "FrameFlow.exe") ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE." }
  } finally { Pop-Location }
} finally {
  if (Test-Path $backup) { Copy-Item $backup $placeholder -Force; Remove-Item $backup -Force }
  Pop-Location
}
$ffmpegSource = $env:FRAMEFLOW_FFMPEG_DIST_DIR
$downloadWork = $null
try {
  if (-not $ffmpegSource) {
    $downloadWork = Join-Path ([System.IO.Path]::GetTempPath()) ("frameflow-ffmpeg-" + [guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force -Path $downloadWork | Out-Null
    $archive = Join-Path $downloadWork "ffmpeg.zip"
    Invoke-WebRequest -Uri "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-8.1.2-essentials_build.zip" -OutFile $archive
    $expectedHash = "db580001caa24ac104c8cb856cd113a87b0a443f7bdf47d8c12b1d740584a2ec"
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) { throw "FFmpeg archive checksum mismatch" }
    Expand-Archive -LiteralPath $archive -DestinationPath $downloadWork -Force
    $ffmpegSource = (Get-ChildItem -LiteralPath $downloadWork -Filter ffmpeg.exe -Recurse -File | Select-Object -First 1).DirectoryName
  }
  if (-not $ffmpegSource) { throw "FFmpeg binaries were not found" }
  $ffmpegSource = (Resolve-Path -LiteralPath $ffmpegSource).Path
  $ffmpegExe = Join-Path $ffmpegSource "ffmpeg.exe"
  $ffprobeExe = Join-Path $ffmpegSource "ffprobe.exe"
  if (-not (Test-Path -LiteralPath $ffmpegExe) -or -not (Test-Path -LiteralPath $ffprobeExe)) {
    throw "FRAMEFLOW_FFMPEG_DIST_DIR must contain ffmpeg.exe and ffprobe.exe"
  }
  $binaryRelease = Join-Path $release "bin"
  New-Item -ItemType Directory -Force -Path $binaryRelease | Out-Null
  Copy-Item -LiteralPath $ffmpegExe -Destination (Join-Path $binaryRelease "ffmpeg.exe") -Force
  Copy-Item -LiteralPath $ffprobeExe -Destination (Join-Path $binaryRelease "ffprobe.exe") -Force
  @"
FFmpeg Windows binaries: Gyan Doshi FFmpeg 8.1.2 essentials build
Source: https://www.gyan.dev/ffmpeg/builds/
Archive SHA-256: db580001caa24ac104c8cb856cd113a87b0a443f7bdf47d8c12b1d740584a2ec
License: GPLv3. See https://ffmpeg.org/legal.html
"@ | Set-Content -LiteralPath (Join-Path $release "FFmpeg-NOTICE.txt") -Encoding utf8
} finally {
  if ($downloadWork) {
    $resolvedTemp = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
    $resolvedWork = [System.IO.Path]::GetFullPath($downloadWork)
    if ($resolvedWork.StartsWith($resolvedTemp, [System.StringComparison]::OrdinalIgnoreCase) -and
        $resolvedWork -ne $resolvedTemp -and (Test-Path -LiteralPath $resolvedWork)) {
      Remove-Item -LiteralPath $resolvedWork -Recurse -Force
    }
  }
}
Write-Host "Created $release\FrameFlow.exe with FFmpeg"
