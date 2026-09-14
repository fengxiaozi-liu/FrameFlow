# Windows 一键运行包

在已安装 Go 1.23、Node.js 22 和 npm 的开发机上，从仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

脚本会构建 Vue 静态资源并嵌入 Go 服务，生成 `release\FrameFlow.exe`。将该文件复制到目标 Windows 目录后双击即可启动，程序会在 `http://127.0.0.1:28741` 启动并打开默认浏览器。Windows 版本以 GUI 子系统运行，不显示控制台窗口；日志保存在 exe 同目录的 `data\frameflow.log`。

可在页面右上角点击“退出”关闭后台服务。若 28741 端口已被占用，可在启动前设置地址：

```powershell
$env:FRAMEFLOW_ADDRESS = "127.0.0.1:18080"
.\release\FrameFlow.exe
```

目标机器只需要 `FrameFlow.exe`，不需要安装 Node.js、npm 或 Go。

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
