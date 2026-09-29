# 任务清单：分镜场景、人物与音色绑定工作台

**Spec**: 尚无独立 spec.md；需求编号沿用 plan.md 中 RQ-001～RQ-009（对话需求映射）
**Plan**: `specs/scene-character-binding/plan.md`
**Created**: 2026-09-28
**项目识别结果**: Vue 3 / TypeScript / Pinia；Go / Gin；SQLite JSON 聚合存储；厂商适配、异步任务与媒体合成。
**技术链依赖**: 本清单依据 speckit-tasks/templates/tasks-template.md 生成。Phase 1 使用 HTTP 黑盒验证与 API 契约设计；Phase 2–4 使用 Go 领域/事务/API、Vue 组件和状态设计；Phase 5–7 使用输入编译、provider 适配、任务编排及媒体处理；Phase 8–9 使用候选应用、版本恢复、集成/E2E/迁移验证；Phase 10–13 完成自动化交互测试、亲自走查、自主修正复测与最终交付门禁。已使用 speckit-plan 与 speckit-tasks；后续衔接 speckit-implement 和 speckit-code-review。未发现可加载的专用 Go/Vue 框架技能，不假设其存在。

---

## 格式说明

- `[TaskID]`：全局递增唯一编号。
- `[技术域]`：任务所属层；每项包含路径、需求映射和可验收结果。新文件路径为建议产物。
- `[✅️]` 表示已实现并验证的结果（按 speckit-implement 约定）；除已执行的接口探测外，实施任务均未开始。
- 阶段按拓扑顺序推进，同阶段任务无强前置依赖；任务包内部相关实现和测试一起完成。这里的 Phase 比 plan 的 P0–P6 更细，避免把接口、实现与消费接口的任务放在同一依赖层。
- P0 对应 Phase 1；P1 对应 Phase 2–4；P2 对应 Phase 2–5；P3/P4 对应 Phase 5–7；P5 对应 Phase 8；P6 对应 Phase 9–13。
- 测试凭证只从运行时环境或凭证保险库读取；禁止写入本清单、测试脚本、日志及夹具。

---

## Phase 1：验证基础服务并冻结接口契约

- [✅️] T001 [Integration] [RQ-006/RQ-008] 使用指定服务完成模型列表与一次 qwen-plus 结构化分镜请求，记录状态、token 用量和实体引用检查；区分独立 HTTP 探测与应用内联调。产物：`specs/scene-character-binding/provider-test.md`。
- [✅️] T002 [API] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-007] 固定实体版本、镜头绑定、PromptDocument、Utterance、VoiceProfile、快照和错误协议；明确项目设定/镜头补充编辑语义、删除规则及版本锁。更新 `docs/api/openapi.yaml`、`specs/scene-character-binding/plan.md`；契约涵盖列表、编辑、预览、生成与选用。
- [✅️] T003 [Provider] [RQ-004/RQ-006] 核对指定服务官方多图/生图/视频/TTS 协议与实际模型权限，用最小请求分别验证并记录请求路径、模型、限制和结果；不得把列表可见视作可调用。产物：`specs/scene-character-binding/provider-capabilities.md`。如服务未提供所需能力，明确缺少的服务信息，不虚构成功。
- [✅️] T004 [UI] [RQ-005/RQ-009] 定义蓝绿色主题令牌、亮暗背景文字搭配、控件状态、卡片布局和小屏规则；明确人物辅助色、卡片作用域提示及首版不含自由画布。产物：`specs/scene-character-binding/ui-contract.md`，建议主色 #14B8A6、亮底按钮 #0F766E。

阶段门槛：T002/T004 冻结后可推进无厂商依赖的工作；T003 未解决时多图/TTS 真实适配任务保持待办，其余模块不伪造成功。

## Phase 2：领域模型、主题基础与 API 类型

- [✅️] T005 [Domain] [RQ-001/RQ-002/RQ-004/RQ-007] 新增版本化 Location、Character、VoiceProfile 与有序 ReferenceImage；档案归项目所有且历史不可原位改写。新增 `backend/internal/domain/project/entities.go` 及实体版本校验测试。
- [✅️] T006 [Domain] [RQ-003/RQ-004/RQ-007] 定义结构化提示词节点、台词与说话者类型，以及纯 ID 引用校验；覆盖重名、旁白、未出镜说话者和无台词情况。新增 `backend/internal/domain/story/prompt_document.go`、`utterance.go` 及测试。
- [✅️] T007 [Domain] [RQ-006/RQ-007] 定义不可变输入快照、内容指纹、speech 任务、generation run 和依赖字段；明确原始任务状态与结果过期状态分离。修改 `backend/internal/domain/task/task.go`，新增 `generation_snapshot.go` 及测试。
- [✅️] T008 [Provider] [RQ-004/RQ-006] 扩展 provider 的多图请求、音色目录、语音合成和能力声明接口；只注册已验证能力，保持旧适配兼容。修改 `backend/internal/domain/provider/provider.go`、`video_capability.go`、`backend/internal/infrastructure/provider/registry.go`，补协议契约测试。
- [✅️] T009 [UI] [RQ-005/RQ-009] 在 `frontend/src/style.css` 建立蓝绿色设计令牌并映射 --accent/--tint；新增 `frontend/src/styles/studio.css` 限定暗色工作台，统一按钮/焦点/选中态。验收亮暗底对比度及其他页面不被误改成暗色。
- [✅️] T010 [API] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-007] 根据 OpenAPI 生成类型并补客户端调用；减少重复手写类型，以契约为唯一来源。更新 `frontend/src/api/schema.d.ts`、`types.ts`、`client.ts`；运行 `npm run api:generate` 验证生成一致。

## Phase 3：聚合存储、真实适配与独立组件

- [✅️] T011 [DB] [RQ-001/RQ-002/RQ-007] 将实体、绑定、台词和快照接入聚合；实现 catalog_version 与 draft.version 独立校验、引用索引事务更新及幂等迁移；旧素材无法识别人时保留待归组。修改 `backend/internal/domain/project/project.go`、`backend/internal/infrastructure/sqlite/task_repository.go`，新增 `entity_migration.go` 与迁移测试。
- [✅️] T012 [Provider] [RQ-006/RQ-008] 为指定服务支持兼容模式模型列表与正确 URL 拼接，保持现有百炼路径可配置；测试完整 completions URL 不会重复拼接。修改 `backend/internal/infrastructure/provider/bailian/client.go`、`client_test.go`；不在测试中嵌入凭证。
- [✅️] T013 [Provider] [RQ-006] 依据 T003 实测实现至少一条多图视觉适配，保留参考图顺序、分组、描述和角色语义映射；用 HTTP 测试确认媒体确实发送。修改 `backend/internal/infrastructure/provider/bailian/protocol/qianwen.go`、`wanxiang.go`、`client.go`；能力不足时返回明确错误。
- [✅️] T014 [Provider] [RQ-004] 依据 T003 实现语音合成、音色查询、超时取消和结果获取；覆盖厂商音色 ID 无效与模型不兼容。新增 `backend/internal/infrastructure/provider/bailian/speech.go`、`speech_test.go`，复用现有凭证读取。
- [✅️] T015 [UI] [RQ-001/RQ-002/RQ-005] 实现场景/人物卡与参考图列表：名称、描述、多选、上传事件、排序、视角标签、移除和折叠；明确项目设定与镜头补充。新增 `frontend/src/components/studio/EntityReferenceCard.vue`、`ReferenceImageList.vue` 和交互测试，先使用契约数据。
- [✅️] T016 [UI] [RQ-003/RQ-005] 实现结构化 @ 编辑器与候选列表，支持改名显示、键盘选择、粘贴、节点删除与中文 IME；不能只用正则匹配名字。新增 `frontend/src/components/studio/MentionPromptEditor.vue` 及测试。
- [✅️] T017 [UI] [RQ-004/RQ-005] 实现逐句台词与音色组件，区分继承/覆盖、旁白、试听状态及错误提示。新增 `frontend/src/components/studio/DialogueVoicePanel.vue`、`VoiceSelector.vue` 及交互测试。
- [✅️] T018 [UI] [RQ-005/RQ-006] 实现小尺寸视频预览、候选列表、生成工具栏与阶段状态组件；参数选项来自能力，不写死“支持任意多图”。新增 `frontend/src/components/studio/ShotPreview.vue`、`GenerationToolbar.vue`。

## Phase 4：业务接口、输入解析与音轨处理

- [✅️] T019 [Service] [RQ-001/RQ-002/RQ-004/RQ-007] 实现项目档案、音色档案 CRUD、归档和影响范围查询；检查实体归属与版本冲突，返回最新版本和使用位置。新增 `backend/internal/application/project_entities.go`、`voice_profiles.go`，接入 `backend/internal/interfaces/http/router.go` 并补 HTTP 测试。
- [✅️] T020 [Service] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-007] 实现镜头绑定、PromptDocument、台词、局部覆盖的原子写入；引用类型错误和跨项目 ID 必须拒绝；旧全量保存保留新字段。修改 `backend/internal/application/project_bindings.go`、`project_scenes.go`、`project_service.go` 并覆盖冲突测试。
- [✅️] T021 [Service] [RQ-003/RQ-004/RQ-006/RQ-007] 实现 BindingResolver，按一致版本解析描述、选用图、主体角色、说话者及音色；同一地点多角度与跨地点组有明确作用。新增 `backend/internal/application/binding_resolver.go` 及纯逻辑测试；输出字段级问题而非自动丢弃输入。
- [✅️] T022 [Media] [RQ-004] 实现配音结果实测时长、顺序/停顿安排和镜头音轨混合；超时长明确报告，不能静默裁剪；明确原片人声保留规则。新增 `backend/internal/infrastructure/media/voice_track.go` 及实际媒体夹具测试。

## Phase 5：输入编译、任务执行基础与工作台接入

- [✅️] T023 [Service] [RQ-006/RQ-007] 实现 GenerationCompiler 与输入预览，按能力选择直接多图或多图生首帧路径；固定实体/素材/模型/适配版本；哈希包括参数、参考顺序及内容摘要，排除临时 URL 与密钥。新增 `backend/internal/application/generation_compiler.go`、`generation_preview.go`，更新路由与编译测试。
- [✅️] T024 [Task] [RQ-004/RQ-006/RQ-007] 接入 speech/image/video 任务处理与可恢复待入队；快照与任务原子持久化，工作进程只读取快照；重复回调、重试、超时状态有明确处理。修改 `backend/internal/domain/processor/processor.go`、`backend/internal/infrastructure/provider/registry.go`、`backend/internal/infrastructure/queue/memory.go` 及相关测试。
- [✅️] T025 [UI] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-007/RQ-009] 将独立组件接入工作台，建立服务端快照/本地脏字段/串行保存与冲突合并；完成多图上传、卡片编辑和 @引用绑定，任务通知不覆盖输入。修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`，新增 `frontend/src/composables/useShotEditor.ts` 及状态测试。
- [✅️] T026 [UI] [RQ-004/RQ-005/RQ-006] 增加语音模型与音色档案配置、素材引用来源和上传失败重试。修改 `frontend/src/views/ConfigView.vue`、`MaterialsView.vue`；从 API 获取能力及音色，不提供虚假可用项。

## Phase 6：真实生成入口、候选结果与成片配音

- [✅️] T027 [API] [RQ-004/RQ-006/RQ-007] 接通镜头生图/视频、逐句配音/试听接口，保存前校验 expected_version 和完整幂等指纹；重试沿用原快照，改参数新建任务。修改 `backend/internal/application/video_tasks.go`、`video_validation.go`，新增 `image_tasks.go`、`speech_tasks.go`，更新 `router.go` 与 HTTP 集成测试。
- [✅️] T028 [Task] [RQ-004/RQ-006/RQ-007] 幂等保存图片/音频候选和视频版本，支持选用及基于依赖的过期提示；镜头删除后晚到结果保留任务历史但不复活镜头。修改 `backend/internal/application/task_result_applier.go`，新增 `project_media_versions.go` 并补回填测试。
- [✅️] T029 [Media] [RQ-004/RQ-007] 合成时按已选台词音频构建镜头音轨，继承现有 voiceover/音乐合成；校验音频与视频实测时长及输出快照。修改 `backend/internal/application/composition_service.go`、`backend/internal/infrastructure/media/compose.go` 和合成集成测试。

## Phase 7：制作流程编排与前端生成联调

- [✅️] T030 [Task] [RQ-004/RQ-006/RQ-007] 实现持久化镜头制作流程：准备配音、生成并选用首帧、生成视频与合成；保存子任务依赖，等待选用状态不误判成功，重启恢复不重复生成已成功步骤。新增 `backend/internal/application/generation_runs.go`、`generation_runs_test.go`，扩展持久层与路由。
- [✅️] T031 [UI] [RQ-004/RQ-005/RQ-006/RQ-007] 接入输入预览、逐句试听、图片/音轨选用、视频提交、过期标记与错误恢复；展示哪些素材用于生图、哪些用于视频。修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/tasks.ts`、`frontend/src/components/studio/GenerationToolbar.vue`，补接口联调测试。
- [✅️] T032 [Story] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-008] 升级故事到分镜候选契约和严格解析，包含场景、人物、引用与台词；模型临时 key 由服务器映射为稳定 ID，未知 key/重复 key 拒绝。修改 `backend/internal/domain/story/generation.go`、`story.go`、`backend/internal/application/candidate_service.go` 并验证中文正文与歧义案例。

## Phase 8：候选应用、历史恢复与制作流程展示

- [✅️] T033 [Service] [RQ-007/RQ-008] 原子应用新候选并保留档案身份，复制镜头重建局部 ID 与 @映射；分镜替换/恢复覆盖新增绑定、台词及选用音轨，不修改其他草稿设定。修改 `backend/internal/application/project_candidates.go`、`project_scenes.go`、`project_versions.go`、`backend/internal/domain/project/project.go` 并补恢复测试。
- [✅️] T034 [UI] [RQ-005/RQ-006/RQ-007] 展示制作流程父子任务、阶段、等待首帧选择、失败重试与取消；“生成视频”入口明确接入所选路径。修改 `frontend/src/views/TasksView.vue`、`StudioView.vue`、`frontend/src/stores/tasks.ts`，验证页面重载可恢复。
- [✅️] T035 [Integration] [RQ-004/RQ-006] 在应用真实链路上分别运行一条多图视觉请求和双人物 TTS 请求，确认实际媒体/音色、文件下载与成片播放。补充 `specs/scene-character-binding/provider-test.md`；缺少对应权限/服务则保留未完成，不能用模拟通过替代真实验收。

## Phase 9：候选界面与独立回归检查

- [✅️] T036 [UI] [RQ-007/RQ-008] 展示 AI 提取的场景/人物/台词候选、消歧和待归组修复，绑定新版应用/恢复接口；修改 `frontend/src/views/StudioView.vue`，新增 `frontend/src/components/studio/EntityCandidateReview.vue`，验证应用前后档案身份稳定。
- [✅️] T037 [Test] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-007/RQ-009] 增加保存刷新、重名改名、中文输入、上传失败、台词覆盖、冲突回填、两种背景对比度与小屏测试。新增 `frontend/e2e/scene-character-binding.spec.ts`；回归现有 `frontend/e2e/core.spec.ts`、`task-results.spec.ts`。
- [✅️] T038 [Test] [RQ-006/RQ-007/RQ-008] 验证旧库迁移幂等、历史恢复、文件删除保护、进程重启、任务重复提交/回调、参数变更、取消及超时。新增 `backend/internal/interfaces/http/binding_generation_flow_test.go`，扩展 `backend/internal/infrastructure/sqlite/repository_test.go` 与处理器测试。
- [✅️] T039 [Docs] [RQ-005/RQ-007/RQ-009] 编写迁移、特性开关、备份恢复和蓝绿色界面使用说明；旧二进制回退需恢复备份，不直接覆盖新版数据库。新增 `docs/migrations/scene-character-binding.md`，更新 `README.md`；记录 payload 增长和参考图数量限制。
## Phase 10：自动化交互测试与基础验收

- [✅️] T040 [Test] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-006/RQ-007/RQ-008/RQ-009] 运行后端 go test ./...、前端 api:generate/build/lint/test/e2e；补验候选确认与完整生成流程，覆盖下方交互场景。自动化脚本必须实际点击、输入、切换并检查页面反馈及保存结果，不能只检查组件存在或截图一致。维护 `frontend/e2e/scene-character-binding.spec.ts`，在 `specs/scene-character-binding/verification.md` 记录命令、环境、结果及模拟/真实服务边界。自动化通过后仍须完成 T041～T043。

## Phase 11：执行者亲自操作并评估交互合理性

- [✅️] T041 [UX] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-006/RQ-007/RQ-008/RQ-009] 启动实际应用，由执行者使用浏览器亲自走完下方场景；从首次使用者角度评估入口、作用域、反馈、错误恢复和操作成本，不能只读代码或依赖自动化结果。逐项记录操作步骤、预期、实际、截图/证据、结论与问题 ID，产物为 `specs/scene-character-binding/interaction-review.md`；证据保存在 `specs/scene-character-binding/evidence/`，不含凭证。每个未通过或有明确改进点的场景均进入 T042。

## Phase 12：自主修正与交互复测

- [✅️] T042 [UX/UI] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-006/RQ-007/RQ-008/RQ-009] 根据 T041 发现的问题，自主修正文案、布局、焦点、控件状态、反馈、保存、引用或恢复逻辑；涉及接口与后端的真实行为问题同步修复，不通过弱化断言或隐藏错误让测试通过。修改 `frontend/src/components/studio/`、`frontend/src/views/StudioView.vue`、`frontend/src/composables/useShotEditor.ts` 及问题实际涉及的后端文件，补必要回归测试。每次修改后重新实际操作失败场景和相邻流程，更新 `specs/scene-character-binding/interaction-review.md` 中的问题、改动、复测证据与结果；复测发现新问题继续循环。未发现问题时记录走查依据并注明无需改动，不为完成任务强行修改。

## Phase 13：修正后的最终验收与交付

- [✅️] T043 [Build/Test] [RQ-001/RQ-002/RQ-003/RQ-004/RQ-005/RQ-006/RQ-007/RQ-008/RQ-009] 在全部交互修正完成后，运行受影响的单元/集成/E2E 测试和构建；再实际走一遍“创建分组→多图与描述→@引用→人物音色与台词→生成→选用→成片”的主流程。确认无未解决的阻断问题、无数据丢失/串绑/误提交，其他改进意见均有明确处理结论；在 `specs/scene-character-binding/verification.md` 汇总最终版本、测试记录、交互评估和外部能力限制。未能执行的场景必须标记未验证，不能勾选为全部通过。

### 必测交互场景与合理性标准

| 场景 | 实际操作 | 合理性与验收标准 |
|------|----------|------------------|
| 空白镜头首次使用 | 从空状态添加场景、人物，填写名称和描述 | 不需要阅读实现文档即可找到入口；下一步明确，必填缺失就地提示 |
| 多场景、多人物、多图 | 添加两个场景组、两个人物，每组添加多张图，排序、预览、移除并重新打开 | 分组归属和图片数量清楚；移除绑定不误删素材；排序及文本持久化 |
| 设定编辑作用域 | 同一人物用于两个镜头；修改本镜头补充，再修改项目设定 | 修改范围在操作前可见；另一个镜头不被隐式覆盖；版本更新和过期提示可理解 |
| @引用 | 键盘选择、中文输入、复制粘贴、删除引用、重名与改名 | 光标/输入法正常；不串人物；无失效标签；失效时提供可操作的修复入口 |
| 台词与音色 | 添加两人台词、重排、改说话者、覆盖音色、恢复继承、连续试听 | 每句对应人物清楚；继承/覆盖可区分；音频不混乱叠播；离开页面停止试听 |
| 自动保存与切换 | 快速输入后切镜头、刷新，保存中继续编辑；模拟网络失败/版本冲突 | 明确显示未保存/保存中/失败；不丢文字；提供重试/冲突处理且不覆盖新输入 |
| 上传失败与约束 | 多图中一张上传失败，尝试超数量或错误格式 | 精确指出失败图片；可以单独重试；不重复上传成功项；限制提前显示 |
| 生成提交与反馈 | 缺音色、失效引用、超模型限制时点击生成；修复后再次提交；快速双击 | 告知具体原因和修复位置；参数不被静默修改；重复点击不会重复创建任务 |
| 等待、取消与恢复 | 生成后离开再返回/刷新，取消及重试，选择候选首帧 | 进度和下一步清楚；等待人工选用不伪装为卡住；重试沿用明确的输入 |
| AI 候选与历史 | 查看/修正候选实体，应用分镜，复制镜头，恢复旧版 | 预览和应用行为清楚；引用、台词、素材正确；恢复不影响无关草稿 |
| 配音时长与成片 | 配音超出镜头长度，修改后生成并播放成片 | 提供可行调整方式，不静默截断；两人不串音色，避免原人声与配音重复 |
| 视觉与可访问性 | 桌面及窄屏、125%/150%缩放、键盘 Tab/Enter/Escape 操作 | 蓝绿色主色一致；文字可读、焦点可见、按钮可触达；弹层关闭后焦点合理；不只用颜色表达状态 |

执行要求：自动化通过只是技术检查，T041 必须实际观察与评估“是否好理解、是否顺手、是否能恢复错误”。用户已授权对本功能内不合理的交互直接优化，无需为常规布局、文案和流程修正再次征求确认；保持多图、描述、人物音色、@引用与蓝绿色主题等已确认需求。涉及新增产品范围、破坏性数据操作或无法判断的业务取舍时另行说明。

外部模型暂不可用时，可以模拟错误响应验证错误交互，但不得以模拟生成结果替代真实多图/配音/视频验收。问题记录使用“待修复→已修改→复测通过”，仅修复而未复测不得关闭；浏览器无法使用或实际未走查时 T041 保持未完成。

---

## 需求覆盖与当前状态

| 需求 | 主要任务 |
|------|----------|
| RQ-001 多场景多图及描述 | T005/T011/T015/T019/T020/T021/T025 |
| RQ-002 多人物多图及描述 | T005/T011/T015/T019/T020/T025 |
| RQ-003 结构化 @ 引用 | T006/T016/T020/T021/T025/T033 |
| RQ-004 人物音色和台词配音 | T003/T006/T008/T014/T017/T022/T027/T029/T035 |
| RQ-005 工作台布局与交互 | T004/T009/T015～T018/T025/T031/T034/T037 |
| RQ-006 真实生成输入与能力 | T001/T003/T008/T012/T013/T023/T024/T027/T030/T035 |
| RQ-007 版本可靠性与迁移 | T005～T007/T011/T019/T020/T024/T028/T033/T038/T039 |
| RQ-008 自动分镜关联 | T001/T032/T033/T036 |
| RQ-009 蓝绿色主题 | T004/T009/T025/T037/T039 |

交互闭环覆盖全部需求：T040 自动化交互测试 → T041 亲自走查 → T042 自主修正与复测 → T043 最终验收。

2026-09-29：Phase 1～13、T001～T043 均完成。后端全量测试、前端类型生成/lint/build、13 个单元测试、20 个 E2E 通过；浏览器走查发现的问题已修正并复测。两次实际多图/双音色/视频/成片流程均成功，最终六秒 720P 成片可播放，真实视频任务重启后沿用相同云端 ID。详见 verification.md、interaction-review.md 和 evidence/final-acceptance.json。主观听感、系统输入法候选窗、浏览器菜单缩放和未配置供应商不属于已验证能力，限制已明确记录。
