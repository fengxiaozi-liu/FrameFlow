$ErrorActionPreference = "Stop"
$repository = "fengxiaozi-liu/FrameFlow"
$installRoot = Join-Path $env:LOCALAPPDATA "FrameFlow"
$apiUrl = "https://api.github.com/repos/$repository/releases/latest"
$headers = @{ "User-Agent" = "FrameFlow-Installer" }

$release = Invoke-RestMethod -Uri $apiUrl -Headers $headers
$asset = $release.assets | Where-Object { $_.name -eq "FrameFlow.exe" } | Select-Object -First 1
if ($null -eq $asset) {
  throw "The latest GitHub Release does not contain FrameFlow.exe."
}

New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
$temporary = Join-Path $installRoot ("FrameFlow-" + [guid]::NewGuid().ToString() + ".tmp")
$executable = Join-Path $installRoot "FrameFlow.exe"
try {
  Invoke-WebRequest -Uri $asset.browser_download_url -Headers $headers -OutFile $temporary
  Move-Item -LiteralPath $temporary -Destination $executable -Force
} finally {
  if (Test-Path $temporary) { Remove-Item -LiteralPath $temporary -Force }
}

Start-Process -FilePath $executable -WorkingDirectory $installRoot
Write-Host "FrameFlow $($release.tag_name) installed to $executable"
