# 安装指南

[English](INSTALL.md) | 简体中文

本文档介绍 PaoPao 在本地体验、开发调试和自部署场景下的推荐安装方式。项目整体说明请参考 [README.md](../README.md)。

## 选择安装方式

| 场景 | 推荐方式 |
| --- | --- |
| 后端或前端开发 | [源码运行](#run-from-source) |
| 部署到服务器 | [发布二进制](#deploy-release-binary) |

## 环境要求

### 源码开发所需环境

- Go `1.24+`
- Node.js `20.19+` 或 `22.12+`
- Yarn `1.x`
- Docker 与 Compose（用于本地 PostgreSQL / Redis / Meilisearch 依赖栈）

### 关键文件

- `config.yaml.sample` - 标准配置模板（叠加在内置默认配置之上）
- `docker-compose.dev.yml` - 本地 PostgreSQL / Redis / Meilisearch 依赖栈
- `scripts/migration/{postgres,mysql}/` - 版本化迁移 SQL
- `scripts/paopao-mysql.sql` - MySQL 初始化脚本
- `scripts/paopao-postgres.sql` - PostgreSQL 初始化脚本

<a id="run-from-source"></a>

## 从源码运行

### 后端

1. 启动本地依赖栈（PostgreSQL、Redis、Meilisearch）：

   ```sh
   make deps-up
   ```

2. 复制配置模板并设置 JWT 密钥——`JWT.Secret` 留空会导致启动直接退出：

   ```sh
   cp config.yaml.sample config.yaml
   openssl rand -base64 32   # 将输出填入 JWT.Secret
   ```

3. 用内嵌迁移脚本建库：

   ```sh
   make migrate
   ```

4. 启动或构建后端：

   ```sh
   make run               # 前端默认已内嵌；先执行 `make build-web` 保证 web/dist 不是空目录
   ```

依赖栈将 `postgres:18.6`、`redis:7.4.11`、`getmeili/meilisearch:v1.54.0` 的端口全部绑定到 `127.0.0.1`，数据保存在命名卷中（`make deps-reset` 会清空）。`config.yaml.sample` 已按该栈配置：PostgreSQL、数据库 `websitecore`、账号/密码 `paopao`。

构建发布二进制：

```sh
make build
```

生成的二进制默认位于 `release/paopao`。

### Web 前端

```sh
cd web
cp .env .env.local
yarn
yarn dev
```

构建 Web 静态资源：

```sh
yarn build
```

### 内嵌 Web UI

前端资源**默认就是内嵌的**——`web/embed.go` 的构建约束是 `//go:build !(slim && embed)`，不加任何标签（或只加 `embed`）构建时都会把 `web/dist` 打进二进制；单独的 `embed` 标签是 no-op，只有 `slim embed` 组合才会排除前端资源。请先构建前端（新克隆的仓库里 `web/dist` 只有 `.gitkeep`），再运行：

```sh
make build-web
make run                 # 提供内嵌前端（需要 Features 中启用 Frontend:EmbedWeb）
make run TAGS='embed'    # 结果完全相同：单独加 embed 不改变任何行为
```

若使用 `make build TAGS='slim embed'`，二进制中不含前端资源且不会注册静态路由，此时需要自行（例如用 Nginx）提供 `web/dist`。

<a id="deploy-release-binary"></a>

## 部署发布二进制到服务器

推荐使用 `migration` 标签构建自包含的二进制：数据库迁移内嵌在二进制中；前端资源默认已内嵌（见上一节），因此 `embed` 标签可写可不写，写与不写结果相同。服务器上只需要二进制本身和 `config.yaml`。

```sh
# 1. 构建前端资源
make build-web

# 2. 构建发布二进制（本机平台）
make build TAGS='migration'        # TAGS='embed migration' 等价

# 或交叉编译 Linux amd64（纯 Go 驱动，无需 CGO）
make linux-amd64 CGO_ENABLED=0 TAGS='migration'
```

产物位于 `release/` 目录。部署步骤：

1. 将 `release/paopao`（Windows 下为 `paopao.exe`）与 `config.yaml` 上传到服务器同一目录。
2. 准备好依赖服务：数据库（MySQL/PostgreSQL）、Redis、Meilisearch，地址写入 `config.yaml`。
3. 启动服务：

```sh
./paopao serve
```

说明：

- 自动迁移需要两个条件同时满足：编译时带 `migration` 标签，且 `config.yaml` 的 `Features` 中声明 `"Migration"`（例如 `Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "Migration"]`）。满足后服务启动时会自动执行数据库迁移，无需手工建表。
- 若不带 `migration` 标签，schema 不会自动创建；请带上标签重新构建，或自行应用 `scripts/migration/` 下的版本化迁移。
- 附件等持久化数据默认保存在二进制所在目录的 `custom/` 下，注意备份。

## 常用构建标签

| 标签 | 作用 |
| --- | --- |
| `embed` | 单独使用是 no-op：只有同时指定 `slim`（`TAGS='slim embed'`）才会裁剪掉内嵌前端 |
| `slim` | 只在与 `embed` 组合时生效，见上一行 |
| `migration` | 在后端二进制中包含 migration 支持 |
| `docs` | 启用开发文档 / OpenAPI 服务 |

示例：

```sh
make build TAGS='migration'
make run TAGS='docs'
make run                       # 前端默认已内嵌
make build TAGS='slim embed'   # 构建不带内嵌前端的二进制
```

## 配置基础

启动时，PaoPao 会按以下顺序读取配置：

1. `./config.yaml`
2. `./custom/config.yaml`

先找到哪个文件，就使用哪个文件（查找顺序见 `internal/conf/setting.go` 的 `newViper`）。

注意：外部配置文件现在不再要求承载全部运行参数。PaoPao 会先加载内置默认配置，再叠加你本地的配置文件。

建议的职责划分：

- **Bootstrap YAML**：端口、Feature 组合、数据库、Redis、JWT、`AdminSettings.EncryptionKey`
- **管理后台**（`/#/admin/settings`）：大部分站点、搜索、存储、短信、支付以及应用行为类配置

如果某个配置项在后台中被标记为**重启后生效**，表示它会先持久化保存，但需要重启进程后才会真正切换到新值。

`Features` 用于控制不同能力组合：

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
  Develop: ["Base", "MySQL", "BigCacheIndex", "Meili", "Sms", "AliOSS", "LoggerMeili", "OSS:Retention"]
  Demo: ["Base", "MySQL", "Option", "Zinc", "Sms", "MinIO", "LoggerZinc", "Migration"]
  Slim: ["Base", "Postgres", "LocalOSS", "LoggerFile", "OSS:TempDir"]
```

常见命令：

```sh
# 使用默认套件
release/paopao serve

# 仅使用 develop 套件
release/paopao serve --no-default-features --features develop

# 在默认套件基础上增加一个功能
release/paopao serve --features sms

# 手动显式指定功能项
release/paopao serve --no-default-features --features postgres,localoss,loggerfile,redis
```

功能项成熟度与支持状态请参考 [features-status.md](features-status.md)。

## 可选基础设施服务

当前更推荐的默认组合是 **Meilisearch**、**Redis**，以及 **LocalOSS / MinIO / 云对象存储** 三选一。其他集成按需启用即可。

### Meilisearch（推荐搜索引擎）

本地开发时 `make deps-up` 已自动启动 Meilisearch。若需单独运行一个实例：

```sh
mkdir -p data/meili/data
docker run -d --name meili \
  -v ${PWD}/data/meili/data:/meili_data \
  -p 127.0.0.1:7700:7700 \
  -e MEILI_MASTER_KEY=paopao-meilisearch \
  getmeili/meilisearch:v1.54.0
```

对应配置示例：

```yaml
Meili:
  Host: 127.0.0.1:7700
  Index: paopao-data
  ApiKey: paopao-meilisearch
  Secure: False
```

### MinIO

```sh
mkdir -p data/minio/data
docker run -d --name minio \
  -v ${PWD}/data/minio/data:/data \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minio-root-user \
  -e MINIO_ROOT_PASSWORD=minio-root-password \
  -e MINIO_DEFAULT_BUCKETS=paopao:public \
  bitnami/minio:latest
```

对应配置示例：

```yaml
MinIO:
  AccessKey: Q3AM3UQ867SPQQA43P2F
  SecretKey: zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG
  Secure: False
  Endpoint: 127.0.0.1:9000
  Bucket: paopao
  Domain: 127.0.0.1:9000
```

### OpenObserve

```sh
mkdir -p data/openobserve
docker run -v ${PWD}/data/openobserve:/data \
  -e ZO_DATA_DIR=/data \
  -p 5080:5080 \
  -e ZO_ROOT_USER_EMAIL=root@paopao.info \
  -e ZO_ROOT_USER_PASSWORD=paopao-ce \
  public.ecr.aws/zinclabs/openobserve:latest
```

### Pyroscope

```sh
docker run -it -p 4040:4040 pyroscope/pyroscope:latest server
```

### Zinc（遗留 / 可选）

仓库中仍保留了 Zinc 相关代码与 Feature 定义，但当前默认推荐的搜索方案是 Meilisearch。只有在你明确需要兼容旧方案时，再考虑启用 Zinc。

## 本地启用 API 文档

在配置中加入 Docs 套件，并使用 `docs` 标签运行：

```yaml
Features:
  Default: ["Base", "Postgres", "Option", "LocalOSS", "LoggerFile", "Docs"]
  Docs: ["Docs:OpenAPI"]
```

```sh
make run TAGS='docs'
```

然后访问：

- `http://127.0.0.1:8011/docs/openapi`

## 安全建议

对外部署前的加固清单（漏洞报告流程见 [SECURITY.md](../SECURITY.md)）：

- **`JWT.Secret` 必填**：留空时进程会打印错误并直接退出。用 `openssl rand -base64 32` 生成随机值，写入 `config.yaml` / `custom/config.yaml`，不要提交到公开仓库；轮换该值会使所有已登录会话失效。
- **替换 sample 中的默认值**：`config.yaml.sample` 里 `App.RunMode: debug`、`AdminSettings.EncryptionKey: CHANGE-ME-TO-A-LONG-RANDOM-SECRET`——生产环境请改为 `RunMode: release` 并自行生成随机密钥（它用于加密后台设置项的落盘内容）。
- **不要把 API 端口直接暴露公网**：`WebServer.HttpIp`/`HttpPort` 只对反向代理开放；PostgreSQL、Redis、Meilisearch 保持在内网（sample 绑定 `127.0.0.1`，且 `paopao`/`paopao` 是仅限本地开发的弱口令，生产必须更换）。
- **上传在服务端已有校验**：仅接受 `public/image`、`public/video`、`public/avatar`、`attachment` 四种上传类型，`Content-Type` 白名单（`webp/png/jpg/gif/mp4/mov/zip`），单文件上限 100MB——反向代理的请求体大小限制需与之对齐，也不要让 `LocalOSS` 存储目录被宽松的文件服务器直接暴露。
- **TLS 在反向代理层终结**：Go 服务只提供明文 HTTP，请在前面放置 Nginx/Caddy 终止 HTTPS（并做限流），只对外暴露该入口。
- **保护数据目录**：`config.yaml` 与 `custom/` 目录包含密钥和用户数据，请收紧文件权限并加密备份。

## 更多部署文档

如果需要平台化或生产化部署参考，请继续阅读：

- [docs/deploy/README.md](deploy/README_ZH.md)
- [docs/deploy/core/](deploy/core/)
- [docs/deploy/local/](deploy/local/)
- [docs/deploy/k8s/](deploy/k8s/)
- [docs/deploy/aliyun/](deploy/aliyun/)
- [docs/deploy/huawei/](deploy/huawei/)
- [docs/deploy/tencent/](deploy/tencent/)

## 运维建议

- 对于长期运行环境，建议使用进程守护工具管理后端服务，并通过 Nginx 做反向代理。
- 示例配置中的短信通道使用 Juhe；如果不适合你的部署场景，可以替换为其他兼容服务商。
- 项目支持多种运行组合，请确保 `Features` 与你实际部署的基础设施保持一致。
