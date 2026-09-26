# 迁移脚本说明

本目录存放按编号组织的数据库迁移 SQL，由 `migration` 构建标签下的
`scripts/migration/embed.go` 嵌入二进制，经
[`internal/infra/migration/migration_embed.go`](../../internal/infra/migration/migration_embed.go)
用 [golang-migrate](https://github.com/golang-migrate/migrate) 的 `iofs` source 依次执行
（`make migrate`，或在 `config.yaml` 的 `Features` 里加 `Migration` 后随启动自动执行）。

```
scripts/migration/
├── embed.go        # go:embed **/*（带 migration 构建标签）
├── mysql/          # MySQL 方言，NNNN_name.{up,down}.sql
└── postgres/       # PostgreSQL 方言，NNNN_name.{up,down}.sql
```

## 关于 SQLite（issue #16 的说法已过时）

**当前主干只支持 MySQL 与 PostgreSQL 两个方言。**
SQLite 支持已在提交 `2e0ac8bf`（`feat: 移除SQLite数据库支持, 精简编译体积`）整体移除：
`scripts/migration/sqlite3/` 目录、`Sqlite3` 特性项、`conf/db_cgo.go` / `db_nocgo.go`
驱动接线一并删除，`internal/infra/migration/migration_embed.go` 也只注册了
`mysql` / `postgres` 两个 source。

因此：

- issue #16 中「`scripts/migration/sqlite3` 缺 `user_password_bcrypt` 迁移」「三方言各一份」
  的描述针对的是移除 SQLite 之前的旧目录树，在当前 `main` 上**不再成立**；
- 校验脚本只比对 `mysql` ↔ `postgres`，原因即上述（见
  `internal/infra/migration/consistency_test.go` 的 `requiredDialects` 注释）；
- 若将来恢复某个方言，把目录建回来即可：校验器会自动发现含 `*.sql` 的子目录，
  并要求它的迁移名集合与现有方言一致（漏写一处即 CI 失败），届时只需在
  `requiredDialects` 里补登记。

## 编号映射表（当前状态）

> 同一编号在两个方言下**含义不同**，排查问题时不要按编号跨方言对照，请按下表按名对照。

| 编号 | mysql | postgres |
| ---- | ----- | -------- |
| 0001 | initialize_schema | initialize_schema |
| 0002 | post_visibility | post_visibility |
| 0003 | feature_contact | feature_contact |
| **0004** | **optimize_idx**（MySQL 独有） | share_count |
| 0005 | share_count | topic_follow |
| 0006 | topic_follow | comment_thumbs |
| 0007 | comment_thumbs | content_type_alter |
| 0008 | content_type_alter | create_view_post_filter |
| 0009 | create_view_post_filter | user_following |
| 0010 | user_following | home_timeline |
| 0011 | home_timeline | comment_essence |
| 0012 | comment_essence | rank_metrics |
| 0013 | rank_metrics | user_relation_view |
| 0014 | user_relation_view | topic_user_pin |
| 0015 | topic_user_pin | site_settings |
| 0016 | site_settings | settings_kv |
| 0017 | settings_kv | add_roles_and_audit |
| 0018 | add_roles_and_audit | message_chat |
| 0019 | message_chat | **user_password_bcrypt** |
| 0020 | **user_password_bcrypt** | comment_audit_and_nickname_review |
| 0021 | comment_audit_and_nickname_review | remove_wallet_schema |
| 0022 | remove_wallet_schema | course_module |
| 0023 | course_module | remove_friendship |
| 0024 | remove_friendship | —（postgres 无此编号） |

- 总数：`mysql` 24 个，`postgres` 23 个。
- 密码 bcrypt 迁移：**postgres `0019`，mysql `0020`**。

### 错位的成因

`0004_optimize_idx` 是 MySQL 独有的一次索引改名（把 `idx_user` 之类改成
`idx_<table>_<column>` 形式）。PostgreSQL 的 `0001_initialize_schema` 建表时就直接按
最终命名创建了索引，所以没有对应迁移。自 `0004` 起 **mysql 编号 = postgres 编号 + 1**：

```
mysql    0004_optimize_idx  0005_share_count … 0024_remove_friendship
postgres 0004_share_count   0005_topic_follow … 0023_remove_friendship
             └──────────── 整体错一位 ────────────┘
```

这也是 issue #16 中「按 postgres 的下一个编号 0022 新增会和 mysql 的
`0022_remove_wallet_schema` 撞号」的根因：**两方言的"下一个编号"本来就不同**。

## 新增迁移的规则

1. **按目录取号，不按对方方言取号**：mysql 目录取本目录 `max(编号)+1`，postgres 目录同理。
   以当前状态为例：mysql 下一个是 **0025**，postgres 下一个是 **0024**。
2. 两方言**同名同序**：同一个变更在 `mysql/` 与 `postgres/` 各写一份，
   文件名除前缀编号外完全一致，`up` / `down` 成对。
3. 若某方言确实不需要该迁移（如本例 `optimize_idx`），在
   `internal/infra/migration/consistency_test.go` 的 `dialectOnlyMigrations`
   中登记 `dialect` / `name` / `reason`，并同步更新上面的映射表；
   登记项失效（对应文件被删或另一方言补了同名迁移）会让校验失败，届时请移除登记。
4. 提交前跑一次校验，并同步本 README 的映射表：

   ```bash
   go test ./internal/infra/migration/ -v
   ```

## 一致性校验

校验逻辑在 `internal/infra/migration/consistency_test.go`（`TestMigrationDialectConsistency`），
CI 的 `Migration consistency check` 步骤会执行；纯 Go 测试，无额外依赖。
检查项：

- `mysql` / `postgres` 目录必须存在（任何含 `*.sql` 的其他子目录也会被自动校验）；
- 文件名必须匹配 `NNNN_name.{up,down}.sql`，且每个迁移 `up` / `down` 成对；
- 单方言内编号从 `0001` 起**连续、无重复**（跳号或一号两迁即失败）；
- 跨方言按**迁移名集合**（忽略编号）比对，缺失项即失败，
  除非已在 `dialectOnlyMigrations` 登记原因；
- 方言独有登记项过期即失败（防止登记清单腐化）。

> 编号错位本身**不会**让校验失败——它是历史既成事实，见下节。
> 校验器另有正/反用例测试（`TestMigrationCheckerDetectsProblems`、
> `TestMigrationCheckerAcceptsHealthyTree`），保证它确实能发现问题。

## 未来可选：编号对齐（破坏性，暂不执行）

让两方言同号同义需要给 mysql 补一个空迁移占位或整体重编号。**重编号是破坏性操作**：
已部署库的 `schema_migrations` 表记录的是旧版本号，重编号后 golang-migrate 会认为
存在未执行的迁移（或直接进入 dirty 状态），必须手工重写该表才能对齐。
**本仓库不做重编号**：只有在确认没有生产数据、需要重置数据库时才考虑，
且应作为独立的、明确标注破坏性的变更提出。当前阶段的正确做法是
「按目录取号 + 名称集合校验」，见上文规则。
