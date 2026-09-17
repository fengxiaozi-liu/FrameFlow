$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$backend = Join-Path $root "backend"
$frontend = Join-Path $root "frontend"
$previewDir = Join-Path ([IO.Path]::GetTempPath()) ("frameflow-preview-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $previewDir | Out-Null

$env:FRAMEFLOW_ADDRESS = "127.0.0.1:18081"
$env:FRAMEFLOW_DATABASE_PATH = Join-Path $previewDir "frameflow.db"
$env:FRAMEFLOW_VAULT_DIR = Join-Path $previewDir "secrets"
$env:FRAMEFLOW_UPLOAD_DIR = Join-Path $previewDir "uploads"
$env:VITE_PROXY_TARGET = "http://127.0.0.1:18081"

$api = Start-Process -FilePath "go" -ArgumentList "run", "./cmd/server" -WorkingDirectory $backend -WindowStyle Hidden -RedirectStandardOutput (Join-Path $previewDir "backend.out.log") -RedirectStandardError (Join-Path $previewDir "backend.err.log") -PassThru
$web = Start-Process -FilePath "npm.cmd" -ArgumentList "run", "dev", "--", "--host", "127.0.0.1", "--port", "15174" -WorkingDirectory $frontend -WindowStyle Hidden -RedirectStandardOutput (Join-Path $previewDir "frontend.out.log") -RedirectStandardError (Join-Path $previewDir "frontend.err.log") -PassThru
Write-Host "Preview: http://127.0.0.1:15174/tasks"
Write-Host "Backend PID: $($api.Id); frontend PID: $($web.Id); isolated data: $previewDir"
