# 任务清单：故事、分镜与素材创作工作台

**Spec**: `specs/story-storyboard-generation/spec.md`
**Plan**: `specs/story-storyboard-generation/plan.md`
**Created**: 2026-09-23
**项目识别结果**: Go/Gin 分层后端 + SQLite JSON 持久化 + Vue 3/Pinia 前端；百炼模型与 FFmpeg 成片
**技术链依赖**: Phase 1 明确 API、迁移与媒体验收契约；Phase 2 使用 Go 领域建模和 SQLite 迁移；Phase 3 使用 Go 应用服务、百炼文本协议及 Vue/Pinia；Phase 4 使用素材绑定、模型能力验证和媒体传输；Phase 5 使用异步任务、百炼视频协议与本地文件持久化；Phase 6 使用 FFmpeg/ffprobe 和成片任务；Phase 7 使用 Go、Vitest、Playwright、OpenAPI 与 Windows/容器发行验证。实施阶段可按 `speckit-implement` 逐 Phase 执行。

---

## 格式说明

- `[TaskID]`：全局递增唯一编号；勾选表示该任务及其验收已完成。
- `[技术域]`：主要负责的架构层；`[RQ-xxx]` 对应 `spec.md` 的需求。
- 每条任务给出主要文件路径或明确产物，并以“验收”说明完成条件。
- Phase 按顺序实施。需要真实凭据或部署环境的冒烟列为发布验收，不妨碍前面阶段使用桩与本地媒体完成开发。

---

## Phase 1：协议与迁移基线

- [✅️] T001 [Contract] [RQ-101] [RQ-105] [RQ-107] [RQ-111] [RQ-113] 定稿草稿版本、候选、镜头绑定、任务版本与成片的请求/响应、错误码和作用域，更新 `docs/api/openapi.yaml`；验收：每个写操作都定义期望版本或幂等键，旧任务/项目接口仍可读取。
- [✅️] T002 [DB] [RQ-114] [RQ-117] 收集旧 visual/character/frame/voice/music 素材、项目和任务 JSON 的回归样本，写出备份、迁移、重复执行和恢复步骤，产物为 `backend/internal/infrastructure/sqlite/testdata/legacy/`、`docs/migrations/story-storyboard-generation.md`；验收：样本覆盖旧首尾帧文件、明确用途、无镜头归属任务及跨草稿数据。
- [✅️] T003 [Provider] [RQ-108] [RQ-109] [RQ-110] 将 Wan 2.7 的首帧/尾帧/驱动音频、2～15 秒、720P/1080P 及文件限制固化为可测试能力清单，产物为 `backend/internal/domain/provider/video_capability.go`、`backend/internal/domain/provider/video_capability_test.go`；验收：未知模型被判为不可提交，已绑定的不支持用途返回明确原因。
- [✅️] T004 [Config] [RQ-109] [RQ-113] 定义 FFmpeg/ffprobe 路径、媒体工作目录与对象存储音频交付的配置及启动检查，产物为 `backend/cmd/server/main.go`、`docs/windows-install.md`；验收：缺少可执行文件或参考音频传输配置时可区分受影响能力，编辑仍可用。

## Phase 2：版本化领域与持久化基础

- [✅️] T005 [Domain/DB] [RQ-101] [RQ-102] [RQ-118] 为草稿增加版本条件保存、正文纯文本更新和原子冲突返回，修改 `backend/internal/domain/project/project.go`、`backend/internal/application/project_service.go`、`backend/internal/infrastructure/sqlite/task_repository.go`；验收：两个并发编辑不会静默覆盖，保存失败后新任务无法引用未保存内容。
- [✅️] T006 [Domain/DB] [RQ-103] [RQ-104] [RQ-105] 增加正文候选、分镜候选和整批替换恢复点的持久化模型，修改 `backend/internal/domain/story/story.go`、`backend/internal/domain/project/project.go`、`backend/internal/infrastructure/sqlite/task_repository.go`；验收：重启后候选与旧分镜快照仍可读取，候选不会自动改写草稿。
- [✅️] T007 [Domain/DB] [RQ-105] [RQ-106] 建立稳定镜头身份、连续排序、复制与删除的领域操作，扩展 `backend/internal/domain/story/story.go`、`backend/internal/domain/story/story_test.go`；验收：排序不改变绑定目标，复制不继承任务或视频版本，删除镜头不让旧结果回插。
- [✅️] T008 [DB] [RQ-114] [RQ-115] [RQ-116] [RQ-117] 增加五类素材、标签、媒体元数据及引用索引的版本化 SQLite 迁移，修改 `backend/internal/domain/material/material.go`、`backend/internal/infrastructure/sqlite/task_repository.go`、`backend/internal/infrastructure/sqlite/repository_test.go`；验收：保留旧 ID/文件/任务输入，visual→场景、frame→带“原首尾帧”标记的场景，重复迁移无副作用。
- [✅️] T009 [Domain/DB] [RQ-107] [RQ-108] [RQ-109] [RQ-111] [RQ-113] 建立镜头绑定用途、视频版本、已选版本及成片快照的持久化结构，修改 `backend/internal/domain/project/project.go`、`backend/internal/domain/task/task.go`、`backend/internal/infrastructure/sqlite/task_repository.go`；验收：不同项目/草稿/镜头间引用不串写，旧 JSON 无新字段仍能读取。

## Phase 3：正文、分镜与素材库

- [✅️] T010 [Service/Provider] [RQ-103] [RQ-104] [RQ-105] 分开实现故事联想与正文拆分镜的版本化 system/user 提示词、结构化解析和候选保存，修改 `backend/internal/domain/processor/processor.go`、`backend/internal/infrastructure/provider/bailian/client.go`、`backend/internal/application/task_result_applier.go`；验收：正文任务只产生正文候选，分镜任务只产生镜头候选，非法 JSON 或领域值不会应用草稿。
- [✅️] T011 [Service/API] [RQ-104] [RQ-105] 实现候选预览、追加/替换正文、撤销本次应用、确认替换分镜及恢复旧版本的版本条件操作，修改 `backend/internal/application/project_service.go`、`backend/internal/interfaces/http/router.go`；验收：生成期间的编辑触发冲突提示，旧镜头连同绑定与选用视频可恢复。
- [✅️] T012 [Service/API] [RQ-106] 实现镜头新增、编辑、复制、删除和排序接口，修改 `backend/internal/application/project_service.go`、`backend/internal/interfaces/http/router.go`；验收：标题、画面描述、旁白和时长可保存，复制/排序/删除遵循 T007 的身份规则。
- [✅️] T013 [UI] [RQ-101] [RQ-102] [RQ-118] 建立正文/分镜/视频页签及正文纯文本编辑、UTF-8 `.txt` 导入、自动保存和离开保护，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`；验收：可跳过 AI 直接写正文，纯空白不能生成分镜，未保存状态与失败重试可见。
- [✅️] T014 [UI] [RQ-103] [RQ-104] [RQ-105] 实现故事联想侧抽屉、候选预览与显式应用，以及分镜候选确认/恢复，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`；验收：关闭重开保留输入、预览和任务状态，候选不会自动覆盖正文或镜头。
- [✅️] T015 [UI] [RQ-105] [RQ-106] 实现可编辑镜头列表、选中检查面板、复制/排序/删除与旧版恢复入口，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`；验收：选中镜头和编辑在页签切换后保留，键盘上移/下移可替代拖拽。
- [✅️] T016 [Service/API] [RQ-114] [RQ-115] [RQ-116] [RQ-117] 实现素材上传、真实媒体信息提取、重命名/分类/标签、搜索筛选排序、使用位置及引用删除保护，修改 `backend/internal/application/material_upload.go`、`backend/internal/application/material_service.go`、`backend/internal/interfaces/http/router.go`；验收：图片尺寸和音频时长来自文件，删除时重新检查引用，解绑不删文件。
- [✅️] T017 [UI] [RQ-114] [RQ-115] [RQ-116] [RQ-117] 重建五类素材库的全部视图、筛选、上传进度、试听、详情和引用跳转，修改 `frontend/src/views/MaterialsView.vue`、`frontend/src/api/client.ts`；验收：无独立首尾帧分类或全局选用状态，旧素材保留“原首尾帧”来源提示。

## Phase 4：镜头绑定与提交能力

- [✅️] T018 [Service/API] [RQ-107] [RQ-108] [RQ-109] 实现按 project/draft/scene 确认图片和音频用途绑定、取消选择、解绑及首尾帧单选约束，修改 `backend/internal/application/project_service.go`、`backend/internal/interfaces/http/router.go`；验收：同一素材可在不同镜头用不同用途，取消选择不改现有绑定。
- [✅️] T019 [Service/API] [RQ-108] [RQ-109] [RQ-110] 将模型能力表接入可用模型查询和逐镜头提交校验，修改 `backend/internal/application/catalog_service.go`、`backend/internal/application/task_service.go`、`backend/internal/interfaces/http/router.go`；验收：数量、用途、时长、画幅、文件存在性错误精确到镜头和字段，未知能力不可提交。
- [✅️] T020 [Provider] [RQ-108] [RQ-109] [RQ-110] 扩展百炼图生视频请求映射为首帧、可选尾帧和驱动音频，并传递时长/清晰度，修改 `backend/internal/domain/provider/provider.go`、`backend/internal/infrastructure/provider/bailian/client.go`、`backend/internal/infrastructure/provider/bailian/protocol/wanxiang.go`；验收：请求体符合已核实的媒体组合，不支持的参考图不被默默忽略。
- [✅️] T021 [Media] [RQ-109] [RQ-110] 实现生成参考音频的对象存储 URL 交付与到期校验，修改 `backend/internal/infrastructure/provider/bailian/`、`backend/cmd/server/main.go`；验收：未配置时阻止该用途提交并给出配置原因，本地配音仍可用于后期合成。
- [✅️] T022 [UI] [RQ-107] [RQ-108] [RQ-109] [RQ-110] 实现镜头内素材选择/上传视图、用途卡片、能力提示和逐字段错误，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`；验收：选择确认才绑定；模型变化后不兼容绑定仍保留但阻止生成。

## Phase 5：视频任务与历史版本

- [✅️] T023 [Service/API] [RQ-110] [RQ-111] 实现单镜头及批量任务创建、提交摘要、部分失败名单和只重试失败项，修改 `backend/internal/application/task_service.go`、`backend/internal/domain/task/task.go`、`backend/internal/interfaces/http/router.go`；验收：任务固定项目/草稿/镜头/输入快照，重复提交不创建重复任务。
- [✅️] T024 [Provider/Media] [RQ-111] [RQ-112] 在成功回调后下载 24 小时有效的远端视频、校验并原子保存本地文件，再创建镜头版本，修改 `backend/internal/domain/processor/processor.go`、`backend/internal/infrastructure/provider/results.go`、`backend/internal/application/task_result_applier.go`；验收：下载失败可重试，不出现指向临时 URL 或缺失文件的成功版本。
- [✅️] T025 [Service/API] [RQ-111] [RQ-112] 实现视频版本列表、选用、输入指纹与“基于旧设置”判定，修改 `backend/internal/application/project_service.go`、`backend/internal/interfaces/http/router.go`；验收：晚到和重复回调不覆盖新编辑或已选版本，已删除镜头结果仅在任务中心可见。
- [✅️] T026 [UI] [RQ-110] [RQ-111] [RQ-112] 实现单镜头/批量生成状态、失败重试、历史版本预览和显式选用，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/tasks.ts`、`frontend/src/views/TasksView.vue`；验收：新版本到达只提示，旧视频可播放/下载，正文变更仅标记分镜待复核。

## Phase 6：成片合成与视频页

- [✅️] T027 [Service/API] [RQ-109] [RQ-113] 建立成片提交校验和不可变输入快照，修改 `backend/internal/application/task_service.go`、`backend/internal/domain/task/task.go`、`backend/internal/interfaces/http/router.go`；验收：每镜头必须有有效已选版本，缺片段、超长配音或不可用音频时拒绝并指出镜头。
- [✅️] T028 [Media] [RQ-109] [RQ-113] 实现 FFmpeg/ffprobe 规范化、等比留边拼接、镜头配音与背景音乐循环/音量混合，产物为 `backend/internal/infrastructure/media/`；验收：异画幅片段顺序、真实时长和音轨均正确，失败清理临时文件。
- [✅️] T029 [Service/API] [RQ-113] 实现成片任务执行、进度、结果保留和本地下载，修改 `backend/internal/domain/processor/processor.go`、`backend/internal/application/task_result_applier.go`、`backend/internal/application/task_service.go`；验收：提交后编辑不改变执行中输入，旧成片仍可预览下载并显示未合成修改。
- [✅️] T030 [UI] [RQ-109] [RQ-113] 实现视频页播放器、镜头顺序条、配乐/音量/输出设置、缺失镜头跳转与成片下载，修改 `frontend/src/views/StudioView.vue`、`frontend/src/stores/workspace.ts`；验收：顺序与分镜一致，无有效片段不能导出完整成片。
- [✅️] T031 [Config] [RQ-113] 将 FFmpeg/ffprobe 纳入容器镜像和 Windows 发行包，修改 `backend/Dockerfile`、`scripts/build-windows.ps1`、`docs/windows-install.md`；验收：两种交付方式启动检查通过，可对本地样本完成一次真实合成。

## Phase 7：跨流程验收与交付

- [✅️] T032 [API] [RQ-101] [RQ-107] [RQ-111] [RQ-113] 对齐全部新增接口的 OpenAPI 与前端类型，更新 `docs/api/openapi.yaml`、`frontend/src/api/types.ts`、`frontend/src/api/schema.d.ts`、`frontend/src/api/client.ts`；验收：类型生成后无手改漂移，旧客户端基础读取兼容。
- [✅️] T033 [Test] [RQ-104] [RQ-105] [RQ-111] [RQ-112] 验证候选并发应用、旧分镜恢复、跨草稿任务、晚到/重复回调和已选版本保护，补充 `backend/internal/interfaces/http/server_test.go`、`backend/internal/infrastructure/sqlite/repository_test.go`、`frontend/e2e/core.spec.ts`；验收：这些场景均无静默覆盖或串写。
- [✅️] T034 [Test] [RQ-108] [RQ-109] [RQ-110] [RQ-113] 用本地媒体验证模型能力拦截、参考音频配置降级、远端结果转存和 FFmpeg 成片，补充 `backend/internal/infrastructure/provider/bailian/client_test.go`、`backend/internal/infrastructure/media/`、`frontend/e2e/task-results.spec.ts`；验收：失败返回可读的镜头/字段定位且可重试。
- [✅️] T035 [Test] [RQ-114] [RQ-115] [RQ-116] [RQ-117] 在旧数据库副本上演练迁移、重跑、失败回滚和素材引用删除保护，产物为 `backend/internal/infrastructure/sqlite/repository_test.go`、`docs/migrations/story-storyboard-generation.md`；验收：素材 ID/文件、旧任务输入与明确镜头用途不变。
- [✅️] T036 [UI/Test] [RQ-118] [RQ-119] [RQ-120] 验收保存/加载/上传/生成失败、窄屏单面板、键盘排序与抽屉焦点恢复，并按交互设计核对五张概念图，修改 `frontend/e2e/core.spec.ts`、`frontend/src/views/StudioView.vue`、`frontend/src/views/MaterialsView.vue`；验收：错误保留重试入口，状态不只靠颜色。
- [✅️] T037 [Release] [RQ-110] [RQ-113] 运行 Go 测试与 vet、前端 lint/test/build/e2e，并在有凭据环境完成真实故事/视频模型、对象存储及容器/Windows 成片冒烟，记录结果于 `docs/release.md`；验收：全部通过，或逐项记录受环境限制的未完成门槛，不把未验证能力标为已交付。
