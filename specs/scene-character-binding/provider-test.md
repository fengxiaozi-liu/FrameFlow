# 指定接口测试记录

以下第一节为 2026-09-28 的首次探测记录；后续应用内生成、中文候选与最终验收已补齐，见本文后面的补充记录。

- 日期：2026-09-28
- 服务：用户提供的阿里云工作空间域名 `ws-zq5c50cpzq9db8rg.cn-beijing.maas.aliyuncs.com`
- Chat 路径：`/compatible-mode/v1/chat/completions`
- 鉴权：使用用户提供的 Bearer 凭证，仅在测试进程内使用；本文件不保存凭证。
- 执行方式：独立 HTTP 请求，并未通过 FrameFlow 应用配置或处理器调用。

| 检查 | 请求 | 结果 | 能证明什么 |
|------|------|------|-----------|
| 模型发现 | GET /compatible-mode/v1/models | HTTP 200，返回模型 ID 列表 | 网络、鉴权与列表路径可用 |
| 分镜 JSON | POST /compatible-mode/v1/chat/completions，model=qwen-plus | HTTP 200，finish_reason=stop | 一次短文本分镜请求成功 |
| 输出解析 | 对模型原始 content 直接做 JSON 解析 | 通过；1 个场景、2 个人物、1 个镜头 | 本次输出无 Markdown 包裹且语法有效 |
| 引用完整性 | 检查 scene_keys、character_keys、speaker_key | 全部指向返回的实体 key | 本次场景、人物与说话者未产生悬空引用 |

请求使用虚构咖啡馆对话：Ming 穿蓝色外套，Yu 穿米色毛衣，分别说两句短台词。没有上传项目源代码、私人文件或图片。请求设置 max_tokens=700、temperature=0.1、stream=false；输入 123 tokens，输出 249 tokens，共 372 tokens。

模型返回 key 为 cafe、ming、yu，镜头引用 cafe 与两个角色，台词分别引用 ming 与 yu。测试仅使用英文短输入；尚未验证中文长正文、复杂消歧、多次稳定性及完整 JSON Schema。

## 未覆盖与后续接入

- 未执行多图理解、多图生图、视频生成、音色目录、TTS 或对口型测试；这些路径和实际可用模型需要单独核实。列表中出现相关模型名称不代表具备调用权限或当前应用已支持。
- 当前客户端已有文本请求路径 `/compatible-mode/v1/chat/completions`，配置时需依据 URL 拼接规则选择服务根地址，不能直接把完整 completions 地址重复拼接。
- 当前 `bailian/client.go` 的模型发现使用 `/api/v1/models`，与本次成功的兼容路径不同；需实现明确的端点配置或协议适配并补测试。
- 当前分镜解析器接受旧 scenes 契约。本次提议的 scenes/characters/shots 返回需要新版解析与应用逻辑，不能宣称已在 FrameFlow 内跑通。
- 测试前沙箱网络连接失败，取得联网执行许可后成功；连接失败不是服务端鉴权失败。
- 下一次测试从应用运行时环境或现有凭证保险库读取密钥，不把聊天凭证复制进脚本、测试夹具、日志或源码。
# 应用内真实生成补充（2026-09-29）

- 独立测试数据库与凭据保险库，未修改用户原项目；脚本 `live-integration.cjs` 不含凭据。
- 完整流程 `run-537bae9c913d08e228f7220c705b9ba9`：两张上传参考图 → qwen-image-2.0-pro → Cherry / Ethan 两句中文配音 → wan2.7-i2v 六秒视频 → FFmpeg 成片。
- 五项任务全部成功并保存本地文件，见 `evidence/live-integration.json`。
- 实际浏览器通过「播放成片」加载并播放，媒体 `duration=6`、`readyState=4`、`paused=false`，画面见 `evidence/live-playback.png`。
- 合成联调发现旧执行器误将本地合成视为缺模型，已修复并重试原合成任务；未重复生成成功的云端结果。
- 语音角色关联通过不可变快照中的 Cherry / Ethan 验证。音画采用后期配音，不声明口型同步；人物一致性仍受生成模型影响。

## 最终版本浏览器验收（2026-09-29）

- qwen-plus 实际中文候选任务 `task-64e23133a6249454e824323805e6ffa8`：一个场景、两个人物、两句台词；在界面修正人物名字/描述后应用，复制与恢复成功。
- 第二次完整实际流程 `run-c989010241683f4fb95bb9335b907f70`：从空镜头绑定三个档案和两张参考图，设置两句中文台词，界面逐项选用并导出。
- 五项子/合成任务成功，父流程 completed；最终文件 `evidence/final-video.mp4`，H.264 + AAC，1280×720，6.000 秒；浏览器播放 readyState=4、paused=false。
- 视频生成期间重启测试服务，前后云端任务 ID 均为 `702e51d8-1837-4171-9a3f-56eeea48d9a1`；成功恢复落盘。
- 证据和源码摘要：`evidence/final-acceptance.json`；界面：`evidence/final-playback.png`、`evidence/final-workbench.png`。测试凭据保存在隔离运行环境保险库中，脚本与证据不含凭据。
