param(
  [string]$Executable = (Join-Path $PSScriptRoot "../release/FrameFlow.exe"),
  [string]$AudioPath = "",
  [string]$ReportPath = (Join-Path $PSScriptRoot "../release/single-exe-verification.json")
)
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$testDir = Join-Path ([IO.Path]::GetTempPath()) ("frameflow-single-exe-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $testDir | Out-Null
Copy-Item -LiteralPath (Resolve-Path -LiteralPath $Executable).Path -Destination (Join-Path $testDir "FrameFlow.exe")
if (@(Get-ChildItem -LiteralPath $testDir).Count -ne 1) { throw "Test directory must initially contain only the EXE" }
$savedEnvironment = @{}
Get-ChildItem Env: | Where-Object Name -Like 'FRAMEFLOW_*' | ForEach-Object {
  $savedEnvironment[$_.Name] = $_.Value
  Remove-Item -LiteralPath ("Env:" + $_.Name)
}
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = $listener.LocalEndpoint.Port
$listener.Stop()
$env:FRAMEFLOW_ADDRESS = "127.0.0.1:$port"
$env:FRAMEFLOW_OPEN_BROWSER = "0"
$env:FRAMEFLOW_RATE_LIMIT = "10000"
$baseURL = "http://127.0.0.1:$port"
$client = [Net.Http.HttpClient]::new()
$taskProcess = $null
$socket = $null

function Start-IsolatedServer {
  $started = Start-Process -FilePath (Join-Path $testDir "FrameFlow.exe") -WorkingDirectory $testDir -WindowStyle Hidden -PassThru
  try {
    for ($attempt = 0; $attempt -lt 480; $attempt++) {
      if ($started.HasExited) { throw "Packaged EXE exited before becoming healthy" }
      try {
        $health = Invoke-RestMethod "$baseURL/health" -TimeoutSec 2
        if ($health.status -eq "healthy") { return $started }
      } catch { Start-Sleep -Milliseconds 250 }
    }
    throw "Packaged EXE startup timed out"
  } catch {
    if (-not $started.HasExited) { Stop-Process -Id $started.Id }
    throw
  }
}

function Stop-IsolatedServer($running) {
  $null = Invoke-RestMethod "$baseURL/api/system/shutdown" -Method Post
  if (-not $running.WaitForExit(10000) -or $running.ExitCode -ne 0) { throw "Graceful shutdown failed" }
}

function Upload-Audio([string]$path, [string]$kind) {
  $form = [Net.Http.MultipartFormDataContent]::new()
  try {
    $fileContent = [Net.Http.ByteArrayContent]::new([IO.File]::ReadAllBytes($path))
    $form.Add($fileContent, "file", [IO.Path]::GetFileName($path))
    $form.Add([Net.Http.StringContent]::new([IO.Path]::GetFileName($path)), "name")
    $form.Add([Net.Http.StringContent]::new($kind), "kind")
    $response = $client.PostAsync("$baseURL/api/materials/upload", $form).GetAwaiter().GetResult()
    try {
      $body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
      if ([int]$response.StatusCode -ne 201) { throw "Audio upload failed: $body" }
      return $body | ConvertFrom-Json
    } finally { $response.Dispose() }
  } finally { $form.Dispose() }
}

function Run-FFmpeg([string[]]$arguments) {
  & $ffmpeg -hide_banner -loglevel error -y @arguments
  if ($LASTEXITCODE -ne 0) { throw "Embedded ffmpeg command failed" }
}

try {
  $taskProcess = Start-IsolatedServer
  if (Test-Path -LiteralPath (Join-Path $testDir "bin")) { throw "Standalone EXE unexpectedly needs a bin directory" }
  $runtime = Join-Path $testDir "data/runtime/media"
  $ffmpeg = (Get-ChildItem -LiteralPath $runtime -Recurse -Filter ffmpeg.exe -File | Select-Object -First 1).FullName
  $ffprobe = (Get-ChildItem -LiteralPath $runtime -Recurse -Filter ffprobe.exe -File | Select-Object -First 1).FullName
  if (-not $ffmpeg -or -not $ffprobe) { throw "Embedded tools were not prepared" }
  $toolHash = (Get-FileHash -LiteralPath $ffprobe).Hash
  $toolModified = (Get-Item -LiteralPath $ffprobe).LastWriteTimeUtc
  if (-not (Test-Path -LiteralPath (Join-Path (Split-Path $ffprobe -Parent) "FFmpeg-LICENSE.txt"))) { throw "Embedded license missing" }
  $mediaDir = Join-Path $testDir "data/uploads"
  New-Item -ItemType Directory -Force -Path $mediaDir | Out-Null
  if (-not $AudioPath) {
    $AudioPath = Join-Path $testDir "sample.mp3"
    Run-FFmpeg -arguments @("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "1", $AudioPath)
  }
  $AudioPath = (Resolve-Path -LiteralPath $AudioPath).Path
  $audioAsset = Upload-Audio $AudioPath "voice"
  $expectedDuration = (& $ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 $AudioPath)
  if ($LASTEXITCODE -ne 0 -or [Math]::Abs($audioAsset.media.duration_seconds - [double]::Parse($expectedDuration, [Globalization.CultureInfo]::InvariantCulture)) -gt 0.001) { throw "Uploaded audio duration is incorrect" }
  $audioDownload = Join-Path $testDir "downloaded-audio.mp3"
  Invoke-WebRequest ($baseURL + $audioAsset.url) -OutFile $audioDownload -UseBasicParsing
  if ((Get-FileHash -LiteralPath $AudioPath).Hash -ne (Get-FileHash -LiteralPath $audioDownload).Hash) { throw "Audio download bytes differ" }
  $metrics = Invoke-WebRequest "$baseURL/metrics" -UseBasicParsing
  if ($metrics.Content -notmatch "frameflow_queue_depth") { throw "Metrics unavailable" }
  $dist = Join-Path $PSScriptRoot "../frontend/dist"
  $assetCount = 0
  $guideCount = 0
  if (Test-Path -LiteralPath $dist) {
    $dist = (Resolve-Path -LiteralPath $dist).Path
    $staticDir = Join-Path $testDir "static-check"
    New-Item -ItemType Directory -Path $staticDir | Out-Null
    foreach ($asset in Get-ChildItem -LiteralPath $dist -Recurse -File) {
      $relative = $asset.FullName.Substring($dist.Length + 1).Replace('\', '/')
      $download = Join-Path $staticDir ($relative.Replace('/', '_'))
      Invoke-WebRequest "$baseURL/$relative" -OutFile $download -UseBasicParsing
      if ((Get-FileHash -LiteralPath $download).Hash -ne (Get-FileHash -LiteralPath $asset.FullName).Hash) { throw "Embedded frontend mismatch: $relative" }
      $assetCount++
      if ($relative.StartsWith('guide/')) { $guideCount++ }
    }
  }
  Run-FFmpeg -arguments @("-f", "lavfi", "-i", "testsrc2=size=320x240:rate=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", (Join-Path $mediaDir "clip1.mp4"))
  Run-FFmpeg -arguments @("-f", "lavfi", "-i", "testsrc2=size=240x320:rate=30", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", (Join-Path $mediaDir "clip2.mp4"))
  $musicPath = Join-Path $testDir "music.wav"
  Run-FFmpeg -arguments @("-f", "lavfi", "-i", "sine=frequency=220:sample_rate=48000", "-t", "0.5", $musicPath)
  $musicAsset = Upload-Audio $musicPath "music"
  $project = Invoke-RestMethod "$baseURL/api/projects" -Method Post -ContentType "application/json" -Body '{"name":"Single EXE composition test"}'
  $draftData = @{
    id="single-exe-draft";name="Test draft";version=0
    story=@{body="Local synthetic media";scenes=@(
      @{id="scene1";order=1;title="Landscape";visual_prompt="Synthetic";duration_seconds=1},
      @{id="scene2";order=2;title="Portrait";visual_prompt="Synthetic";duration_seconds=1})}
    video_versions=@(
      @{id="v1";scene_id="scene1";task_id="fixture1";local_media_path="/media/clip1.mp4";duration_seconds=1},
      @{id="v2";scene_id="scene2";task_id="fixture2";local_media_path="/media/clip2.mp4";duration_seconds=1})
    selected_versions=@{scene1="v1";scene2="v2"}
  }
  $null = Invoke-RestMethod "$baseURL/api/projects/$($project.id)/drafts" -Method Post -ContentType "application/json" -Body ($draftData | ConvertTo-Json -Depth 8)
  $socket = [Net.WebSockets.ClientWebSocket]::new()
  $cancel = [Threading.CancellationTokenSource]::new(5000)
  try { $null = $socket.ConnectAsync([Uri]"ws://127.0.0.1:$port/ws", $cancel.Token).GetAwaiter().GetResult() } finally { $cancel.Dispose() }
  $compositionBody = @{expected_version=0;idempotency_key="single-exe-test";aspect_ratio="9:16";resolution="720P";source_volume=1;voice_volume=1;music_volume=0.25;music_material_id=$musicAsset.id} | ConvertTo-Json
  $task = Invoke-RestMethod "$baseURL/api/projects/$($project.id)/drafts/single-exe-draft/compositions" -Method Post -ContentType "application/json" -Body $compositionBody
  $eventBytes = [byte[]]::new(4096)
  $cancel = [Threading.CancellationTokenSource]::new(5000)
  try { $event = $socket.ReceiveAsync([ArraySegment[byte]]::new($eventBytes), $cancel.Token).GetAwaiter().GetResult() } finally { $cancel.Dispose() }
  $eventJSON = [Text.Encoding]::UTF8.GetString($eventBytes, 0, $event.Count) | ConvertFrom-Json
  if ($eventJSON.task_id -ne $task.id) { throw "Task progress WebSocket event missing" }
  for ($attempt = 0; $attempt -lt 120; $attempt++) {
    $task = Invoke-RestMethod "$baseURL/api/tasks/$($task.id)"
    if ($task.status -eq "succeeded") { break }
    if ($task.status -eq "failed") { throw "Composition failed: $($task.error)" }
    Start-Sleep -Milliseconds 250
  }
  if ($task.status -ne "succeeded") { throw "Composition timed out" }
  $compositionFile = Join-Path $testDir "composition-download.mp4"
  Invoke-WebRequest "$baseURL/api/tasks/$($task.id)/download" -OutFile $compositionFile -UseBasicParsing
  $probe = (& $ffprobe -v error -show_entries 'format=duration:stream=codec_type,width,height' -of json $compositionFile) | ConvertFrom-Json
  $video = $probe.streams | Where-Object codec_type -EQ "video" | Select-Object -First 1
  if ($LASTEXITCODE -ne 0 -or $video.width -ne 720 -or $video.height -ne 1280 -or -not ($probe.streams | Where-Object codec_type -EQ "audio") -or [double]$probe.format.duration -lt 1.9) { throw "Composition output is invalid" }
  $socket.Abort(); $socket.Dispose(); $socket = $null
  Stop-IsolatedServer $taskProcess
  $taskProcess = Start-IsolatedServer
  if ((Get-Item -LiteralPath $ffprobe).LastWriteTimeUtc -ne $toolModified) { throw "Restart rewrote a valid cached tool" }
  Stop-IsolatedServer $taskProcess
  Set-Content -LiteralPath $ffprobe -Value "damaged cache" -Encoding ascii
  Remove-Item -LiteralPath $ffmpeg
  $taskProcess = Start-IsolatedServer
  if ((Get-FileHash -LiteralPath $ffprobe).Hash -ne $toolHash -or -not (Test-Path -LiteralPath $ffmpeg)) { throw "Tool cache was not repaired" }
  $repairedAsset = Upload-Audio $AudioPath "voice"
  if ($repairedAsset.media.duration_seconds -ne $audioAsset.media.duration_seconds) { throw "Audio upload failed after cache repair" }
  Stop-IsolatedServer $taskProcess
  $report = [ordered]@{
    executable=(Resolve-Path -LiteralPath $Executable).Path
    exe_sha256=(Get-FileHash -LiteralPath $Executable).Hash.ToLowerInvariant()
    initially_only_exe=$true;external_media_configuration=$false;bin_required=$false
    health="passed";metrics="passed";static_assets_verified=$assetCount;guide_images_verified=$guideCount
    mp3_upload="passed";mp3_duration_seconds=$audioAsset.media.duration_seconds;audio_download_checksum="passed"
    wav_upload="passed";http_composition="passed";composition_width=$video.width;composition_height=$video.height
    composition_has_audio=$true;websocket_task_progress="passed";restart_cache_reuse="passed";cache_repair="passed"
    graceful_shutdown="passed";paid_model_requests=0;test_directory=$testDir
  }
  $report | ConvertTo-Json | Set-Content -LiteralPath $ReportPath -Encoding utf8
  $report | ConvertTo-Json
} finally {
  if ($socket) { $socket.Abort(); $socket.Dispose() }
  if ($taskProcess -and -not $taskProcess.HasExited) { Stop-Process -Id $taskProcess.Id }
  $client.Dispose()
  Get-ChildItem Env: | Where-Object Name -Like 'FRAMEFLOW_*' | ForEach-Object { Remove-Item -LiteralPath ("Env:" + $_.Name) }
  foreach ($name in $savedEnvironment.Keys) { Set-Item -LiteralPath ("Env:" + $name) -Value $savedEnvironment[$name] }
}
