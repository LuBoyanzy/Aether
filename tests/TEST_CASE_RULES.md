# 测试用例生成规范（Playwright）

最优先原则：全程使用中文（用例标题、断言信息、报告解读尽量中文化）。

## 目标
- 覆盖每个功能的核心流程、边界场景与异常处理。
- 稳定、可重复、可回归，优先可维护性与可读性。

## 目录结构（建议）
- `tests/` 为独立测试包（单独的 `package.json`），每个“导航栏入口/模块”一个子目录。
- 每个模块目录必须自包含（用例 + 配置 + 产物目录约定）：
  - `tests/api-tests/api.spec.ts`（API 断言）
  - `tests/api-tests/ui.spec.ts`（UI 断言）
  - `tests/api-tests/support.ts`（公共工具：登录、创建/清理数据等）
  - `tests/api-tests/playwright.config.ts`（模块专属 Playwright 配置）
  - `tests/api-tests/.env`（本地环境变量文件，必须忽略）
  - `tests/api-tests/test-results/`（测试产物输出，必须忽略）
  - `tests/api-tests/playwright-report/`（HTML 报告输出，必须忽略）
- 旧目录 `tests/playright/` 逐步迁移到新结构，新增用例统一使用上述目录。
- 禁止把测试配置放在 `internal/site/`：测试包独立运行，不依赖前端工程目录。

## 运行方式（建议）
- 首次安装依赖（一次性）：
  - `cd tests && npm install`
- 执行“接口管理(api-tests)”全部用例：
  - `cd tests && npm run test:api-tests`
- 仅执行 API / UI：
  - `cd tests && npm run test:api-tests:api`
  - `cd tests && npm run test:api-tests:ui`
- 或在仓库根目录执行（推荐，避免切目录）：
  - `npm --prefix tests run test:api-tests`

## 分层原则
- **API 层**：用 `APIRequestContext` 做数据准备/清理与接口断言。
- **UI 层**：覆盖关键用户路径与页面呈现（按钮、表单、表格、状态等）。
- **禁止**：仅用 UI 覆盖所有场景，或仅用 API 替代 UI 交互验证。

## 生成流程
1. 阅读后端接口实现（禁止猜测字段与返回格式）。
2. 先写 API 用例（创建/查询/更新/删除/执行）。
3. 再写 UI 用例（入口、导航、表单、列表、可视化、状态更新）。
4. 补充清理与回滚逻辑。

## 用例要求
- 每个功能最少覆盖：**创建、查询、更新、删除、执行/触发、历史/日志**。
- 全局配置类用例必须**读取旧值→修改→保存→恢复旧值**。
- 仅在必要时新增测试数据，且必须在测试结束时清理。

## 选择器与稳定性
- 优先 `getByRole` + 可访问名称；避免依赖 CSS 类名。
- 必须在操作前 `expect().toBeVisible()`，禁止盲等。
- 弹窗/确认框必须使用 `waitForEvent("dialog")` 明确处理。

## 环境变量（示例）
- `PLAYWRIGHT_BASE_URL`
- `PLAYWRIGHT_EMAIL`
- `PLAYWRIGHT_PASSWORD`
- `PLAYWRIGHT_CASE_URL`
- 本地可使用 `tests/api-tests/.env` 统一管理变量，避免在命令行明文输入；该文件必须加入 `.gitignore`，不得提交到仓库。

## 测试报告与产物
- 默认控制台输出结果（list reporter）。
- `api-tests` HTML 报告输出到：
  - `tests/api-tests/playwright-report/`
  - 查看命令：`cd tests && npm run report:api-tests`
- `api-tests` 失败上下文、截图、trace、视频等产物输出到：
  - `tests/api-tests/test-results/`
- `api-tests` 机器可读 JSON 报告输出到：
  - `tests/api-tests/test-results/results.json`
- `api-tests` 可读的简体中文汇总报告（从 JSON 生成）：
  - 生成命令：`cd tests && npm run report:api-tests:cn`
  - 一键命令（先跑用例再生成汇总）：`cd tests && npm run test:api-tests:cn`
  - 输出文件：`tests/api-tests/test-results/report.zh-CN.md`

## 禁止事项
- 不自动启动前后端服务。
- 不吞异常；失败要保留完整错误信息。
- 不猜测后端接口结构，必须先查后端代码。
