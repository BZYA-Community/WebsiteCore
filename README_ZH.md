<div align="center">
  <h1>WebsiteCore</h1>
  <p>
    基于 <a href="https://github.com/rocboss/paopao-ce">paopao-ce</a> 深度定制的微社区系统<br>
    Go + Vue3 全栈 · 身份组权限 · 内容审核 · B站式站内私信
  </p>
  <a href="https://github.com/BZYA-Community/WebsiteCore/actions/workflows/ci.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/BZYA-Community/WebsiteCore/ci.yml?style=flat-square&label=CI" alt="CI">
  </a>
  <a href="https://goreportcard.com/report/github.com/BZYA-Community/WebsiteCore">
    <img src="https://img.shields.io/badge/Go%20Report%20Card-A%2B-brightgreen?style=flat-square" alt="Go Report Card">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore">
    <img src="https://img.shields.io/github/go-mod/go-version/BZYA-Community/WebsiteCore?style=flat-square&label=Go" alt="Go Version">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/BZYA-Community/WebsiteCore?style=flat-square" alt="License">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/issues">
    <img src="https://img.shields.io/github/issues/BZYA-Community/WebsiteCore?style=flat-square" alt="Issues">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/stargazers">
    <img src="https://img.shields.io/github/stars/BZYA-Community/WebsiteCore?style=flat-square" alt="Stars">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/network/members">
    <img src="https://img.shields.io/github/forks/BZYA-Community/WebsiteCore?style=flat-square" alt="Forks">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/graphs/contributors">
    <img src="https://img.shields.io/github/contributors/BZYA-Community/WebsiteCore?style=flat-square" alt="Contributors">
  </a>
</div>

<br>

[English](README.md) | 简体中文

## 简介

WebsiteCore 是一个自托管的微社区/论坛系统：Go 后端（Gin + GORM + Redis + Meilisearch）内嵌 Vue 3 前端单页应用，单二进制即可运行。在上游 paopao-ce 基础上，本仓库增加了身份组体系、内容审核流、会话化站内私信与课程模块，由学生社区自主维护，服务于一个面向未成年人的平台。

## 功能特性

- **用户身份组与 RBAC**：游客 / 道友 / 导师 / 审核 / 管理员 / 运维，管理后台支持角色变更、禁言与软删除，全程留痕（角色变更日志）
- **内容审核**：普通用户发帖进入审核队列，导师及以上免审；审核拒绝打回私密并可重新提交，审核结果站内通知
- **站内私信（B站式）**：消息中心 = 会话列表（系统联系人置顶）+ 独立聊天窗；已读/未读、历史分页、按身份组控制私信权限
- **系统通知会话**：关注 / 评论 / 回复 / 审核 / 管理通知统一归入「系统通知」会话，可跳转到帖子与用户主页
- **课程与长文**：课程分组、播放量、签名播放、Markdown 长文、话题标签、热搜趋势
- **关注与可见性**：单向关注、帖子可见性分级（公开 / 关注可见 / 私密）
- **可插拔特性**：存储（LocalOSS/MinIO/S3/阿里OSS/腾讯COS/华为OBS）、搜索（Meilisearch/Zinc）、数据库（PostgreSQL/MySQL）等均通过 `Features` 开关装配

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go · Gin · GORM · Redis(rueidis) · go-mir(接口代码生成) · golang-migrate(数据库迁移) |
| 前端 | Vue 3 · Vite · Naive UI · Pinia · vue-advanced-chat(私信) · md-editor-v3(长文) · Artplayer |
| 依赖服务 | PostgreSQL / MySQL · Redis · Meilisearch(可选) |

## 快速开始

### 1. 启动依赖服务

本地开发依赖（PostgreSQL、Redis、Meilisearch）由仓库自带的 compose 一键启动：

```bash
make deps-up        # 启动并等待健康检查通过
```

镜像均已 pin（`postgres:18.6` / `redis:7.4.11` / `getmeili/meilisearch:v1.54.0`），端口只绑定 `127.0.0.1`。详见 [docs/deploy/local.md](docs/deploy/local.md)。

数据库默认 PostgreSQL，也支持 MySQL；全文搜索使用 Meilisearch（可选）。

### 2. 配置

```bash
cp config.yaml.sample config.yaml
```

关键项：

- `JWT.Secret`：**必填**，留空启动会直接退出。生成：`openssl rand -hex 24`
- `Features.Default`：特性开关列表，默认已是 `Postgres`；自动建表迁移需加 `Migration` 特性并用 `migration` tag 编译（或直接用 `make migrate`）
- `WebServer.HttpPort`：监听端口（默认 8008）
- 数据库 / Redis / Meili 连接信息已与 `docker-compose.dev.yml` 对齐，无需改动

### 3. 建库

```bash
make migrate        # 用内嵌迁移脚本建出完整 schema
```

### 4. 构建前端并运行

```bash
make build-web          # 构建前端产物(供 embed 内嵌)
make run TAGS='embed'   # 启动后端并内嵌前端
```

访问 `http://127.0.0.1:8008`。完整安装与部署指南见 [docs/INSTALL.md](docs/INSTALL.md) 与 [docs/deploy/](docs/deploy/)。

## 开发

```bash
make run          # 后端开发模式(go run)
make gen-mir      # 接口代码再生成: mirc/ -> auto/(勿手改生成物)
make gen-enum     # 枚举代码再生成
make test         # 测试
cd web && npm run dev    # 前端开发服务
```

新增 API 的标准流程：在 `mirc/web/v1/` 声明接口签名 → `make gen-mir` 生成路由骨架 → 在 `internal/servants/web/` 实现业务。数据库结构变更在 `scripts/migration/{mysql,postgres}/` 按编号新增 `NNNN_name.{up,down}.sql`（两方言各一份）。完整开发指南见 [docs/development.md](docs/development.md)。

## 贡献

每个 PR 必须通过 CI、AI 审查与 **BVT（构建验证测试）**：后端语法/构建/lint/测试检查；前端改动还须通过标准视口下的页面重叠检查。完整流程、角色晋升制度与审查规则见 [CONTRIBUTING.md](CONTRIBUTING.md)。

本仓库由学生社区自主维护，所有贡献都会记录在每周五自动生成的周报中。

## 目录结构

```
├── auto/          # go-mir 生成的路由/接口代码(勿手改)
├── cmd/           # 命令入口(serve/migrate/version)
├── internal/      # 后端实现: conf(配置) core(接口) dao(数据层) servants(HTTP层) service(装配) sitesetting(后台设置)
├── mirc/          # mir 接口声明(API 的唯一事实来源)
├── pkg/           # 通用工具库
├── scripts/       # 迁移 SQL、端到端/视觉验证脚本、服务单元
├── web/           # Vue3 前端
└── docs/          # 全部文档
```

## 文档

| 文档 | 说明 |
| --- | --- |
| [docs/INSTALL.md](docs/INSTALL.md) | 安装与本地部署指南 |
| [docs/deploy/](docs/deploy/) | 部署手册：配置、数据库、短信、生产部署、Docker Compose、公网上线清单 |
| [docs/development.md](docs/development.md) | 开发指南：环境、架构、代码生成、测试 |
| [docs/ci-cd.md](docs/ci-cd.md) | CI 流水线、AI 审查、周报 |
| [docs/features-status.md](docs/features-status.md) | 功能项成熟度矩阵 |
| [docs/CHANGELOG.md](docs/CHANGELOG.md) | 本仓库变更记录 |
| [docs/governance.md](docs/governance.md) | 社区治理章程 |
| [docs/requirements-starisle-v2.0.md](docs/requirements-starisle-v2.0.md) | 平台需求基线 |
| [docs/openapi/](docs/openapi/) | OpenAPI 文档资源（`docs` tag 运行时由 `/docs/openapi` 提供） |

完整索引见 [docs/README.md](docs/README.md)。

## 致谢

- 上游项目 [rocboss/paopao-ce](https://github.com/rocboss/paopao-ce)（MIT License）

## 开源协议

[MIT](LICENSE)

## 祝愿所有舰长、旅行者、开拓者、绳匠

为世界上所有美好而战！

我们终将重逢。

原此行，终抵群星！

迎来到新艾利都。
