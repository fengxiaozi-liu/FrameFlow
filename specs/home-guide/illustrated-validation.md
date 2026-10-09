# 图文操作指南增补验证

日期：2026-10-09。

## 改动

- 四个阶段补充 10 张基于当前界面的截图：厂商连接、模型启用、故事联想、正文采用、分镜应用、素材引用、分镜视频设置、结果选用、成片设置和下载。
- 图中以橙色边框和编号标记操作位置；每图附入口路径、对应说明及完成状态。均标记为演示数据，不包含用户数据或真实凭据。
- 图解支持页签、上一张/下一张，以及指南内原尺寸放大、滚动、Esc 返回和焦点恢复。手机优先显示图解，详细文字与 FAQ 保留。
- 修正旧指南「生成」按钮文案为当前实际「生成视频」。
- 图片来自实际组件与隔离的模拟响应；视频预览仅使用演示封面，说明中明确标注。

## 验证

- 图片生成脚本：1 项通过；所有 API 响应在浏览器中拦截，不向厂商发请求。
- 首页和指南回归：6 项通过（13.4 秒）。逐一验证全部 10 图在桌面和手机加载、切换、放大/返回、焦点恢复、无页面横向溢出，四步桌面 axe WCAG 2A/2AA 检查通过。
- 验证阅读图文指南不会创建项目或触发生成；原有跳转与已有正文路径正常。
- npm run build（含 vue-tsc）、npm run lint、git diff --check 通过。
- 已检查全部 10 张图片，以及桌面、手机与放大预览截图。
- EXE 隔离启动通过；内嵌前端为最新版本，10 张 PNG 均以 image/png 返回且长度与源文件相同；正常关闭，测试包不含用户数据。

## 维护和交付

图片位于 frontend/public/guide。重新截图：在 frontend 目录运行 `npx playwright test --config scripts/guide-capture.config.ts`。脚本独立于日常回归测试；真实界面变化后可重新生成。

测试包：release/FrameFlow-test-20261009-illustrated-guide.zip。
大小：93,158,339 字节。
SHA-256：52FEA8A4C948E97D567A74C5B3F6183366E4FF600182FD8F889ED87883D02788。

运行结果：artifacts/illustrated-exe-smoke.json。
界面截图：artifacts/illustrated-guide-desktop.png、illustrated-guide-mobile.png、illustrated-guide-zoom.png。
