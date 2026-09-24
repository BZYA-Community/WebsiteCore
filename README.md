[![CI](https://github.com/BZYA-Community/WebsiteCore/actions/workflows/ci.yml/badge.svg)](https://github.com/BZYA-Community/WebsiteCore/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/BZYA-Community/WebsiteCore)](https://goreportcard.com/report/github.com/BZYA-Community/WebsiteCore)
[![Forks](https://img.shields.io/github/forks/BZYA-Community/WebsiteCore?style=flat)](https://github.com/BZYA-Community/WebsiteCore/network/members)
[![Stars](https://img.shields.io/github/stars/BZYA-Community/WebsiteCore.svg?style=flat)](https://github.com/BZYA-Community/WebsiteCore/stargazers)
[![MIT License](https://img.shields.io/github/license/BZYA-Community/WebsiteCore.svg?style=flat)](https://github.com/BZYA-Community/WebsiteCore/blob/main/LICENSE)
[![Contributors](https://img.shields.io/github/contributors/BZYA-Community/WebsiteCore?style=flat)](https://github.com/BZYA-Community/WebsiteCore/graphs/contributors)

<div align="center">
  <img src="./.assets/readme/paopao-logo.png" alt="logo" width="88" height="88">
  <h1>WebsiteCore</h1>
  <p>
    基于 <a href="https://github.com/rocboss/paopao-ce">paopao-ce</a> 深度定制的微社区系统<br>
    Go + Vue3 全栈 · 身份组权限 · 内容审核 · B站式站内私信
  </p>
</div>

---

## 简介

WebsiteCore 是一个自托管的微社区/论坛系统：Go 后端（Gin + GORM + Redis + Meilisearch）内嵌 Vue 3 前端单页应用，单二进制即可运行。在上游 paopao-ce 基础上，本仓库增加了身份组体系、内容审核流与会话化站内私信等能力。

## 功能特性

- **用户身份组与 RBAC**：游客 / 道友 / 导师 / 审核 / 管理员 / 运维，管理后台支持角色变更、禁言与软删除，全程留痕（角色变更日志）
- **内容审核**：普通用户发帖进入审核队列，导师及以上免审；审核拒绝打回私密并可重新提交，审核结果站内通知
- **站内私信（B站式）**：消息中心 = 会话列表（系统联系人置顶）+ 独立聊天窗；已读/未读、历史分页、身份组权限（道友↔道友禁止私信，道友对高级身份首条限制，回复后解除）
- **系统通知会话**：关注 / 评论 / 回复 / 审核 / 管理通知统一归入「系统通知」会话，可跳转到帖子与用户主页
- **好友与关注**：好友申请（通讯录内同意/拒绝）、单向关注、好友可见/关注可见等帖子可见性
- **内容形态**：短动态（图片/视频/附件/收费附件）、Markdown 长文、话题标签、热搜趋势
- **可插拔特性**：存储（LocalOSS/MinIO/S3）、搜索（Meilisearch/Zinc）、数据库（PostgreSQL/MySQL/SQLite）等均通过 `Features` 开关装配

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go · Gin · GORM · Redis(rueidis) · go-mir(接口代码生成) · golang-migrate(数据库迁移) |
| 前端 | Vue 3 · Vite · Naive UI · Pinia · vue-advanced-chat(私信) · md-editor-v3(长文) |
| 依赖服务 | PostgreSQL / MySQL / SQLite · Redis · Meilisearch(可选) |

## 快速开始

### 1. 准备依赖服务

需要一个数据库（PostgreSQL/MySQL/SQLite）、Redis；全文搜索可选 Meilisearch。

### 2. 配置

```bash
cp config.yaml.sample config.yaml
```

关键项：

- `Features.Default`：特性开关列表。数据库特性名写 `Postgres` / `MySQL` / `Sqlite3`；启用自动建表迁移需同时加 `Migration` 并在编译时带 `migration` tag
- `WebServer.HttpPort`：监听端口（默认 8008）
- 数据库 / Redis / Meili 等连接信息按环境填写

### 3. 构建并运行

```bash
# 构建前端(产物内嵌进二进制)
cd web && npm install && npx vite build && cd ..

# 构建后端(embed=内嵌前端, migration=启动时自动迁移数据库)
make build TAGS='embed migration'

# 运行(配置与数据文件相对二进制所在目录)
cd release && ./paopao serve        # Windows: paopao.exe serve
```

访问 `http://127.0.0.1:8008`。更多安装/部署细节见 [docs/INSTALL_ZH.md](docs/INSTALL_ZH.md)。

## 开发

```bash
make run          # 后端开发模式(go run)
make gen-mir      # 接口代码再生成: mirc/web/v1/*.go -> auto/api/v1/(勿手改生成物)
make gen-enum     # 枚举代码再生成
make test         # 测试
cd web && npm run dev    # 前端开发服务
```

新增 API 的标准流程：在 `mirc/web/v1/` 声明接口签名 → `make gen-mir` 生成路由骨架 → 在 `internal/servants/web/` 实现业务。数据库结构变更在 `scripts/migration/{mysql,postgres,sqlite3}/` 按编号新增 `NNNN_name.{up,down}.sql`（三方言各一份）。

## 目录结构

```
├── auto/          # go-mir 生成的路由/接口代码(勿手改)
├── cmd/           # 命令入口(serve/version/migrate)
├── internal/      # 后端实现: conf(配置装配) core(服务接口) dao(数据层) servants(HTTP层)
├── mirc/          # mir 接口声明(API 的唯一事实来源)
├── pkg/           # 通用工具库
├── release/       # 构建输出
├── scripts/       # migration SQL 等脚本
├── web/           # Vue3 前端
└── docs/          # 全部文档(安装/部署/提案/API/变更记录)
```

## 文档

| 文档 | 说明 |
| --- | --- |
| [docs/INSTALL_ZH.md](docs/INSTALL_ZH.md) | 安装与本地部署指南 |
| [docs/deploy/](docs/deploy/) | 本地/云平台/K8s 部署参考 |
| [docs/features-status.md](docs/features-status.md) | 功能项成熟度与状态 |
| [docs/CHANGELOG.md](docs/CHANGELOG.md) | 版本变更记录 |
| [docs/proposal/](docs/proposal/) | 设计提案与实现笔记 |
| [docs/openapi/](docs/openapi/) | OpenAPI 文档资源（运行时由 `/docs/openapi` 提供） |

完整索引见 [docs/README_ZH.md](docs/README_ZH.md)。

## 致谢

- 上游项目 [rocboss/paopao-ce](https://github.com/rocboss/paopao-ce)（MIT License）

## License

[MIT](LICENSE)
