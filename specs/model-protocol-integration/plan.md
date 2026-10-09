# 实施计划：统一模型协议接入

Created: 2026-10-09 · Status: 已完成 · Spec: spec.md

## 摘要
采用模块化单体、DDD 值对象与领域校验、Repository、协议 Adapter 和显式 Registry；取消运行期按模型名称分流。

## 项目适配输入
Go/Gin/SQLite + Vue/TS；交付配置领域模型、四类协议调用、迁移、配置 UI、契约与测试。检查 go test/vet、前端 lint/test/build、关键 E2E。

## Phase 0：调研与关键决策
### 实体与关系分析
模型聚合持有操作绑定与规格及修订，通过 connection_id 引用连接；任务持有无密钥配置快照。既有创作聚合保持边界。
### 状态流转
未配置 → 配置完整；启用独立控制。配置保存使用预期修订；真实调用成功不由配置推断。任务仍使用既有状态机。
### 依赖调研
复用现有协议请求/响应和 mock 契约，保留 SQLite JSON 数据结构兼容；新 handler 放接口层。
### 调研结论
按模型适配拒绝；动态脚本模板引擎复杂且不可控，拒绝；按 wire protocol 注册复用选用。

## Phase 1：技术设计
### 模块设计
provider 领域负责操作配置及解析；应用用例负责修改配置；interfaces/http 解析命令；infrastructure/provider 选择协议；application/execution 编排。
### 协议/接口设计
增加模型配置更新和协议目录；能力查询增加各操作的配置状态及原因，旧能力字段兼容。生成页根据后端规格渲染。
### 数据/配置设计
Model.operations 按能力映射绑定和规格；revision 乐观并发；旧配置只在兼容边界填入预设；Task 冻结 Model 配置，保留 connection_id 凭据引用。
### 错误/枚举/文案
configuration_missing、protocol_unsupported、disabled、invalid_parameters、revision_conflict；提示用户去模型管理补充配置。

## 测试与验证
陌生 ID 四操作、参数边界、协议不匹配、配置并发、旧配置迁移、快照不可变、单图/多图视频、现有取消与恢复回归。只使用隔离库与模拟协议。

## 风险与边界
厂商接口格式不统一：协议必须显式选择；未知参数不推断。保留旧 HTTP 用例债务，本次不全面重写。新字段为增量 JSON，回滚前备份数据，旧程序不能正确使用新自定义协议任务。
