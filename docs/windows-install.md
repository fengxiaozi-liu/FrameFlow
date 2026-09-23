# Windows 一键运行包

在已安装 Go 1.23、Node.js 22 和 npm 的开发机上，从仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

脚本会构建 Vue 静态资源并嵌入 Go 服务，生成 `release\FrameFlow.exe`，并将 FFmpeg/ffprobe 放入 `release\bin`。打包时默认下载并校验固定版本的 Windows FFmpeg；也可用 `FRAMEFLOW_FFMPEG_DIST_DIR` 指定包含两个 exe 的目录。将整个 `release` 目录复制到目标 Windows 目录后运行 `FrameFlow.exe`，程序会在 `http://127.0.0.1:28741` 启动并打开默认浏览器。Windows 版本启动后会将独立控制台最小化到任务栏，不输出接口请求日志；错误和运行日志保存在 exe 同目录的 `data\frameflow.log`。若从已有终端启动，则不会最小化该终端。

点开控制台可以看到运行状态、访问地址和日志位置。EXE 和传统控制台窗口使用 FrameFlow 图标；若系统默认使用 Windows Terminal，任务栏及标签图标仍由 Windows Terminal 管理，无法由控制台程序替换。启动失败时会显示错误提示，详情保存在日志中。

可在页面右上角点击“退出”关闭后台服务。若 28741 端口已被占用，可在启动前设置地址：

```powershell
$env:FRAMEFLOW_ADDRESS = "127.0.0.1:18080"
.\release\FrameFlow.exe
```

目标机器需要完整 `release` 目录，不需要安装 Node.js、npm 或 Go。

## 媒体能力配置

成片合成依赖 `ffmpeg` 和 `ffprobe`。Windows 发行包默认从 exe 同目录的 `bin` 加载；启动时检查两者能否执行 `-version`，并检查媒体工作目录是否可写。检查失败会拒绝合成提交，不影响草稿编辑。可通过 `FRAMEFLOW_FFMPEG_PATH`、`FRAMEFLOW_FFPROBE_PATH` 指定其他完整路径，`FRAMEFLOW_MEDIA_WORK_DIR` 指定临时工作目录（默认 `data\media-work`）。

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
