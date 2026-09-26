# 使用 Kubernetes 部署（占位 / TODO）

> **状态：占位文档。** 本仓库目前**不包含任何 Kubernetes 清单**（Deployment/Service/ConfigMap 等），也**没有生产用 `Dockerfile`**。本页只列出自行落地 K8s 部署时需要准备的事项，**不是**可直接照做的步骤。
> 上游 paopao-ce 的 [`docs/deploy/k8s`](https://github.com/rocboss/paopao-ce/tree/main/docs/deploy/k8s) 同样只是两行占位说明（上游有 `Dockerfile` 但无 k8s 清单），暂无可参考的上游清单。

## 自行部署需要准备什么

1. **镜像 / 二进制**
   - 本仓库没有 `Dockerfile`：可以参考上游 [Dockerfile](https://github.com/rocboss/paopao-ce/blob/main/Dockerfile) 自行编写并在本仓库验证，或直接使用静态二进制：

     ```sh
     make build-web                                  # 构建前端产物到 web/dist（构建时默认内嵌）
     make linux-amd64 CGO_ENABLED=0 TAGS='migration' # 交叉编译 → release/linux-amd64/paopao-ce/paopao
     ```

     （`linux-amd64` 等目标的输出路径见根目录 `Makefile` 中的 `RELEASE_LINUX_AMD64`；若要在集群里跑 `./paopao migrate`，二进制需带 `migration` 标签。）

   - 注意：`make buildx` 只是 `go mod download && go build`（本机平台），**不是**镜像构建。
2. **配置**：准备外部配置文件（按 `./config.yaml` → `./custom/config.yaml` 顺序查找，取先找到的一份），覆盖二进制内嵌默认值；模板见 [`config.yaml.sample`](../../../config.yaml.sample)。`JWT.Secret` 必填；PostgreSQL/Redis/Meilisearch 的地址要指向集群内 Service；`Features` 需与实际提供的组件一致（默认 `["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]`）。
3. **数据库迁移**：两种方式都要求二进制带 `migration` 构建标签——以 `migration` 标签构建后，用一次性 Job 执行 `./paopao migrate`；或在 `Features` 中加入 `"Migration"`，由服务启动时自动迁移（标签缺失时启动会报 “lacks the `migration` build tag”；也可在裸机上用 `make migrate`，详见 [../../INSTALL_ZH.md](../../INSTALL_ZH.md)）。
4. **持久化**：附件等数据保存在 `custom/` 目录，需要挂载 PV；数据库与搜索服务也各自需要持久卷。
5. **暴露与安全**：集群内只暴露 HTTP 端口，由 Ingress 终结 TLS。上线前请阅读 [../../INSTALL_ZH.md](../../INSTALL_ZH.md) 中的「安全建议」一节。

## 待办

- [ ] 提供可验证的镜像构建（`Dockerfile`）
- [ ] 提供 Kubernetes 清单 / Helm Chart
- [ ] 在真实集群中跑通并验证整套流程

欢迎在验证后向本目录补充内容。
