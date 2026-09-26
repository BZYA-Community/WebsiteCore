# web

前端（Vue 3 + Vite + TypeScript）。构建产物输出到 `dist/`，由后端 `web/embed.go` 默认内嵌进二进制（构建约束 `//go:build !(slim && embed)`，详见 [docs/INSTALL_ZH.md](../docs/INSTALL_ZH.md) 的「内嵌 Web UI」一节）。

## 开发

```sh
npm install    # 或 yarn / pnpm
npm run dev    # 启动 Vite 开发服务器
```

> 注意：仓库目前**不提交 lockfile**（被 `.gitignore` 排除），构建可复现性问题见 #36。

## 构建

```sh
npm run build  # 产出到 web/dist/
```

- 仓库根目录的 `make build-web` 等价于在本目录执行 `yarn build`（会先清空 `dist/`）。
- 产物供后端内嵌：`make build-web && make run`（前端默认已内嵌，无需 `TAGS`）。

## 常用脚本

| 命令 | 说明 |
| --- | --- |
| `npm run lint` / `npm run lint:fix` | ESLint 检查/修复 |
| `npm run check` / `npm run format` | Biome 检查/格式化 |
| `npm run preview` | 预览构建产物 |

前端相关说明也散落在根目录 [README.md](../README.md) 与 [docs/INSTALL_ZH.md](../docs/INSTALL_ZH.md) 中。
