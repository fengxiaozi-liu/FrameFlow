# Release and recovery

## Pre-release checks

Run `go test ./...`, `go vet ./...`, `npm run lint`, `npm test`, `npm run build`, and `npm run e2e`. Build containers with `docker compose build`, or build the self-contained Windows package with `powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1`. Verify `/health`, `/metrics`, task creation, WebSocket progress, cancellation, retry, and result download.

## Backup

Create a consistent SQLite online backup from `backend` with `go run ./cmd/maintenance -action backup -database data/frameflow.db -output backups/frameflow.db`. The destination must not already exist. Back up `data/uploads`, `data/secrets/vault.key`, and `data/secrets/credentials.json` with it. The encrypted credentials cannot be restored without `vault.key`.

## Restore

Stop the backend, restore the database, uploads, and both vault files into the same data volume, then run `go run ./cmd/maintenance -action integrity -database data/frameflow.db`. Start the stack and verify `/health` before accepting traffic.

## Migration rehearsal

Restore the latest backup into a temporary volume, start the target release, confirm schema creation is idempotent, and execute the E2E suite. Keep the previous image and backup until the new version passes smoke tests.

## Performance baseline

The MVP acceptance target is: task creation responds in under one second locally, list endpoints return no more than 100 records per request, and WebSocket progress appears within two seconds. `TestTaskCreationPerformanceAndMetrics` enforces the local 20-task creation target. Monitor `frameflow_queue_depth` and `frameflow_tasks{status=...}` from `/metrics`, together with CPU, memory, SQLite lock errors, and total task duration.
