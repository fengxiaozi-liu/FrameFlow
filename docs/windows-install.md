# Windows 一键运行包

在已安装 Go 1.23、Node.js 22 和 npm 的开发机上，从仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

脚本会构建 Vue 静态资源并嵌入 Go 服务，生成 `release\FrameFlow.exe`。将 `FrameFlow.exe` 复制到目标 Windows 目录后双击即可启动，程序会在 `http://127.0.0.1:8080` 启动并打开默认浏览器。运行数据保存在 exe 同目录的 `data` 文件夹。

停止程序可关闭命令窗口或使用 Ctrl+C。若 8080 已被占用，请先停止原 FrameFlow 进程。

目标机器只需要 `FrameFlow.exe`，不需要 Node.js、npm 或 Go；构建机仍需要这些工具。

## GitHub 自动发布

推送版本标签后，GitHub Actions 会在 Windows runner 上构建并创建 Release：

```powershell
git add .
git commit -m "release: prepare windows package"
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

工作流文件为 `.github/workflows/release.yml`。标签格式必须是 `v` 开头，例如 `v0.1.0`。完成后可在 GitHub 的 Releases 页面下载 `FrameFlow.exe`。
