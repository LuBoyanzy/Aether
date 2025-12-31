# Repository Guidelines

## 项目结构
- Go Hub：`internal/cmd/hub`，核心业务在 `internal/hub/**`。
-,Go Agent：`internal/cmd/agent`，通用逻辑在 `/agent/**`。
- 前端：`internal/site`（Vite + React + TypeScript），静态资源在 `internal/site/public/static/`。
- 发布与部署：Docker/K8s 在 `supplemental/docker`、`supplemental/kubernetes`；安装脚本在 `supplemental/scripts`。
- 品牌/常量：`aether.go`，Logo 在 `internal/site/public/static/logo.svg`。

## 开发与运行
- Hub（开发）：`APP_URL=http://localhost:19090 make dev-hub`
- 前端（开发）：`PORT=19091 make dev-server`
- Agent（开发）：`KEY="..." TOKEN="..." HUB_URL="http://localhost:19090" make dev-agent`
- Go 测试：`go test ./...`
- 前端检查：`cd internal/site && npm run lint` / `npm run format`（Biome），构建：`npm run build`。

## 代码风格与命名
- Go：提交前运行 `gofmt`；服务名、路径前缀统一用 `aether`（如 `/api/aether/...`、`aether-agent`）。
- TS/JS：遵循 Biome 默认风格；组件用 PascalCase，变量用 camelCase。
- 数据目录与路径：`/var/lib/aether-agent`、`~/.config/aether` 等。

## 测试规范
- 后端：标准 `testing` 框架，文件以 `_test.go` 结尾，鼓励表驱动测试。
- 前端：仅在涉及复杂逻辑时补充针对性测试，避免过度快照。

## 提交与 PR
- 使用中文
- Commit 风格：短句、现在时，常用前缀如 `chore: ...`、`docs: ...`、`refactor: ...`。
- 每次提交聚焦单一变更，必要时在正文写明动机或风险。
- PR 应包含变更摘要、已运行测试（如 `go test ./...`），以及 API/端口改动说明；UI 变更最好附截图/说明。

## 安全与配置
- 不硬编码密钥；使用 env/参数传入 KEY、TOKEN、HUB_URL。
- 默认端口：Hub 19090、前端 19091、Agent 45876；全部使用 `aether` 前缀服务/路径。
- 镜像与发布来自 `LuBoyanzy/Aether` GitHub Releases；已移除旧的 Beszel 镜像/代理依赖。
