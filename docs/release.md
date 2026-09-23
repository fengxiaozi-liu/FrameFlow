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

## 故事与分镜生成功能交付检查

日期：2026-09-23。范围：`specs/story-storyboard-generation/tasks.md` T001–T037。

## 已通过

| 检查 | 结果 |
| --- | --- |
| `go test ./...`（设置本地 FFmpeg/ffprobe 路径） | 通过，包含真实两段异画幅视频、旁白和循环配乐合成；输出分辨率与音轨已验证 |
| `go vet ./...` | 通过 |
| `npm run lint`、`npm run test`、`npm run build` | 通过；Vitest 8/8 |
| `npm run e2e -- --reporter=list --workers=1` | 通过；Playwright 12/12，覆盖焦点、窄屏、键盘排序、失败与重试 |
| `npm run api:generate` 后校验 `schema.d.ts` SHA256 | 无生成漂移 |
| Windows `scripts/build-windows.ps1` | 通过；发行包包含 FFmpeg/ffprobe。发行版启动后 `/health`、`/`、JS 静态资源均返回 200 |
| 后端 Docker 镜像构建与启动 | 通过；`/health` 与 `/` 返回 200，容器内 ffprobe 可执行，日志显示 `composition=true` |
| `git diff --check` | 通过 |

SQLite 旧数据迁移、重复执行、失败回滚、素材引用删除保护及跨草稿任务均有自动回归测试。概念图与当前页面的对应关系记录在 `specs/story-storyboard-generation/images/README.md`。

## 尚待有凭据环境验证的发布门槛

本机未配置百炼 API 凭据及音频对象存储的地址、桶和访问凭据。因此，真实故事模型调用、Wan 2.7 视频模型调用、驱动音频上传并由模型读取、远端结果回传的云端全链路冒烟尚未执行。当前验证覆盖请求结构、模拟响应、转存及失败路径，不能替代这些真实调用。

本机已用实际 FFmpeg 执行应用层合成测试，并验证 Windows 包和容器含可用 FFmpeg；尚未分别在 Windows 发行版和容器内通过 HTTP 提交完整成片任务并下载输出。此项在目标环境补跑后再将其标为发布通过。

容器检查针对后端镜像。`docker-compose.yml` 使用独立前端容器；单独运行后端镜像时首页可能是嵌入的提示页，应通过前端容器访问完整界面。
