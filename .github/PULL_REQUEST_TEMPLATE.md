## 对应 Issue / 提案

<!-- closes #xx；一个 PR 只对应一个工作流，跨工作流请拆 PR。关联提案：docs/proposal/<编号>-<名称>.md -->

## 改动说明

<!-- 按工作流列点：删了什么、改了什么口径、新增了什么。
     涉及生成代码(auto/)、迁移脚本、embed config.yaml 与 sample 双份、文档的，单列一行。 -->

## 自检清单

- [ ] `go build ./...`、`go vet ./...`、`go test ./...` 全绿
- [ ] gofmt 干净（CRLF 工作区用 LF-index 比对：`git ls-files '*.go' | grep -v '^auto/' | while read f; do cmp -s <(git show ":$f"|gofmt) <(git show ":$f") || echo DIRTY $f; done`）
- [ ] 动 `mirc/`：已跑 `go generate mirc/gen.go`，`auto/` 无手改痕迹
- [ ] 动迁移：方言齐备（或 Issue 明确单方言），本地带 `migration` tag 启动验证建表/变更成功
- [ ] 动 `web/`：`npx vite build` + `npx eslint src/` 0 error，且已恢复 `web/dist/.gitkeep`
- [ ] 动配置：`internal/conf/config.yaml`(embed) 与 `config.yaml.sample` 同步
- [ ] 无构建产物入提交（`paopao-*.exe`、`*.exe~` 已 gitignore）

## 回归结果

<!-- 贴脚本输出尾部计数行；未跑的要写明原因。与本 PR 无关的条目直接删除。 -->

- [ ] `scripts/test_audit_flow.py`：PASS=__ FAIL=__
- [ ] `scripts/test_course_flow.py`（两阶段）：PASS=__ FAIL=__
- [ ] `scripts/test_whisper_matrix.py`：PASS=__ FAIL=__
- [ ] `scripts/verify_sidebar_830.py`：ALL PASS

## 影响面

<!-- API 语义/配置项/go.mod 依赖净减/数据迁移；部署侧需要做什么（改 config、跑迁移、清 redis 缓存）。 -->

## 验证证据

<!-- 行为类改动附 curl 输出或 Playwright 截图(scripts/shots/)；纯删除附 grep 零命中结果。 -->
