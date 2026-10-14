# Windows 一键运行包

在已安装 Go 1.23、Node.js 22 和 npm 的开发机上，从仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

脚本会构建 Vue 静态资源，将前端、指南图片和压缩后的 FFmpeg/ffprobe 一起嵌入 `release\FrameFlow.exe`。打包时默认下载并校验固定版本的 Windows FFmpeg；也可用 `FRAMEFLOW_FFMPEG_DIST_DIR` 指定包含两个 exe 的目录。目标 Windows 电脑只需复制 `FrameFlow.exe` 到可写目录后运行，无需另带 `bin` 文件夹。程序会在 `http://127.0.0.1:28741` 启动并打开默认浏览器。Windows 版本启动后会将独立控制台最小化到任务栏，不输出接口请求日志；错误和运行日志保存在 exe 同目录的 `data\frameflow.log`。若从已有终端启动，则不会最小化该终端。

点开控制台可以看到运行状态、访问地址和日志位置。EXE 和传统控制台窗口使用 FrameFlow 图标；若系统默认使用 Windows Terminal，任务栏及标签图标仍由 Windows Terminal 管理，无法由控制台程序替换。启动失败时会显示错误提示，详情保存在日志中。

可在页面右上角点击“退出”关闭后台服务。若 28741 端口已被占用，可在启动前设置地址：

```powershell
$env:FRAMEFLOW_ADDRESS = "127.0.0.1:18080"
.\release\FrameFlow.exe
```

目标机器不需要安装 Node.js、npm、Go 或媒体工具。首次启动自动准备内嵌工具，可能比后续启动稍慢。建议将 EXE 放在固定文件夹，便于保存和备份同目录的 `data`；从 ZIP 直接启动时数据仍保存在 Windows 的临时解压目录。

## 媒体能力配置

成片合成和音频元数据读取使用 EXE 内嵌的 `ffmpeg` 和 `ffprobe`。启动时自动释放到 `data\runtime\media\<内嵌包摘要>`，保留许可证与来源说明，并以 SHA-256 校验；缺失或损坏的缓存自动重建。不同内嵌版本使用不同目录，避免替换仍在使用的工具。文件先完整写入临时文件、校验通过后才替换，支持同时启动时复用同一版本。

启动时检查工具能否执行 `-version` 和媒体工作目录是否可写。准备失败会记录原因，草稿编辑仍可用。可通过 `FRAMEFLOW_FFMPEG_PATH`、`FRAMEFLOW_FFPROBE_PATH` 显式指定其他完整路径（优先于内嵌工具），`FRAMEFLOW_MEDIA_WORK_DIR` 指定合成临时目录（默认 `data\media-work`）。普通开发构建及非 Windows 构建继续使用原有 `bin` / PATH 配置；带 `embedded_media` 标签的 Windows 构建须先运行 `scripts/prepare-windows-media.ps1`。自动发布的 Windows 构建使用同一准备脚本。

“生成参考音频”需要模型可访问的阿里云 OSS 地址。配置 `FRAMEFLOW_AUDIO_OBJECT_BASE_URL` 为 Bucket 的 HTTPS OSS 主机，例如 `https://examplebucket.oss-cn-beijing.aliyuncs.com`，并配置 `FRAMEFLOW_AUDIO_OBJECT_BUCKET`、`FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_ID`、`FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_SECRET`。提交驱动音频时，服务端使用 OSS 签名上传本地 MP3/WAV，并向模型传递两小时有效的签名下载 URL。启动时检查配置是否齐备；缺少配置时拒绝驱动音频提交，本地后期配音编辑仍可使用。密钥不会写入日志。

## GitHub 自动发布

推送版本标签后，GitHub Actions 会构建 Windows、Linux 和 macOS 包并创建 Release：

```powershell
git add .
git commit -m "release: prepare package"
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

标签必须以 `v` 开头，例如 `v0.1.0`。完成后可在 GitHub 的 Releases 页面下载对应平台的文件。
