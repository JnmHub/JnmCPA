# JnmCPA 官方补全升级计划书

生成日期：2026-04-24  
适用仓库：`/Volumes/Jnm/Code/golang/CLIProxyAPI`  
参考基线：官方 `router-for-me/CLIProxyAPI` `v6.9.36`（2026-04-23 发布）  
前置报告：`OFFICIAL_UPGRADE_GAP_REPORT_2026-04-24.md`

## 1. 目标

在不丢失当前 fork 核心能力的前提下，分阶段补齐官方近期缺失的高价值功能与修复。

这次升级不是“全量回归官方”，而是“保留 fork 产品化能力，补齐官方高价值运行时能力”。

## 2. 总体策略

### 2.1 核心原则

1. 兼容性优先  
先补会影响客户端正常使用的能力，再补纯优化项。

2. 保留 fork 核心能力  
必须保留当前项目已有的：
- SQLite / Mongo 认证存储
- Qwen / IFlow provider
- 双管理密钥
- `/lijinmu` 管理页入口
- 认证文件批量探测、自动删除、统计、重置等管理能力
- 自己的最小化发布与安装链路

3. 不做全仓硬同步  
禁止直接用官方目录整体覆盖当前仓库。

4. 先补低冲突高收益，再补高冲突深层逻辑  
优先级顺序：
- 模型表
- 图片能力
- Responses 流式兼容
- Session affinity
- Claude 签名兼容
- Antigravity 深层逻辑

### 2.2 非目标

本轮不做：

- 恢复官方 Docker / docker-compose 主路线
- 恢复官方 `/management.html` 入口替代 `/lijinmu`
- 恢复官方远程自动拉取 panel 的策略
- 放弃当前前端源码入仓模式
- 删除 SQLite / Mongo / Qwen / IFlow / operator 角色能力

## 3. 需求摘要

### 3.1 必补官方缺口

1. GPT-5.5 模型支持
2. OpenAI / Codex 图片生成与编辑接口
3. `/v1/responses` SSE / WebSocket 兼容修复
4. Session affinity 会话粘性路由
5. Claude CCH signing 与相关兼容增强
6. Antigravity credits fallback
7. Gemini CLI 内部端点开关
8. 管理接口 `auth-index` 与重复 key 删除稳态修复
9. 回归测试与 PR 级 CI 守护补强

### 3.2 必须保留的 fork 能力

1. SQLite / Mongo store
2. Qwen provider
3. IFlow provider
4. operator / admin 双密钥与角色控制
5. 认证自动删除统计、批量探测、批量重置重试时间
6. 管理前端源码与当前页面功能
7. minimal Linux 安装与发布链路

## 4. 验收标准

### 4.1 功能验收

1. `GET /v1/models` 能返回 `gpt-5.5` 相关模型。
2. `POST /v1/images/generations` 和 `POST /v1/images/edits` 可用。
3. Codex 路径在需要时能自动注入 `image_generation` tool。
4. `/v1/responses` SSE 输出不再出现不完整 frame。
5. Responses WebSocket 在 tool call / compaction / transcript 替换场景下行为稳定。
6. 支持 `session-affinity` 和 `session-affinity-ttl`。
7. Claude OAuth 相关请求具备新版 signing / cch 兼容能力。
8. Antigravity 在 free-tier 耗尽后可按配置走 credits fallback。
9. `enable-gemini-cli-endpoint: false` 时不注册 `/v1internal:method`。
10. 管理接口支持稳定 `auth-index`，重复 key 删除行为更安全。

### 4.2 回归验收

1. SQLite store 正常读写、删除、订阅更新。
2. Mongo store 正常读写、删除、订阅更新。
3. 认证文件页面现有批量探测、删除统计、重试重置、单文件探测仍可用。
4. `/lijinmu` 入口和当前前端功能不回退。
5. Qwen / IFlow 登录与执行链路不被破坏。

### 4.3 工程验收

1. 新增或改动的关键路径都有对应测试。
2. 至少补上一条 PR 级自动测试 workflow。
3. 每个阶段都有可独立回滚的提交边界。

## 5. 工作流分期

## Phase 0：基线冻结与回归护栏

### 目标

在合并官方逻辑前，先把当前 fork 的关键行为锁住。

### 涉及文件

- `sdk/cliproxy/auth/conductor.go`
- `sdk/cliproxy/auth/selector.go`
- `internal/config/config.go`
- `internal/api/handlers/management/*`
- `internal/store/sqlitestore.go`
- `internal/store/mongostore.go`
- `internal/runtime/executor/qwen_executor.go`
- `internal/runtime/executor/iflow_executor.go`

### 实施内容

1. 为当前 fork 独有能力补回归测试：
- SQLite store
- Mongo store
- operator / admin 权限边界
- auth delete stats
- auth probe batch
- auth retry reset
- `/lijinmu` 页面入口

2. 记录当前关键配置兼容面：
- `sqlite-store`
- `mongo-store`
- `auth-auto-delete`
- `auth-probe-batch`
- `auth-probe-models`
- `retry-model-not-supported`
- `retry-thinking-validation-error`
- `remote-management.operator-secret-key`
- `remote-management.panel-title`

### 验收

- `go test` 可覆盖当前 fork 独有能力核心路径。
- 当前功能在升级前有明确“保护网”。

### 风险

- 如果不先做这一步，后续合官方代码时无法知道哪里是被误伤。

## Phase 1：低冲突高收益能力补齐

### 目标

先补最容易拿、收益最高、冲突最小的官方功能。

### 子项 1：GPT-5.5 模型注册

#### 涉及文件

- `internal/registry/models/models.json`
- `internal/registry/model_registry.go`
- `internal/registry/model_definitions.go`

#### 实施内容

1. 合并官方 `gpt-5.5` 相关模型定义。
2. 校验本地别名、provider 路由不会误判。

#### 验收

- 模型列表可见 `gpt-5.5`
- 请求能正确走现有路由

### 子项 2：OpenAI / Codex 图片接口

#### 涉及文件

- `internal/api/server.go`
- `sdk/api/handlers/openai/openai_images_handlers.go`
- `internal/runtime/executor/codex_executor.go`
- `internal/translator/codex/openai/chat-completions/codex_openai_response.go`
- `internal/translator/codex/gemini/codex_gemini_response.go`

#### 实施内容

1. 新增：
- `/v1/images/generations`
- `/v1/images/edits`

2. 合并官方图片 handler。
3. 补 Codex `image_generation` tool 注入。
4. 补转换器与测试。

#### 验收

- 图片生成 / 编辑接口可用
- 不影响现有 `/v1/chat/completions`、`/v1/responses`

### 本阶段复杂度

- 风险：中
- 推荐优先级：最高

## Phase 2：Responses 流式与 WebSocket 兼容补齐

### 目标

提升 `/v1/responses` 的官方兼容度和稳态表现。

### 涉及文件

- `sdk/api/handlers/openai/openai_responses_handlers.go`
- `sdk/api/handlers/openai/openai_responses_websocket.go`
- `sdk/api/handlers/openai/openai_responses_websocket_toolcall_repair.go`

### 实施内容

1. 合并官方 SSE frame buffering 逻辑。
2. 合并 WebSocket toolcall repair。
3. 处理：
- orphaned tool outputs
- 重复 `call_id`
- compaction 后 transcript 替换
- session 级 tool state

### 验收

- SSE 输出按合法 frame 发出
- WebSocket 下 tool call 状态稳定
- 不引入当前 fork 的批量探测逻辑回退

### 风险

- 中
- 这部分和你本地管理功能冲突不大，但和 `sdk/api/handlers/openai/*` 行为耦合较深

## Phase 3：Session affinity 与认证选择层升级

### 目标

补齐官方会话粘性路由，同时保留你当前的优先级和禁用逻辑。

### 涉及文件

- `internal/config/config.go`
- `config.example.yaml`
- `sdk/cliproxy/auth/selector.go`
- `sdk/cliproxy/auth/session_cache.go`
- `sdk/cliproxy/auth/types.go`
- `sdk/cliproxy/auth/scheduler.go`

### 实施内容

1. 合并配置项：
- `claude-code-session-affinity`
- `session-affinity`
- `session-affinity-ttl`

2. 合并 session cache。
3. 在 selector 中增加会话 ID 提取逻辑：
- `metadata.user_id`
- `X-Session-ID`
- `Idempotency-Key`
- `conversation_id`

4. 保留当前 fork 行为：
- `priority` 权重优先
- `disabled` / `cooldown` / `usable` 等本地状态语义
- 批量探测后的本地状态展示

### 验收

- 同会话请求优先复用同一 auth
- auth 不可用时能自动 failover
- 不破坏当前权重优先逻辑

### 风险

- 高
- 这是第一块高冲突区域

### 处理策略

- 不建议 cherry-pick
- 采用手工对照合并

## Phase 4：认证自动刷新调度升级

### 目标

把当前简单 ticker 刷新逻辑升级到官方的队列式 auto-refresh scheduler。

### 涉及文件

- `sdk/cliproxy/auth/conductor.go`
- `sdk/cliproxy/auth/auto_refresh_loop.go`
- `internal/config/config.go`

### 实施内容

1. 引入官方 `auto_refresh_loop.go`
2. 用队列式调度替代当前简单定时循环
3. 保留本地自定义状态、自动删除、管理探测逻辑
4. 兼容当前：
- SQLite / Mongo store
- auth delete stats
- probe batch

### 验收

- 自动刷新 worker 并发可控
- 大量 auth 下 CPU 不显著上涨
- 刷新与批量探测、自动删除不互相踩状态

### 风险

- 高
- 和你本地 conductor 改造重合度很高

## Phase 5：Claude 兼容层升级

### 目标

补齐官方最新 Claude Code 指纹兼容与 signing 逻辑。

### 涉及文件

- `internal/runtime/executor/claude_executor.go`
- `internal/runtime/executor/claude_signing.go`
- `internal/runtime/executor/helps/utls_client.go`
- `internal/config/config.go`
- `config.example.yaml`

### 实施内容

1. 合并官方 `claude_signing.go`
2. 增加 `experimental-cch-signing`
3. 引入 uTLS client 能力
4. 合并以下兼容行为：
- OAuth tool 名称映射
- `max_tokens` 补全
- thinking 流程下 temperature 归一
- cch 自动签名

5. 保留当前 fork 自定义：
- 本地 panel 与管理能力
- provider 扩展

### 验收

- Claude OAuth 请求头与 payload 更贴近官方行为
- 不破坏现有 Claude 用户可用性

### 风险

- 高
- 需重点回归 Claude 相关所有路径

## Phase 6：Antigravity 深层能力补齐

### 目标

补官方最近在 Antigravity 上的两块关键增强：
- 严格 signature validation
- credits fallback

### 涉及文件

- `internal/runtime/executor/antigravity_executor.go`
- `internal/translator/antigravity/claude/signature_validation.go`
- `sdk/cliproxy/auth/antigravity_credits.go`
- `sdk/cliproxy/auth/conductor.go`
- `internal/config/config.go`

### 实施内容

1. 增加严格 Claude thinking signature 校验。
2. 增加 bypass mode 下无效 thinking 块剥离。
3. 加入 `quota-exceeded.antigravity-credits`
4. 接入 credits hint 与 fallback retry。

### 验收

- free-tier 用尽时按配置回退到 credits
- signature validation 行为与官方对齐

### 风险

- 很高
- 这是整套升级中最容易引发运行时副作用的一块

### 策略

- 放在后面
- 必须独立提交、独立测试

## Phase 7：Gemini CLI 开关与管理接口稳态修复

### 目标

补齐两个小而有用的官方能力。

### 子项 1：Gemini CLI endpoint 开关

#### 涉及文件

- `internal/config/sdk_config.go`
- `internal/api/server.go`
- `config.example.yaml`

#### 实施内容

1. 引入 `enable-gemini-cli-endpoint`
2. 仅在开关开启时注册 `/v1internal:method`

#### 验收

- 配置为 `false` 时端点不存在
- 配置为 `true` 时端点可用

### 子项 2：管理接口硬化

#### 涉及文件

- `internal/api/handlers/management/config_auth_index.go`
- `internal/api/handlers/management/config_lists.go`

#### 实施内容

1. 返回稳定 `auth-index`
2. 删除重复 key 时结合 `base-url` 做更稳的匹配

#### 验收

- 管理前端能够稳定识别对象
- 重复 key 删除不会误删其他配置

## Phase 8：CI、测试与发布守护

### 目标

把升级后的行为尽量稳定住。

### 涉及文件

- `.github/workflows/`
- `test/`
- `sdk/api/handlers/openai/*_test.go`
- `sdk/cliproxy/auth/*_test.go`
- `internal/runtime/executor/*_test.go`

### 实施内容

1. 增加 PR 级 workflow：
- `go test` 核心模块
- frontend type-check / build

2. 选择性补官方 sentinel / usage tests
3. 保留你当前的 `release-minimal.yml`
4. 不恢复官方 Docker workflow 为主线

### 验收

- PR 变更能自动跑核心测试
- 发布 workflow 与 PR workflow 分工清晰

## 6. 任务优先级矩阵

### P0：立刻做

1. Phase 0 基线护栏
2. Phase 1 GPT-5.5
3. Phase 1 图片接口
4. Phase 2 Responses 修复

### P1：第二批做

1. Phase 3 Session affinity
2. Phase 4 自动刷新调度
3. Phase 5 Claude 兼容升级

### P2：最后做

1. Phase 6 Antigravity credits fallback
2. Phase 7 Gemini CLI 开关
3. Phase 7 管理接口硬化
4. Phase 8 CI / sentinel tests

## 7. 实施顺序建议

### 推荐顺序

1. 建护栏
2. 模型表
3. 图片接口
4. Responses SSE / WS 修复
5. Session affinity
6. auto refresh scheduler
7. Claude signing / uTLS
8. Antigravity signature / credits
9. Gemini CLI 开关
10. 管理接口硬化
11. CI 补强

## 8. 每阶段提交策略

每个阶段单独提交，禁止把多个高风险点混成一个大提交。

### 推荐提交边界

1. `models: add gpt-5.5 registry support`
2. `openai: add images generation and edit handlers`
3. `responses: repair SSE and websocket toolcall state`
4. `auth: add session-affinity routing with TTL cache`
5. `auth: replace refresh ticker with queued auto-refresh loop`
6. `claude: add signing and compatibility updates`
7. `antigravity: add signature validation and credits fallback`
8. `management: harden config list identifiers and duplicate key delete`
9. `ci: add PR test workflow and regression coverage`

## 9. 风险清单

### 风险 1：`conductor.go` 冲突爆炸

原因：

- 官方与 fork 都大量改过

应对：

- 先补测试
- 分功能手工合并
- 绝不整文件覆盖

### 风险 2：前端适配不上后端字段变化

原因：

- 你当前前端源码是自己维护的

应对：

- 后端字段新增尽量向后兼容
- 每阶段都检查 `static/Cli-Proxy-API-Management-Center-main` 是否需要同步改动
- 每次都重建并同步 `static/management.html`

### 风险 3：SQLite / Mongo 与官方新调度逻辑互相打架

原因：

- 官方主路径并不以这两个 store 为主

应对：

- store 层独立回归测试
- conductor 和 selector 修改后单独验证 DB-backed auth 行为

### 风险 4：Claude / Antigravity 升级引入隐性兼容回退

原因：

- 上游这些改动直接影响请求内容、签名和头部

应对：

- 分阶段启用
- 新增配置开关
- 优先 default false，再逐步切换

## 10. 建议的文件归类

### 可以优先参考、相对容易移植

- `CLIProxyAPI-main/internal/registry/models/models.json`
- `CLIProxyAPI-main/sdk/api/handlers/openai/openai_images_handlers.go`
- `CLIProxyAPI-main/sdk/api/handlers/openai/openai_responses_websocket_toolcall_repair.go`
- `CLIProxyAPI-main/internal/api/handlers/management/config_auth_index.go`

### 需要手工 merge，不能直接覆盖

- `CLIProxyAPI-main/internal/api/server.go`
- `CLIProxyAPI-main/internal/config/config.go`
- `CLIProxyAPI-main/sdk/cliproxy/auth/conductor.go`
- `CLIProxyAPI-main/sdk/cliproxy/auth/selector.go`
- `CLIProxyAPI-main/internal/runtime/executor/claude_executor.go`
- `CLIProxyAPI-main/internal/runtime/executor/antigravity_executor.go`

### 明确保留 fork 自己实现

- `internal/store/sqlitestore.go`
- `internal/store/mongostore.go`
- `internal/auth/qwen/*`
- `internal/auth/iflow/*`
- `internal/runtime/executor/qwen_executor.go`
- `internal/runtime/executor/iflow_executor.go`
- `install.sh`
- `installer/*`
- `static/Cli-Proxy-API-Management-Center-main/*`
- `/lijinmu` 管理页入口逻辑

## 11. 验证计划

### 单元测试

- model registry
- image handlers
- responses SSE
- responses websocket tool repair
- session cache
- selector session affinity
- conductor refresh scheduler
- claude signing
- antigravity credits hint / fallback
- config list hardening

### 集成测试

- `/v1/models`
- `/v1/images/generations`
- `/v1/images/edits`
- `/v1/responses`
- `/backend-api/codex/responses`
- `/v1internal:method` 开关控制
- DB-backed auth 管理与运行时读取

### 手工验证

- `/lijinmu` 页面
- 单文件测试
- 批量测试
- 批量重置重试时间
- 下载 / 导出 / 上传 zip
- 权重优先级
- Quota 弹窗
- operator 角色权限边界

## 12. 交付物

本计划执行完成后，应该交付：

1. 一组分阶段可回滚提交
2. 更新后的 `config.example.yaml`
3. 更新后的管理前端构建产物
4. 新增测试与 PR workflow
5. 更新后的升级报告与变更记录

## 13. 推荐的实际执行模式

这项工作建议分 4 轮推进：

### 第 1 轮

- Phase 0
- Phase 1
- Phase 2

### 第 2 轮

- Phase 3
- Phase 4

### 第 3 轮

- Phase 5
- Phase 6

### 第 4 轮

- Phase 7
- Phase 8

## 14. 最终建议

如果要开始实做，最合理的开工顺序是：

1. 先做 Phase 0 护栏
2. 立即补 Phase 1 和 Phase 2
3. 然后进入 Phase 3 和 Phase 4
4. Claude / Antigravity 放在后半程

这样做的原因很简单：

- 前两轮可以先把最明显的官方兼容缺口补上
- 中段再解决认证调度这类高冲突问题
- 最后再碰 Claude / Antigravity 这类最深、最容易引发副作用的逻辑

---

下一份如果继续做，建议直接出：

`OFFICIAL_UPGRADE_PHASE1_TASKLIST.md`

专门把 Phase 0 到 Phase 2 拆成逐文件任务单，进入可直接编码执行状态。
