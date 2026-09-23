# 故事、分镜与素材迁移

本文件记录上线前的迁移和恢复步骤。样本位于 `backend/internal/infrastructure/sqlite/testdata/legacy/`，代表旧项目、任务、五类素材，以及没有镜头归属的历史任务。迁移及其重跑、失败回滚、引用删除保护由 SQLite 和 HTTP 回归测试覆盖。

## 迁移前

1. 停止服务和后台任务，记录数据库路径、上传目录及版本。
2. 使用现有仓储备份能力生成 SQLite 一致性备份，并复制整个上传目录；对数据库运行完整性检查，对媒体文件清点 ID、路径、大小与哈希。
3. 在数据库和媒体备份上先演练，不能直接在唯一生产副本上试跑。

## 目标与规则

- 保留项目、草稿、镜头、素材和任务的 ID 与文件路径；旧任务原始输入不可改写。
- `visual` 映射“场景”；`frame` 映射“场景”并标记“原首尾帧”；`character`、`voice`、`music` 分别映射“角色”、“配音”、“音乐”。
- 旧任务中的 `source_image_url` 保持原义，但它不构成新镜头绑定。缺少镜头归属的旧任务只在任务中心显示；不同项目和草稿的结果不得串写。
- 若旧数据存在明确的首帧/尾帧用途，保留该用途；单纯全局选中状态不推断为所有镜头绑定。
- 迁移采用版本号与 SQLite 事务；重复运行应无改动。媒体文件不通过删除或重命名来完成分类迁移。

## 验证与恢复

1. 比较迁移前后项目、草稿、镜头、素材、任务数及原有 ID；检查五个旧分类映射、标签和旧文件可读性。
2. 重跑迁移，检查记录与文件哈希不变；模拟事务中途失败，检查数据库回滚。
3. 检查跨项目/草稿任务、无镜头归属任务和已删除镜头结果，确认没有自动关联。
4. 若验证失败，停止服务，保留失败副本供分析，从备份恢复数据库与上传目录后再次做完整性检查；不能靠清空草稿或删除旧素材修复。

自动演练覆盖 `TestLegacyMaterialMigrationIsIdempotentAndKeepsIdentity`、`TestLegacyMaterialMigrationRollsBackOnBadPayload`、`TestTaskScopeAndLegacySchemaMigration` 和 `TestSceneBindingRequiresConfirmedMaterialAndVersion`。旧媒体文件自身不参与 SQL 迁移，发布前仍需在生产备份副本上执行第 1 步的文件哈希清点。
