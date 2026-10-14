$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$assetDir = Join-Path $root "backend/internal/infrastructure/mediaruntime/assets"
$work = Join-Path ([IO.Path]::GetTempPath()) ("frameflow-media-embed-" + [guid]::NewGuid().ToString())
$sourceDir = $env:FRAMEFLOW_FFMPEG_DIST_DIR
$sourceDescription = "User-supplied FRAMEFLOW_FFMPEG_DIST_DIR; binary hashes are recorded in manifest.json"
$expectedHash = "db580001caa24ac104c8cb856cd113a87b0a443f7bdf47d8c12b1d740584a2ec"
New-Item -ItemType Directory -Path $work | Out-Null
try {
  if (-not $sourceDir) {
    $download = Join-Path $work "ffmpeg.zip"
    $sourceUrl = "https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-8.1.2-essentials_build.zip"
    Invoke-WebRequest -Uri $sourceUrl -OutFile $download
    if ((Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expectedHash) {
      throw "FFmpeg archive checksum mismatch"
    }
    $extracted = Join-Path $work "download"
    Expand-Archive -LiteralPath $download -DestinationPath $extracted
    $sourceDir = (Get-ChildItem -LiteralPath $extracted -Filter ffmpeg.exe -Recurse -File | Select-Object -First 1).DirectoryName
    $sourceDescription = "$sourceUrl`nArchive SHA-256: $expectedHash"
  }
  if (-not $sourceDir) { throw "FFmpeg binaries were not found" }
  $sourceDir = (Resolve-Path -LiteralPath $sourceDir).Path
  $staging = Join-Path $work "payload"
  New-Item -ItemType Directory -Path $staging | Out-Null
  foreach ($name in @("ffmpeg.exe", "ffprobe.exe")) {
    $sourceFile = Join-Path $sourceDir $name
    if (-not (Test-Path -LiteralPath $sourceFile -PathType Leaf)) {
      throw "FRAMEFLOW_FFMPEG_DIST_DIR must contain ffmpeg.exe and ffprobe.exe"
    }
    Copy-Item -LiteralPath $sourceFile -Destination (Join-Path $staging $name)
  }
  Copy-Item -LiteralPath (Join-Path $root "backend/internal/infrastructure/mediaruntime/FFmpeg-LICENSE.txt") -Destination $staging
  @"
FFmpeg Windows binaries, embedded in FrameFlow.exe
Source: $sourceDescription
Upstream: https://www.gyan.dev/ffmpeg/builds/
License: GPLv3. See FFmpeg-LICENSE.txt and https://ffmpeg.org/legal.html
"@ | Set-Content -LiteralPath (Join-Path $staging "FFmpeg-NOTICE.txt") -Encoding utf8
  $hashes = [ordered]@{}
  Get-ChildItem -LiteralPath $staging -File | ForEach-Object {
    $hashes[$_.Name] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  }
  $manifestJSON = @{files=$hashes} | ConvertTo-Json -Depth 3
  [IO.File]::WriteAllText((Join-Path $staging "manifest.json"), $manifestJSON, [Text.UTF8Encoding]::new($false))
  $payload = Join-Path $work "media-tools.zip"
  Compress-Archive -LiteralPath (Get-ChildItem -LiteralPath $staging -File).FullName -DestinationPath $payload -CompressionLevel Optimal
  New-Item -ItemType Directory -Force -Path $assetDir | Out-Null
  Move-Item -LiteralPath $payload -Destination (Join-Path $assetDir "media-tools.zip") -Force
  Write-Host "Prepared embedded FFmpeg/ffprobe payload"
} finally {
  $resolvedWork = [IO.Path]::GetFullPath($work)
  $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
  if (-not $resolvedWork.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) { throw "Media staging path is outside temp directory" }
  if (Test-Path -LiteralPath $resolvedWork) { Remove-Item -LiteralPath $resolvedWork -Recurse -Force }
}
