# bonkeAI 视频失败分析与配置对照

日期：2026-10-13。依据：用户提供的《视频生成 API（Seedance · 火山 Ark 原生兼容）》（标注 2026-09-23，部分内容更新至 2026-09-30）及《素材库 API》（2026-09-20-r2）。未调用真实视频创建接口，未修改用户连接或密钥。

## 本次 HTTP 404 的原因

失败任务冻结的地址为 `https://ai-token-admin.xtt.xyz/v1/generate`，配置为同步 JSON，结果字段为 `/url`。两份文档均没有 `/v1/generate`；新版视频文档列出的创建路径为 `/api/v3/contents/generations/tasks`，附录还提供 `/v1/videos`。主地址与文档一致，创建路径配置不匹配是此次 404 的明确问题。

两个入口都是异步任务：创建只返回 `id`，不能立即读取视频 URL。因此只修复请求路径仍不足以完成生成，还要改为异步轮询并配置查询与结果映射。

## 两种协议不得混用

| 配置 | Ark 原生 | OpenAI Video 风格 |
| --- | --- | --- |
| 创建 POST | `/api/v3/contents/generations/tasks` | `/v1/videos` |
| 创建响应任务 ID | `/id` | `/id` |
| 查询 GET | `/api/v3/contents/generations/tasks/{task_id}` | `/v1/videos/{task_id}` |
| 提示词 | `/content/0/text`，该项 `type=text` | `/prompt` |
| 时长 | `/duration` | `/seconds` |
| 分辨率 / 比例 | `/resolution`、`/ratio` | `/metadata/resolution`、`/metadata/ratio` |
| 多图参考 | `content` 中逐张 `type=image_url`、`role=reference_image`、`image_url.url` | 顶层 `images` 或 `metadata.content` |
| 查询状态 | `/status` | `/status` |
| 成功状态 | `succeeded` | `completed` |
| 视频地址 | `/content/video_url` | `/ark/content/video_url`（按附录的 Ark 透传结构） |

Ark 待处理状态为 `queued,running`，失败终态为 `failed,cancelled,expired`。OpenAI 风格文档未完整列出待处理枚举，须按实际查询响应确认；可将 `queued,in_progress,running` 配置为待处理，成功配置 `completed`，失败配置 `failed,cancelled,expired`。不要在 OpenAI 路径发送顶层 `content`，素材文档明确说明它会被丢弃。

## 适合现有自定义 JSON 配置的示例

现有适配器能动态绑定字符串图片数组，不能自动把任意数量图片转换成 Ark 的 `content` 对象数组。因此多图参考可先选择文档支持的 `/v1/videos` + 顶层 `images`。

请求示例（图片必须替换成实际可访问地址；模型以账户目录为准）：

```json
{
  "model": "doubao-seedance-2-5-260628",
  "prompt": "镜头描述",
  "seconds": 5,
  "images": ["https://your-public-host.example/reference.png"],
  "metadata": {
    "resolution": "720p",
    "ratio": "16:9"
  }
}
```

请求字段绑定：模型 ID → `/model`，提示词 → `/prompt`，视频时长 → `/seconds`，智能参考图片 → `/images`，画面比例 → `/metadata/ratio`。创建响应示例为 `{"id":"example-task"}`，远端任务 ID → `/id`。查询响应示例可填 `{"status":"completed","ark":{"content":{"video_url":"https://your-public-host.example/result.mp4"}}}`，映射状态 → `/status`、媒体结果 URL → `/ark/content/video_url`。

**分辨率不能直接忽略大小写差异**：文档使用小写 `480p/720p`，FrameFlow 当前视频界面和传入变量使用大写 `480P/720P`，自定义适配器原样发送变量。若先以固定 `720p` 验证，应解除清晰度字段绑定，并将该操作支持的清晰度限制为仅 `720P`，让界面选择与实际请求一致。需要同时动态选择多种清晰度时，仍需明确的参数转换支持，不能无声改变用户选择。

以上只是配置与协议对照，不代表此请求已获远端接受或真实生成成功。

## 还需核对的接入条件

2026-10-14 使用当前账户、`seedance-2.5-global-token`、原首帧和 5 秒请求进行了一次真实提交。`GET /v1/models` 返回 200，确认当前模型及 `doubao-seedance-2-5-260628` 均在目录；创建返回 400，`error.code=fail_to_fetch_task`，嵌套错误明确为 `field 'ratio' value 16:9 is not in the allowed options, allowed values: adaptive`。因此当前模型应配置比例仅为 `adaptive`，并将 `/metadata/ratio` 绑定到画面比例。此限制来自该次实际错误，不能推广到其他模型；没有返回任务 ID、未验证真实出片或图片是否可接受。新版本配置方式见 [自定义比例](migrations/custom-ratio-options.md)。

- 失败任务模型 ID 为 `seedance-2.5-global-token`，文档列举的是 `doubao-seedance-2-5-260628` 等 ID。不能据此断言前者不存在，也不能只修改显示名称；需要同步该账户的 `/v1/models`，选择实际返回的远端 ID。Seedance 2.5 普通生成文档只列出 480p/720p；1080p 成片属于另一种基于样片 ID 的工作流，不应与普通生成混合声明。
- FrameFlow 当前把本地参考图编码为 `data:image/...;base64,...`。文档列出的素材地址为公网可下载 HTTPS 或 `asset://`，没有声明支持 Data URI。这是路径修复后的兼容风险，需要服务商确认或增加可公开下载的媒体交付；本机 `/media/...`、localhost 和 Windows 路径不能直接供远端下载。
- 普通风景、产品等素材不要求走真人认证流程。私有真人资产按素材文档完成认证及入库，`Active` 后使用 `asset://<asset_id>`；这是远端平台素材库，与 FrameFlow 本地素材管理不同。
- 视频与素材使用同账号的模型 API 令牌。HTTP 404 本身不能证明密钥无效，本次没有重新读取或更换密钥。
- 修改连接和模型配置不会改变历史任务。修复后点击“生成视频”创建新记录；“按原输入重试”仍调用旧快照中的 `/v1/generate`。

## 文档矛盾与验证边界

旧素材文档第 6 节写着 Ark 原生路径会返回 `Invalid URL`，要求改用 `/v1/videos`；同一文档第 5 节又包含 Ark 示例。较新视频文档明确宣布支持 Ark 原生路径。这是文档版本不一致，应以较新视频文档为主要依据，并向平台确认部署版本。

2026-10-13 只读、未带凭据的 GET 检查：主地址返回 200，`/v1/models`、`/v1/videos`、`/v1/videos/analysis-nonexistent` 和 `/api/v3/contents/generations/tasks/analysis-nonexistent` 都返回 JSON 401。这表明这些请求进入鉴权处理，不能证明 POST 创建成功、模型权限或生成参数有效。没有进行计费创建、远端删除、素材认证或真实素材入库。

文档不提供远端视频任务取消/删除接口。本次失败记录删除实现只操作 FrameFlow 的本地历史和子任务，不向该平台发送 DELETE，已生成素材保留。
