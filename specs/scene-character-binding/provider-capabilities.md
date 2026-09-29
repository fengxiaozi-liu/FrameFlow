# 生成服务能力核验（2026-09-28）

工作空间：用户指定的北京服务；凭证不保存。以下为独立 HTTP 实测，不代表应用适配已实现。

| 能力 | 模型/路径 | 实测结果 | 证据 |
|---|---|---|---|
| 文本分镜 | qwen-plus；/compatible-mode/v1/chat/completions | 已通过 JSON 与引用检查 | provider-test.md |
| 多图生图 | qwen-image-2.0-pro；/api/v1/services/aigc/multimodal-generation/generation | 两张官方示例图，n=1，1024×1024；HTTP 200，返回一张图片并下载 | evidence/multi-image-probe.png；request d9c373b8-b297-9fdb-b7f9-3cc8b6e0e74e |
| TTS | qwen3-tts-flash；同上 multimodal-generation 路径 | Cherry、English、20 字符短句；HTTP 200，返回音频并下载 | evidence/tts-probe.wav；request 0e5eef64-5b9e-9d37-b08d-523c85edfbc4 |
| 图生视频 | wan2.7-i2v；/api/v1/services/aigc/video-generation/video-synthesis | first_frame、720P、2秒；异步任务 SUCCEEDED，下载 MP4 | evidence/video-probe.mp4；task a1a87b52-becd-4a74-9213-b2562112c437 |

路径选择：第一版实现多图生首帧→选用首帧→图生视频。不是将角色图直接送给 i2v。模型发现兼容列表未展示 wan2.7-i2v，但该模型实际调用成功，因此目录与能力注册不能仅靠列表过滤。

协议依据：

- [官方多图编辑 API](https://www.alibabacloud.com/help/en/model-studio/qwen-image-edit-api)：content 按顺序包含 image 与 text；参考图最多三张，文件最多 10MB。此次只实测两图，其他边界需适配测试验证。
- [官方 TTS API](https://help.aliyun.com/en/model-studio/qwen-tts-api)：input 包含 text、voice、language_type；输出 audio.url。首次支持默认语速，不把未核验的语速参数假装发送成功。
- [官方音色表](https://help.aliyun.com/en/model-studio/qwen-tts-voice-list)：按模型维护兼容的静态音色目录，保留来源与版本；本次只验证 Cherry，双音色/中文联调仍在 T035。
- [官方图生视频 API](https://help.aliyun.com/en/model-studio/image-to-video-general-api-reference)：media 包含 first_frame；异步请求头；查询 /api/v1/tasks/{id}。使用已有仓库的 2–15 秒能力范围，额外边界另测。

档案可保存多图，但本次选定生图路径每镜头选用总数不得超过三张。超限必须要求用户选择子集或更换经验证的模型，不自动丢图。视频实际画幅继承首帧，输出画幅选择必须在生首帧时生效。

未声称验证：多角色身份质量、所有官方限制、全部音色、中文稳定性、音画同步、应用内部流程。以上文件仅证明服务返回了可下载产物；后续必须通过内容检查及真实应用联调。
