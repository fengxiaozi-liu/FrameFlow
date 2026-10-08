# 分镜工作区兼容说明
> 2026-10-08：本页记录初版图→视频流程。新建任务已改为素材直接生成视频，详见 [后续兼容说明](reference-video-workflow.md)。历史任务仍适用本页原规则。


## 页面入口

- 分镜列表：`/studio/:projectId/:draftId/shots`。
- 分镜详情：`/studio/:projectId/:draftId/shots/:sceneId`。
- 制作任务详情：`/studio/:projectId/:draftId/shots/:sceneId/runs/:runId`。
- 旧 `?scene=...` 链接跳转新详情。正文的分镜标签进入列表，成片入口保留在正文工作区的 `?tab=video`。原独立 Task 继续可在任务中心查看、重试、下载。

## 镜头配置

Scene.generation_settings 是可选字段。旧草稿没有该字段时，不自动指定厂商模型；用户选择分镜图/视频模型后保存完整设置。画幅默认 16:9、清晰度默认 720P，时长沿用 Scene.duration_seconds。服务端校验模型能力并同步 Scene.duration_seconds。

镜头标题、旁白、描述、绑定和设置通过版本检查原子保存。保存失败保留本地输入；跳转和生成前保存。设置仅属于当前镜头。

EntityBinding.reference_material_ids 是当前镜头完整参考图覆盖列表，支持直接上传、替换、移除；缺省沿用 selected_material_ids 或项目档案参考图，空数组表示不使用图片。旧 selected_material_ids 仍只能选择档案内图片。参考图覆盖不增加项目档案版本；显式“编辑项目设定”继续创建新版本并仅由当前镜头采用。

新增字段保存在项目 JSON 聚合内，无破坏性 SQLite 表变更。素材引用索引包含当前镜头独立图片、历史分镜快照、生成 Run 和底层 Task 快照；有引用的素材不能删除。升级前按现有运维流程备份 SQLite 和媒体文件。

## 一个生成对应一个任务

一次提交创建一个 GenerationRun。图片、配音和视频是 Run 内部 Task，重试这些步骤保留 Run ID 和冻结输入。重复提交同一键与相同请求返回同一 Run；输入变化必须新建意图。网络中断时浏览器保留请求与幂等键，重试恢复原提交。

新 Run 含独立 selection、created_at、updated_at。图片和每句配音选择属于当前 Run；视频由冻结描述/参数和该 Run 首帧推进。视频选用后发布到草稿 selected_versions，供现有成片流程消费。视频生成成功、视频已选用、整片合成完成是不同状态。

取消 Run 停止后续推进，已完成媒体保留；已取消 Run 的底层步骤不可重启，需要重新生成。活动 Run 尚在制作/等待候选选用时，页面禁用再次生成；失败、取消或视频已选用后可明确发起新一轮。

## 历史记录

已有 Run ID、Task ID、快照和结果保留。旧 Run 缺 selection 时，不自动从草稿全局选择推断任务内选择；继续使用旧推进/选择行为，并在任务详情通过旧选择接口处理。缺时间显示“历史记录”，不补写猜测时间。旧 ID 算法的相同幂等请求仍能恢复已有 Run。

选用历史视频不会重写历史输入。当前台词或设定已变化时，现有合成校验仍可能提示选用旧音频等问题；需在成片页面处理或针对当前设定重新生成。

## 已知运行条件

厂商调用需要有效模型和密钥，整片导出需要 ffmpeg/ffprobe，参考音频交付需要对象存储。界面能够选择模型不代表这些运行条件已配置。验证范围见 `specs/shot-task-workspace/validation.md`。
