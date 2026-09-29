# 实施验证记录

日期：2026-09-28～2026-09-29。实施与最终验收完成；历史阶段记录保留，最终结论见下文。

## Phase 1

- OpenAPI：新增 22 个结构、16 个路径，openapi-typescript 校验通过。
- 指定工作空间独立 HTTP 多图生图/TTS/视频生成成功，产物已下载，详见 provider-capabilities.md。
- ui-contract.md 已固定作用域、蓝绿色主题、错误和保存反馈。

## Phase 2

- 领域与 provider 单元测试通过；修复快照 canonical 副本复用 slice 导致 URL 被意外清空的问题，保留回归测试。
- 前端 api:generate 与 build 通过；新增 speech 类型的任务标签适配通过。
- 对比度计算：#062D29/#14B8A6 为 5.96；白字/#0F766E 为 5.47；#A2BAB3/#182421 为 7.78。

## Phase 3

- SQLite 迁移重开与历史素材删除保护测试通过。
- 多图 HTTP 顺序、语音参数和兼容列表路径测试通过；旧原生发现接口仍有测试覆盖。
- 前端 build 通过；组件交互 4 项通过（稳定引用、IME、多图排序/移除、音色继承恢复）。

## Phase 4

- application 与 HTTP 测试通过，包含实体版本冲突、镜头原子更新、外部身份拒绝和无效修改回滚。
- 音轨处理的实测媒体测试待 FFmpeg/ffprobe 运行环境准备完成后执行；不得将跳过视为通过。

## Phase 5～10：实现与联调

- 完成解析器、不可变任务快照、逐句配音、媒体版本、流程编排、候选应用与恢复；SQLite 引用保护及版本冲突有集成覆盖。
- 配置测试专用 FFmpeg/ffprobe，实际执行音频顺序/停顿/时长及成片媒体测试，补齐 Phase 4 的待验项。
- Phase 8 真实应用流程五项任务成功，`provider-test.md` 和 `evidence/live-integration.json` 记录供应商与本地结果；`evidence/live-playback.png` 记录播放。
- Phase 10 全量 `go test ./...`、`npm run api:generate`、`lint`、`test`（13 项）、`build`、`e2e`（19 项）通过。

## Phase 11～12：亲自走查和修正

- 实际浏览器逐项操作，详见 `interaction-review.md`；包含两场景两人物各两张图、引用、音色、断网、冲突、真实候选、时长超限、缩放及窄屏。
- 修复档案版本未递增、Enter/取消串绑、重复表单、弹层焦点、试听竞争、人物音色版本、快速切换回填、停顿保存和任务订阅初始化竞争。
- 恢复本地合成任务；下载失败不再重新生成；完整流程提前验证视频配置；取消与流程推进互斥；过期首帧拒绝继续生成。
- 新增 E2E 验证编辑版本、精确键盘引用和快速切换持久化。保留并修复原候选事件用例，无弱化断言。

## Phase 13：最终验收结果

环境：Windows PowerShell；隔离 SQLite、上传目录与保险库（`release/binding-test/`）；浏览器 Chromium；前端 15174 / 后端 19090。自动 E2E 使用独立临时库及 15173 / 18080。

| 检查 | 命令/证据 | 最终结果 |
|---|---|---|
| 后端全量 | 配置 FRAMEFLOW_FFMPEG_PATH、FRAMEFLOW_FFPROBE_PATH 后 `go test ./...` | 全部通过，含实际媒体测试；日志 `release/binding-test/final-go.log` |
| 类型生成 | `npm run api:generate` | 通过 |
| 静态检查 | `npm run lint` | 通过 |
| 前端单测 | `npm run test` | 5 文件、13 项通过 |
| 构建 | `npm run build` | TypeScript 与 Vite 通过；最终生成 index-DbH9NbgQ.js |
| 全量 E2E | `npm run e2e -- --reporter=line` | 20 项通过，28.9 秒；日志 `release/binding-test/final-e2e.log` |
| 最后阶段文案修正 | 流程状态改为“正在生成视频”；重跑流程 HTTP 用例、lint/build | 通过 |
| 实际供应商主流程 | run-c989010241683f4fb95bb9335b907f70 | image + speech × 2 + video + composition 全成功，父流程 completed |
| 实际重启恢复 | 云端视频任务 702e51d8-1837-4171-9a3f-56eeea48d9a1 | 重启前后 ID 一致，原视频成功落盘 |
| 实际播放 | 界面选用首帧/音轨/视频，导出后点击播放 | duration=6、readyState=4、paused=false |
| 源码与证据 | `evidence/final-acceptance.json` | 记录最终核心源码 SHA256、任务状态与音色；不含密钥 |
| 基础检查 | git diff --check；源码/文档凭证模式扫描 | 无空白错误；无测试密钥写入源码/文档 |

可直接查看：`evidence/final-video.mp4`、`evidence/final-playback.png`、`evidence/final-workbench.png`。第二次最终主流程通过浏览器从空镜头复用项目档案，绑定两张人物/服装图、添加 @ 场景、设置两人台词及 200ms 停顿、提交制作并逐项选用完成；不是仅复播第一次生成结果。

### 验证范围与外部限制

- 真实调用用户指定阿里云工作空间；多图使用 qwen-image-2.0-pro（当前上限 3 张），两人音色 Cherry/Ethan，视频 wan2.7-i2v，成片 FFmpeg。
- 图片参考用于生成首帧，再以首帧生成视频。多图分组可保存更多素材，但超模型输入数量必须明确取消本镜头选用，不能静默裁剪。
- 配音为后期音轨合入，不承诺口型同步或模型生成的人物绝对一致性。工具验证角色/音色参数、媒体文件、时长和可播放性，没有进行主观听感打分。
- 部分上传失败、快速双击和迁移异常用故障注入/自动测试验证；实际服务断开、重启、超长配音和真实模型主流程已另外操作。二者不混同。
- 125%/150% 采用临时 CSS zoom 实际页面渲染测试，已移除临时样式；未验证浏览器菜单自身的缩放操作。系统输入法候选窗未人工操作，中文内容与 isComposing 行为有独立覆盖。
- 未运行其他云厂商、未配置的音色、浏览器或操作系统组合。旧数据库回退遵循 `docs/migrations/scene-character-binding.md` 的备份恢复要求。

当前不存在未解决的阻断问题。43 个任务的验收范围已完成；以上限制保留在交付记录中，不将其描述为已测试能力。
