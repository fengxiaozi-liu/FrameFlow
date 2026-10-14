# Release and recovery

## Pre-release checks

Run `go test ./...`, `go vet ./...`, `npm run lint`, `npm test`, `npm run build`, and `npm run e2e`. Build containers with `docker compose build`, or build the self-contained Windows package with `powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1`. Verify `/health`, `/metrics`, task creation, WebSocket progress, cancellation, retry, and result download.

Windows 单文件交付须验证：仅复制 `FrameFlow.exe` 到新的可写目录，不设置媒体工具路径、不复制 `bin`，使用隔离数据启动；上传 MP3 / WAV 并核对时长、下载原始字节，执行真实本地成片合成，验证重启复用和删除 / 损坏工具缓存后的自动修复。内嵌包由 `scripts/prepare-windows-media.ps1` 生成，包含 FFmpeg、ffprobe、GPLv3 许可证、来源说明及 SHA-256 清单；运行时写入版本化缓存，发行包禁止包含此缓存或用户数据。原 `bin` 目录仅可用作开发机的工具来源，不进入新的单文件 ZIP。

可在仓库根运行 `./scripts/test-windows-package.ps1` 自动执行上述隔离验证；使用 `-AudioPath <本地 MP3 路径>` 验证已有音频，省略时生成测试音频。脚本只操作新建临时目录和独立端口，不启动现有用户数据库；合成输入是本地生成的演示片段，不调用模型。

2026-10-13 单文件修复验证：后端全量测试和 vet、带 `embedded_media` 标签的运行时 / 启动测试、媒体缓存并发测试（重复 20 次）通过；前端 lint、56 项单元测试、生产构建及 62 项 E2E 通过。仅复制 EXE 的隔离目录中，实际 MP3 与生成的 WAV 上传、原始音频字节下载、两段不同画幅视频与循环音乐的 HTTP 合成及下载通过，输出为带音轨的 720×1280 视频；30 个前端资源（含 23 张指南图）逐项校验通过，任务 WebSocket 进度、重启复用、工具缓存缺失 / 损坏后的自动修复与正常退出通过。未操作真实用户数据或执行付费模型调用；自动发布流程已使用内嵌构建，但本轮未在 GitHub 触发发布。

2026-10-12 自定义连接测试版的验证范围、真实目录同步、兼容边界与回滚步骤见 [验证记录](../specs/custom-provider-connection/verification.md)。该记录不代表真实计费生成已通过。Windows ZIP 按文件白名单创建，不将整个 release 目录打包；该目录可能含运行日志、用户 data 和历史 ZIP。

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
