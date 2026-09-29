# 实施计划：分镜场景、人物与音色绑定工作台

**Feature Branch**: 未创建；本次仅规划
**Created**: 2026-09-28
**Status**: Draft
**Spec**: 尚无独立 spec.md；依据本次对话需求及用户确认的暗色工作台设计图

---

## 摘要

将当前按素材用途绑定的分镜编辑器，扩展为按场景和人物分组的工作台。每组包含名称、可编辑文字描述、多张参考图；人物附带默认音色。当前镜头通过结构化 @ 引用使用这些设定，台词逐句绑定说话者，并将最终解析输入送入真实生成链路。

以下 RQ 为本计划对对话需求的映射，非已存在的 spec 编号：

- RQ-001：一个镜头可关联多个场景组，每个场景组支持描述及多张小尺寸参考图。
- RQ-002：一个镜头可关联多个人物组，每个人物支持描述、多张参考图与音色。
- RQ-003：镜头描述使用 @ 引用明确实体身份；重命名后引用仍有效。
- RQ-004：台词按人物绑定音色，支持试听、配音生成及成片合成。
- RQ-005：复现确认图的暗色布局、紧凑素材卡片、小预览与生成入口。
- RQ-006：文字、参考图、音色真正进入生成请求，能力不支持时明确阻止或提示可用路径。
- RQ-007：跨镜头复用、版本追踪、旧数据兼容、任务重试及并发保存可靠。
- RQ-008：故事生成分镜时能输出实体与台词候选，用户确认后建立绑定。
- RQ-009：主题统一为蓝绿色（teal）；工作台保留暗色底，按钮、选中态、焦点和引用强调使用统一色板，人物辅助色不替代品牌主色。

## 项目适配输入

- 项目类型: Vue 3 + TypeScript + Pinia 前端；Go/Gin 分层后端；SQLite；异步任务与媒体合成。
- 结构约束: 沿用 domain/application/infrastructure/interfaces；保留现有 Scene 作为镜头的代码名称，新增 Location 表示故事地点；不为此引入微服务。
- 交付物类型: 领域模型、迁移、OpenAPI、厂商适配、工作台组件、回归测试、迁移说明。
- 特殊验证要求: OpenAPI 类型生成、前端构建与交互验证、Go 测试、迁移与真实厂商联调。
- 技能: 本计划使用 speckit-plan；后续可用 speckit-specify 补规范、speckit-tasks 拆任务、speckit-implement 实施及 speckit-code-review 复审。当前目录未发现适用 AGENTS.md 或宪法文件；未加载不存在的框架技能。
- 本阶段只维护计划和任务文档，不修改业务代码。2026-09-28 已使用用户指定服务完成模型列表与 qwen-plus 文本结构化分镜实测；尚未验证多图、生图、视频与 TTS。详见 provider-test.md，不把模型出现在列表中等同于能力已通过。

---

## Phase 0：调研与关键决策

### 实体与关系分析

| 实体 | 关键属性 | 与其他实体关系 | 来源 RQ |
|------|---------|---------------|---------|
| Location | id、名称、描述、revision、references[] | 项目级身份；多个镜头引用某个版本 | RQ-001/007 |
| Character | id、名称、描述、revision、references[]、voice_profile_id | 项目级身份；被出场绑定及台词引用 | RQ-002/004 |
| ReferenceImage | material_id、内容哈希、视角说明、顺序 | 归属某个实体版本；文件仍由素材库管理 | RQ-001/002 |
| VoiceProfile | id、revision、connection_id、model_id、remote_voice_id、语言、默认参数 | 人物默认使用；台词可以显式覆盖 | RQ-004 |
| ShotEntityBinding | binding_id、entity_type、entity_id、entity_revision、局部描述、选用参考图 | 镜头关联多个场景/人物；用固定版本隔离后续档案编辑 | RQ-001/002/007 |
| PromptDocument | 文本片段与实体引用节点 | 引用绑定 ID，不通过名字正则猜测身份 | RQ-003 |
| Utterance | id、speaker_id、文本、顺序、情绪、音色覆盖、停顿 | 属于镜头；说话者为人物或旁白，允许画外人物 | RQ-004 |
| AudioTake | 台词 ID、任务 ID、输入哈希、音频素材、实测时长 | 一句台词可有多个候选配音；明确选用版本 | RQ-004/007 |
| GenerationSnapshot | 实体版本、文本、参考素材、音色、模型、参数、输入哈希 | 任务持有不可变输入；生成结果追溯该输入 | RQ-006/007 |

### 状态流转（若适用）

| 实体 | 状态列表 | 流转规则 | 来源 RQ |
|------|---------|---------|---------|
| 编辑内容 | clean / dirty / saving / conflict / failed | 防抖保存；失败保留本地编辑；冲突显式处理 | RQ-005/007 |
| 实体绑定 | valid / update_available / missing / unresolved | 修改档案产生新版本，镜头保持旧版并提示可升级；缺失引用阻止相关生成 | RQ-007 |
| 生成任务 | 复用 queued/running/succeeded/failed/cancelled | 重试使用原快照；输入更改必须创建新任务 | RQ-006/007 |
| 镜头制作流程 | preparing / generating / awaiting_selection / completed / failed / cancelled | 依次准备配音、首帧与视频；保存子任务关系，可恢复，单步失败不重复全部任务 | RQ-004/006 |
| 生成结果 | current / stale | 任务成功状态与结果过期状态分开；只标记受依赖更改影响的结果 | RQ-007 |
| AI 候选 | preview / applied / rejected | 用户应用候选时校验 ID 和版本，不直接覆盖编辑内容 | RQ-008 |

### 依赖调研

| 依赖对象 | 用途 | 已有能力 | 需要新增或修改 |
|----------|------|----------|----------------|
| domain/story/story.go、generation.go | 分镜及生成契约 | 镜头描述、narration、时长，严格 JSON 解析 | 实体候选、结构化引用、台词，升级提示词与解析版本 |
| domain/project/project.go | 草稿与绑定 | SceneBinding、视频版本、分镜快照 | 项目设定版本、镜头实体绑定、音频候选、快照覆盖新增数据 |
| application/project_bindings.go | 素材绑定 | 用途和素材类型校验、草稿版本锁 | 按实体归属及多图校验；保留首尾帧等用途绑定 |
| application/video_tasks.go、video_validation.go | 视频提交 | 首尾帧和驱动音频；当前代码拒绝三类参考图 | 统一解析输入、按适配能力选择路径，禁止静默丢弃 |
| domain/provider、infrastructure/provider | 厂商协议 | story/image/video；ImageRequest 目前只有文本等字段 | 多图与语义映射、speech 能力、TTS 适配、音色目录 |
| infrastructure/sqlite | 持久化 | projects/tasks/materials JSON payload；material_references 引用保护 | 版本化设定、快照依赖及引用索引事务维护、迁移版本 |
| infrastructure/media、composition_service.go | 合成 | 镜头视频、单份后期音轨、音乐混音 | 多句配音时间安排，混为镜头音轨，实测时长校验 |
| StudioView.vue、workspace.ts | 工作台 | 集中页面、自动保存、素材选择、任务回填 | 组件拆分、多选图、描述编辑、结构化 @ 编辑、字段级脏状态 |
| docs/api/openapi.yaml、frontend/src/api | API 契约 | 生成 schema.d.ts；部分类型仍手写 | 新契约及统一类型来源，更新客户端 |

### 调研结论

- **DR-001：区分档案与镜头**
  - 决策: 项目拥有场景/人物档案及历史版本；镜头固定引用版本，支持明确标注的本镜头局部描述覆盖。
  - 理由: 跨镜头复用并避免修改一个人物意外改变其他草稿。卡片默认编辑本镜头补充设定；“编辑项目设定”显式产生新版本并展示影响范围。
  - 排除方案: 仅在 SceneBinding 增加名字字段，无法表达身份、版本和跨镜头复用。
- **DR-002：结构化 @ 引用**
  - 决策: 存 text/entity_ref 片段，引用稳定 binding_id；显示名称由实体解析。保留 visual_prompt 作为兼容渲染文本，新接口以结构化文档为准。
  - 理由: 支持重名、中文输入、改名、复制粘贴和引用删除检查。
  - 排除方案: 只存字符串并用正则替换 @名字。
- **DR-003：增量持久化**
  - 决策: 第一版档案历史及镜头数据存入现有 project payload，项目设定单独 catalog_version，草稿继续 draft.version；新增快照依赖索引，扩展现有素材引用保护。
  - 理由: 匹配当前聚合仓储，避免同时重写整个存储层。所有写入通过同一事务维护 payload 和引用索引；测量 payload 增长后再决定拆表。
  - 排除方案: 一次性迁移所有草稿、素材和视频到全新规范化表。
- **DR-004：模型能力驱动**
  - 决策: 多参考图直接生成与“多图生首帧→图生视频”两条路径均有明确能力声明，至少打通其中一条可实用链路；TTS 单独适配。
  - 理由: 当前仓库视频适配只接受首帧、尾帧和驱动音频，生图接口也未支持参考图。
  - 排除方案: 删除校验限制后假定图片已被模型使用；仅把图片文件名加入提示词。
- **DR-005：首版界面范围**
  - 决策: 实现暗色点阵背景和固定面板布局，支持卡片折叠、图片排序、滚动与响应式；不实现无限画布、节点拖拽或自由连线。
  - 理由: 当前目标是清晰编辑并真实生成；图中的端口和装饰不应暗示尚不可用的操作。
  - 排除方案: 第一版引入完整节点编辑框架。
- **DR-006：多个场景组的语义**
  - 决策: 多图可描述同一地点的角度；多个地点组可作为主场景/补充环境参考。明显涉及地点切换的描述提示拆镜头，避免把多个地点自动拼为同一空间。
  - 理由: 支持多绑定，同时明确各参考的作用。

---

## Phase 1：技术设计

### 模块设计

| 模块 | 职责 | 主要输入 | 主要输出 | 来源 RQ |
|------|------|---------|---------|---------|
| domain/project 设定模型 | 档案版本与镜头引用规则 | 档案编辑、绑定修改 | 新版本、引用校验 | RQ-001/002/007 |
| application 设定服务 | 档案 CRUD、版本升级、影响分析 | 项目与版本锁 | 档案/受影响镜头 | RQ-007 |
| application BindingResolver | 合并档案、局部设定、参考图、音色 | 镜头及版本 | 有序标准输入、字段问题清单 | RQ-003/006 |
| application GenerationCompiler | 按模型协议编译实际请求 | 标准输入、能力信息 | 提示词、媒体顺序、快照哈希 | RQ-006 |
| provider image/video/speech | 执行厂商请求 | 已编译输入 | 生图、视频、音频 | RQ-004/006 |
| task/processor/result_applier | 排队、恢复、回填、幂等 | 快照、阶段依赖 | 候选图/音轨/视频版本 | RQ-006/007 |
| media 音轨准备 | 按顺序、停顿或显式起点混音 | 已选 AudioTake | 镜头音轨、时长 | RQ-004 |
| StudioView | 路由与镜头切换、整体布局 | 草稿状态 | 编辑工作台 | RQ-005 |
| EntityReferenceCard / ReferenceImageList | 描述、图片多选、移除、排序、视角备注 | 实体绑定 | 编辑事件 | RQ-001/002 |
| MentionPromptEditor | @候选、稳定引用节点、中文输入处理 | 文档、可引用档案 | 结构化文档 | RQ-003 |
| DialogueVoicePanel / VoiceSelector | 台词 CRUD、音色继承、试听 | 人物、音色、台词 | 台词更新、配音任务 | RQ-004 |
| ShotPreview / GenerationToolbar | 小预览、候选选用、能力提示、进度 | 任务与版本 | 生成/选用操作 | RQ-005/006 |
| workspace 与编辑 composables | 服务端状态、本地脏字段、串行保存、任务合并 | API/WebSocket/用户编辑 | 不丢编辑的状态更新 | RQ-007 |

前端具体改造：

1. 从 StudioView.vue 拆出上述组件，样式限定在工作台范围；不覆盖素材库/配置中心等页面的全局样式。
2. 每组名称、描述、图片数量、缩略图、添加按钮、折叠与操作菜单；图片支持多选上传、独立上传进度、失败重试、删除绑定和大图预览。
3. 人物卡的音色代表默认音色；台词处展示“继承”或“已覆盖”，避免两处设置语义不明。
4. 卡片新增“本镜头补充”和“项目设定”作用域提示；升级档案版本前列出变化，旧结果仅标记过期。
5. @ 下拉按场景/人物分组，支持键盘选择；未绑定实体被选择时原子创建绑定；删除引用不能意外删除档案或素材。
6. 提交生成前刷新待保存编辑，失败则不提交；WebSocket 回填仅合并任务结果，不覆盖正在输入的描述和台词。
7. 小屏改为顺序堆叠；图片行可横向滚动；焦点、按钮名称、对比度与中文输入法行为纳入验收。
8. ConfigView 增加语音模型配置；MaterialsView 支持图片视角信息、音频时长及引用来源；TasksView 增加配音和父子任务展示。
9. 蓝绿色主题令牌集中定义于 frontend/src/style.css：建议暗色主色 #14B8A6、悬停 #2DD4BF、按下 #0D9488；亮色页面按钮用 #0F766E 搭配白字，暗色按钮优先深色字。将旧 --accent/--tint 映射到新令牌，分别验证两种背景的文字对比度，键盘焦点不只靠颜色区分。工作台深色背景限定作用域，不强制把其他页面一起变暗。

### 协议 / 接口设计（若适用）

下表路径均为建议，实施前统一写入 OpenAPI。前缀 P 为 `/api/projects/:id`，S 为 `P/drafts/:draftId/scenes/:sceneId`。

| 服务或模块 | 方法或动作 | 请求关键字段 | 响应关键字段 | 来源 RQ |
|------------|-----------|-------------|-------------|---------|
| 设定库 | GET/POST P/entities；PATCH/DELETE P/entities/:entityId | kind/name/description/references、expected_catalog_version | 实体版本、catalog_version、使用位置 | RQ-001/002/007 |
| 镜头绑定 | PUT S/entity-bindings | bindings、expected_version | draft、新版本 | RQ-001/002 |
| 镜头编辑 | 扩展 PATCH S | prompt_document、台词、局部描述、expected_version | draft、新版本 | RQ-003/004 |
| 音色 | GET /api/models/:model/voices；项目内 voice-profiles CRUD | connection/model/remote_voice_id、参数 | 可用音色/配置版本 | RQ-004 |
| 能力目录 | 扩展 /api/models/:model 能力查询 | 输入类型 | 图片数量/总量/顺序/音频/画幅/时长限制、适配版本 | RQ-006 |
| 输入预览 | POST S/generation-preview | model_id、路径、expected_version、输出参数 | 解析输入、实际使用素材、问题、fingerprint | RQ-006 |
| 生图 | POST S/image-tasks | expected_version、model_id、idempotency_key | task_id、snapshot_id | RQ-006 |
| 配音及试听 | POST S/utterances/:utteranceId/speech-tasks | expected_version、参数、idempotency_key | task_id、snapshot_id | RQ-004 |
| 视频/制作流程 | 扩展 S/video-tasks，新增 S/generation-runs | expected_version、路径、idempotency_key | task/run、阶段与依赖 | RQ-006/007 |
| 结果选用 | 扩展镜头候选图与音频选用接口 | candidate_id、expected_version | 更新后镜头 | RQ-004/007 |

写接口检查项目归属、实体类型、版本存在、图片类型及绑定归属。实体被引用时归档或拒绝硬删，并返回使用位置。重复幂等键须核对完整请求指纹；不同分辨率等参数不能错误复用旧任务。

### 数据或配置设计（若适用）

| 对象 | 字段或配置项 | 类型 | 约束 | 说明 |
|------|--------------|------|------|------|
| Project | catalog_version、entities、voice_profiles | 版本号与版本化集合 | 与草稿版本分别校验，仓储事务串行更新 | 跨草稿复用身份 |
| Scene | entity_bindings、prompt_document、utterances | 数组/文档 | 引用均可解析，服务端生成 ID | 兼容旧 visual_prompt/narration |
| ReferenceImage | material_id、content_hash、label、order | 结构体 | 稳定顺序、同组不重复、素材内容不可原位替换 | 同一素材可被不同组引用 |
| ShotEntityBinding | entity_revision、description_override、selected_references、role | 结构体 | 区分继承与显式空覆盖 | 未覆盖默认使用固定版本的引用图 |
| PromptDocument | schema_version、nodes | JSON | text/entity_ref；实体节点引用 binding_id | 标签显示值不作为身份依据 |
| Utterance | speaker_type、speaker_id、voice_override、timing | 结构体 | 旁白或合法人物；只在配音任务需要时要求有效音色 | 无台词镜头允许无音色 |
| GenerationSnapshot | schema_version、resolved_input、provider_input、hash | JSON | 不可变；排除密钥与临时签名 URL | 保存顺序、内容哈希、模型和适配版本 |
| Task | kind=speech、snapshot_id、run_id、depends_on | 扩展字段 | 恢复与重试只读快照 | 保持现有任务状态枚举 |
| 素材引用索引 | source_kind/source_id/material_id | 索引关系 | 覆盖实体历史、分镜快照、生成快照和音频 | 防止被运行任务及可恢复历史引用的素材被删 |

生成链路：保存编辑 → 同一一致性读取解析实体版本 → 模型能力校验 → 固化快照 → 创建任务 → 持久化结果 → 用户选用 → 合成。

- 快照保存与任务创建须原子完成，或采用可恢复的持久待入队状态；不能任务已入队而快照尚未保存。定期恢复未入队任务，重复回填按任务 ID 幂等。
- 图片、音频资源以稳定 material_id 和内容哈希定位；提交厂商时生成可访问的临时 URL，URL 过期不改变输入身份。
- 两阶段路径中，参考图只进入生图，选定的首帧进入视频；UI 分别展示两阶段实际使用的输入。
- 配音按实测音频时长安排顺序和停顿；超出镜头时长时提示延长/调整/重生成，不能静默裁剪。第一版先混为单份镜头 voiceover 复用现有合成。
- 若走驱动音频路径，先完成配音再创建视频任务；若走后期配音，可独立制作后汇合。后期配音不承诺口型同步；混音需明确原片人声是否保留，避免双重说话声。
- 素材与描述冲突时保留可检查的解析预览，由用户修正；不宣称单靠结构化绑定可保证模型画面一致。
- 新建或替换分镜候选时沿用档案身份，仅创建镜头引用；旧快照必须包含新增引用/台词/选用音轨。复制镜头时重建镜头内绑定与台词 ID，并重映射 @ 节点。

#### 实施顺序与交付门槛

| 阶段 | 后端工作 | 前端工作 | 完成标准与依赖 |
|------|----------|----------|----------------|
| P0 契约与能力验证 | 固定领域边界、OpenAPI 草案；核对选定厂商官方协议并做最小多图/TTS 请求验证 | 明确卡片作用域、编辑状态、错误展示 | 至少确定一条可行多图生成路径及一个 TTS 实现；可与无厂商依赖的数据工作交错推进 |
| P1 数据与迁移 | 实体/音色版本、镜头绑定、引用索引、CRUD、锁和旧数据迁移 | 更新 API 类型、store、接口模拟 | 多场景、多人物、多图、描述能保存、刷新恢复；引用不会串档案 |
| P2 工作台交互 | 支持原子镜头更新、影响查询 | 拆组件并完成设计图、@ 编辑、多图上传排序、台词和音色选择 | 用户可完成全部编辑操作；此阶段不宣称生成链路完成 |
| P3 多图视觉链路 | Resolver/Compiler、快照、生图适配、候选首帧、视频任务与能力校验 | 输入预览、候选选用、生成路径、阶段进度 | 抓取实际请求确认文字与每张选用图送达；至少一个真实模型生成可播放视频 |
| P4 音色与配音 | speech provider/processor、音色目录、AudioTake、顺序混音及合成 | 试听、台词继承/覆盖、音频版本与超时长提示 | 两人不同音色且台词不串绑；成片可听；口型同步按独立能力标示 |
| P5 自动分镜关联 | 升级 generation.go 契约、实体候选与消歧、应用/恢复/复制逻辑 | 候选确认与未识别引用修复 | 从故事生成分镜后档案与台词引用可确认，历史数据可恢复 |
| P6 联调与发布 | 恢复/幂等/迁移/引用保护测试、文档、特性开关 | E2E、响应式、冲突与错误恢复 | 完整验收、升级演练及旧流程回归通过 |

依赖关系：P0 契约 → P1 → P2；P1 与已验证的模型适配 → P3/P4；P5 依赖稳定实体与分镜契约；P6 汇总。前后端可在契约固定后分别开发，不需要等待全部厂商适配才做卡片。P3/P4 未完成不能以静态界面代替功能验收。

### 错误 / 枚举 / 文案（若适用）

| 类型 | 名称 | 说明 | 来源 RQ |
|------|------|------|---------|
| Error | entity_revision_conflict / draft_version_conflict | 返回最新版本，保留本地编辑供比较 | RQ-007 |
| Error | reference_missing / entity_in_use | 标记具体分组/图片，删除返回引用位置 | RQ-001/007 |
| Error | reference_limit_exceeded / capability_unsupported | 显示模型限制及可选生成路径，不自动丢图 | RQ-006 |
| Error | speaker_unresolved / voice_unavailable | 标明对应台词；不随意替换音色 | RQ-004 |
| Error | audio_exceeds_duration | 明确实际音频与镜头时长差值 | RQ-004 |
| Error | idempotency_key_reused | 同一键对应不同输入时拒绝 | RQ-007 |
| Enum | entity_type=location/character；speaker_type=character/narrator | 档案与说话者类型分开 | RQ-001/002/004 |
| UI | “本镜头补充”“项目设定有更新”“引用已失效”“结果基于旧设定” | 明确作用范围与下一步操作 | RQ-005/007 |

---

## 测试与验证

### 测试目标

| 目标 | 层次 | 类型 | 优先级 | 来源 RQ | 说明 |
|------|------|------|--------|---------|------|
| 两人物同名/改名后引用仍准确 | domain + UI | Unit/E2E | P0 | RQ-003 | ID 与显示名独立 |
| 同组多图顺序与不同组语义映射 | compiler/provider | Integration | P0 | RQ-001/002/006 | 检查实际 HTTP 请求而非仅数据库值 |
| 默认音色与台词覆盖 | domain/provider | Unit/Integration | P0 | RQ-004 | 缺音色阻止相应配音；旁白独立 |
| 连续输入、切镜头、任务完成及版本冲突 | store/HTTP | Integration/E2E | P0 | RQ-007 | 不丢文字、不覆盖未保存编辑 |
| 排队后档案更新、文件删除、任务重试 | task/repository | Integration | P0 | RQ-007 | 原快照可执行，引用删除保护有效 |
| 旧库升级与恢复 | SQLite | Integration | P0 | RQ-007 | 迁移幂等；不能把所有角色图误合为一个人物 |
| 复制/删除/重排/分镜替换/恢复 | application | Integration | P0 | RQ-007/008 | 引用重映射、历史还原及晚到结果可追溯 |
| 配音超长及双人顺序混音 | media | Integration | P0 | RQ-004 | 使用实测时长且不静默截断 |
| 中文 IME、@节点删除与粘贴 | UI | E2E | P1 | RQ-003/005 | 编辑可用且无损 |
| 卡片折叠、多图失败重试、小屏/键盘 | UI | E2E/manual | P1 | RQ-005 | 不仅截图相似 |
| 完整真实生成 | 全链路 | 人工集成 | P0 | RQ-006 | 选定模型验证，记录模型/适配版本与实际输入 |

### 验证要求

- 后端在 backend 下执行 `go test ./...`，重点涵盖新增引用、事务与任务链路；按仓库 CI 要求运行构建。
- 前端在 frontend 下执行 `npm run api:generate`、`npm run build`、`npm run lint`、`npm run test`；确认生成类型与 OpenAPI 一致。
- 执行新增工作台 E2E，并回归故事编辑、素材库、任务中心、视频选用和成片下载。
- 使用两个人物、每人两到三张参考图、一个场景多视角和多句台词，完成从保存到成片的验收；额外验证多个场景组。
- 联调记录所有选用图片是否实际进入正确阶段、TTS 是否使用正确音色、失败重试是否重复收费/提交；外部超时无法保证厂商幂等时显示提交状态待核实。
- 记录输入快照 ID、模型、阶段耗时、失败码和父子任务；日志不记录密钥。实测项目 payload 增长、图片并发上传及长列表编辑响应，再确定性能阈值。
- 本次为计划编制，不执行业务测试，不代表上述验收已通过。

---

## 风险与边界

| 风险项 | 影响 | 缓解措施 |
|--------|------|---------|
| 尚未选定/验证多图与 TTS 厂商 | P3/P4 实际落地受阻 | P0 查官方协议及最小请求验证；契约与数据阶段继续推进 |
| 描述与参考图冲突 | 输出难稳定 | 解析预览、视角说明、候选图人工选用；不承诺自动一致 |
| 档案更新影响多个镜头 | 历史成片被隐式改变 | 固定版本、局部覆盖、显式升级、依赖过期提示 |
| 项目 payload 增长 | 读写成本与版本冲突增加 | 描述/引用存元数据，文件外置；独立 catalog_version；测量后再拆表 |
| 旧绑定没有身份信息 | 不能可靠自动判断谁是谁 | 保留旧用途绑定，迁入“待归组”；人工确认身份，不按文件名合并 |
| 旧 narration/voiceover | 错判人物与音色 | narration 默认视作旁白文本；旧音轨原样保留，不推断其可复用音色 |
| 混用新旧编辑接口 | 全量保存抹掉新字段 | 老接口保留新字段或拒绝不兼容写入；迁移后新增 schema 版本检查 |
| 任务/快照/队列不一致 | 重复执行或结果无归属 | 持久待入队、原子快照、重试指纹、按任务 ID 幂等回填 |
| 删除镜头后晚到结果 | 丢失或误绑结果 | 保留任务输出为历史记录，不复活已删除镜头或写入新镜头 |
| 字幕、对口型和无限画布扩大范围 | 首版工期不可控 | 本计划覆盖后期配音；对口型按适配能力独立验证，完整字幕编辑与自由画布后续规划 |

迁移及回退：升级前备份数据库；新字段采用兼容读取；引用索引在事务中重建；迁移重复执行不创建重复实体。保留旧首尾帧/配音与视频版本。通过特性开关回到旧编辑入口时保护新增字段；降级到不理解新数据的旧二进制必须恢复备份，不能直接覆盖新版数据库。

P0 已确定首条实现路径：指定北京工作空间的 qwen-image-2.0-pro 多图生首帧、wan2.7-i2v 生视频及 qwen3-tts-flash 配音，独立 HTTP 实测成功并下载产物，详见 provider-capabilities.md。每次生图最多选三张参考图，项目档案保存数量不与提交上限混淆。音色采用官方按模型维护的目录，不默认包含声音克隆。

契约冻结：新增接口及结构已写入 docs/api/openapi.yaml，标记 x-implementation-status=planned，避免将契约误称为已上线接口。完整镜头编辑使用 PUT /context 原子更新绑定、描述文档与台词；档案删除首版使用 /archive 动作保留历史。VoiceProfile 与实体均固定版本；本镜头补充描述追加到基础描述，显式 description_override 替换基础描述。UI 细节固定于 ui-contract.md；保留首版固定布局，不包含自由画布。
