# JnmCPA 与官方 CLIProxyAPI 升级差异报告

生成时间：2026-04-24  
当前仓库：`JnmHub/JnmCPA` 本地工作区  
当前本地 HEAD：`de68cfd3688ba0eb918814231e01e997da9f40eb`  
官方对照仓库：[`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI)  
官方最新 Release：[`v6.9.36`](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v6.9.36)  
官方最近确认提交：`f1ba6151a99240902bcda12102c921b0ead01d2d`

## 一句话结论

你的项目不是“单纯落后几次小更新”，而是已经和官方形成了明显分叉。

- 管理端、部署方式、认证文件管理、存储实现，你这边改得很多。
- 协议兼容、模型注册、会话粘性、Codex 图片能力、Antigravity credits 回退，这些官方新能力你这边又确实缺了一部分。
- 所以这次不适合按“直接跟官方同步”理解，更适合按“挑重点功能定向回合并”来做。

## 对照基线说明

这次对照不是只看 README，而是同时看了三层基线：

- 你当前仓库源码。
- 仓库里的 `CLIProxyAPI-main/` 官方原版快照目录。
- 官方 GitHub 当前最新状态与最新 Release。

说明：

- `CLIProxyAPI-main/` 的目录结构和关键文件内容与官方当前主线高度一致，可作为本地对照快照。
- 你的仓库历史已经明显分叉，所以“落后多少 commits”这个指标不再准确，也不适合作为升级难度判断依据。
- 更有意义的是“缺哪些官方能力、保留哪些你自己的能力、哪些文件已经高风险分叉”。

## 差异量化

### 全量文件层面

- 当前仓库参与对比文件数：`771`
- 官方快照参与对比文件数：`510`
- 当前仓库独有文件：`346`
- 官方独有文件：`85`
- 同路径但内容不同的文件：`122`

这个数字会被你额外导入的前端源码明显放大。

### 排除前端源码和打包产物后的核心对比

- 当前仓库核心文件数：`482`
- 官方核心文件数：`504`
- 当前仓库核心独有文件：`57`
- 官方核心独有文件：`79`
- 同路径但内容不同的核心文件：`122`

### 只看 Go 源码

- 你这边独有源码文件：`34`
- 官方独有源码文件：`25`
- 同路径但内容不同的源码文件：`77`

这说明当前分叉已经进入“核心代码层面”，不是单纯文档或脚本差异。

## 你当前项目缺失的官方重要能力

下面这些不是小修小补，而是真正有升级价值的官方能力或修复。

### 1. GPT-5.5 模型注册支持缺失

官方最近在 `v6.9.36` 明确加入了 GPT-5.5 Codex 支持。

- 官方证据：
  - `CLIProxyAPI-main/internal/registry/models/models.json`
  - 官方最近提交：`736018a0`、`7d5f6d93`、`f1ba6151`
- 当前状态：
  - 你的 `internal/registry/models/models.json` 中没有任何 `gpt-5.5` 记录。

影响：

- 你当前项目无法原生列出或正确路由 GPT-5.5 相关模型。

### 2. OpenAI / Codex 图片生成能力缺失

官方已经把图片生成接到了 OpenAI 兼容层和 Codex 路径。

- 官方证据：
  - `CLIProxyAPI-main/internal/api/server.go`
  - `CLIProxyAPI-main/sdk/api/handlers/openai/openai_images_handlers.go`
  - `CLIProxyAPI-main/internal/runtime/executor/codex_executor.go`
- 官方已具备：
  - `/v1/images/generations`
  - `/v1/images/edits`
  - Codex 请求自动补 `image_generation` tool
  - 图片相关返回转换和测试
- 当前状态：
  - 你的 `internal/api/server.go` 没有图片路由。
  - 你的仓库里没有 `sdk/api/handlers/openai/openai_images_handlers.go`。
  - 你的 `internal/runtime/executor/codex_executor.go` 没有 `image_generation` 注入逻辑。

影响：

- 你当前项目落后于官方的 Codex 图片能力兼容。
- 如果上游客户端开始依赖这些能力，你这边会直接表现为功能缺失，而不是仅仅“效果差一点”。

### 3. Antigravity credits 回退机制缺失

这是官方最近一批比较实用的运行时改动。

- 官方证据：
  - `CLIProxyAPI-main/sdk/cliproxy/auth/antigravity_credits.go`
  - `CLIProxyAPI-main/internal/runtime/executor/antigravity_executor.go`
  - `CLIProxyAPI-main/sdk/cliproxy/auth/conductor.go`
  - `CLIProxyAPI-main/internal/config/config.go`
- 官方配置项：
  - `quota-exceeded.antigravity-credits`
- 当前状态：
  - 你的仓库没有 `antigravity_credits.go`
  - 没有 `enabledCreditTypes=["GOOGLE_ONE_AI"]` 注入逻辑
  - 配置中没有 `quota-exceeded.antigravity-credits`

影响：

- 当 Antigravity 免费额度耗尽时，官方可以做一层 credits 回退。
- 你当前版本没有这套兜底逻辑。

### 4. 官方的会话粘性路由能力在你这边缺失

这是我认为最值得关注的上游差异之一。

- 官方证据：
  - `CLIProxyAPI-main/internal/config/config.go`
  - `CLIProxyAPI-main/sdk/cliproxy/auth/selector.go`
  - `CLIProxyAPI-main/sdk/cliproxy/auth/session_cache.go`
- 官方具备：
  - `claude-code-session-affinity`
  - `session-affinity`
  - `session-affinity-ttl`
  - 从 `metadata.user_id`、`X-Session-ID`、`Idempotency-Key`、`conversation_id` 等抽取会话标识
  - TTL session cache
- 当前状态：
  - 你的 `internal/config/config.go` 里 `RoutingConfig` 只剩 `strategy`
  - 你的 `sdk/cliproxy/auth/selector.go` 没有 session-affinity 相关实现
  - 你的仓库没有 `sdk/cliproxy/auth/session_cache.go`

影响：

- 多轮对话、多账户轮询、工具调用链路的一致性会比官方差。
- 这不是 UI 级差异，是认证选择层的运行时行为差异。

### 5. Claude CCH signing 新实现和配置项缺失

官方对 Claude Code 指纹对齐又往前走了一步。

- 官方证据：
  - `CLIProxyAPI-main/internal/runtime/executor/claude_signing.go`
  - `CLIProxyAPI-main/internal/runtime/executor/claude_executor.go`
  - `CLIProxyAPI-main/config.example.yaml`
- 官方配置项：
  - `experimental-cch-signing`
- 当前状态：
  - 你的仓库没有 `internal/runtime/executor/claude_signing.go`
  - `config.example.yaml` 里没有 `experimental-cch-signing`
  - 你当前仍是旧版内联 billing header 逻辑

影响：

- 在 Claude Code 指纹兼容这块，你当前项目落后于官方。

### 6. 官方 Responses 流式链路修复没有并进来

官方最近不只是加新接口，还修了一批 `/v1/responses` 的流式细节。

- 官方证据：
  - `CLIProxyAPI-main/sdk/api/handlers/openai/openai_responses_handlers.go`
  - `CLIProxyAPI-main/sdk/api/handlers/openai/openai_responses_websocket.go`
  - `CLIProxyAPI-main/sdk/api/handlers/openai/openai_responses_websocket_toolcall_repair.go`
- 当前状态：
  - 你的 `sdk/api/handlers/openai/openai_responses_handlers.go` 仍是较简单的 chunk 直写逻辑。
  - 你的仓库没有 `openai_responses_websocket_toolcall_repair.go`。
  - 你的 `sdk/api/handlers/openai/openai_responses_websocket.go` 也没有官方那套 session 级 tool output 修补和去重逻辑。

影响：

- 在 Responses streaming / websocket 下，官方现在对 SSE framing、tool call 状态修复、重复 `call_id`、compaction 后 transcript 替换的处理更完整。
- 你当前版本在这块兼容性和稳态行为上落后。

### 7. 官方认证层新增的一些调度和元数据能力没有并进来

这部分是认证调度层面的新能力，不是简单重构。

- 官方独有文件：
  - `CLIProxyAPI-main/sdk/cliproxy/auth/auto_refresh_loop.go`
  - `CLIProxyAPI-main/sdk/cliproxy/auth/custom_headers.go`
  - `CLIProxyAPI-main/sdk/cliproxy/auth/session_cache.go`
- 当前状态：
  - 你的仓库没有这些文件，也没有同名功能实现。

影响：

- 你会缺少官方对认证自动刷新调度、metadata 自定义 header 注入、session TTL cache 的一部分上游改进。

### 8. Gemini CLI 内部端点开关能力缺失

官方已经把 Gemini CLI 内部端点是否开放做成了配置项。

- 官方证据：
  - `CLIProxyAPI-main/internal/config/sdk_config.go`
  - `CLIProxyAPI-main/internal/api/server.go`
- 官方配置项：
  - `enable-gemini-cli-endpoint`
- 当前状态：
  - 你的 `internal/api/server.go` 仍然直接注册 `/v1internal:method`
  - 当前配置没有对应开关生效逻辑

影响：

- 官方现在可以按部署需求关闭这条内部端点。
- 你当前版本在安全面和暴露面控制上落后一拍。

### 9. 官方管理接口稳态修复没有并进来

官方对管理接口返回和重复 key 删除逻辑又做了一层硬化。

- 官方证据：
  - `CLIProxyAPI-main/internal/api/handlers/management/config_auth_index.go`
  - `CLIProxyAPI-main/internal/api/handlers/management/config_lists.go`
- 官方具备：
  - 稳定 `auth-index` 返回
  - 删除重复 API key 时要求结合 `base-url` 识别，避免误删
- 当前状态：
  - 你的 `internal/api/handlers/management/config_lists.go` 仍是更简单的返回和删除逻辑

影响：

- 在存在重复 key / 多上游配置时，官方管理接口的行为更稳。

### 10. 官方测试与 CI 守护能力缺失

这个差异不会直接表现成“新功能没有”，但会影响你后续继续二开的稳定性。

- 官方有：
  - `.github/workflows/pr-test-build.yml`
  - `.github/workflows/docker-image.yml`
  - `.github/workflows/agents-md-guard.yml`
  - `.github/workflows/pr-path-guard.yml`
  - `test/usage_logging_test.go`
  - `test/claude_code_compatibility_sentinel_test.go`
  - `test/testdata/claude_code_sentinels/*`
- 当前状态：
  - 这些多数不存在。

影响：

- 你现在更依赖人工验证。
- 后面继续改运行时协议兼容时，回归风险会高于官方。

## 你当前项目领先官方的本地定制能力

这部分也很重要，因为不是所有官方变化都值得直接覆盖。

### 1. 你已经做了 SQLite / Mongo 双存储扩展

当前仓库独有：

- `internal/store/sqlitestore.go`
- `internal/store/mongostore.go`
- `config.example.yaml` 中的：
  - `sqlite-store`
  - `mongo-store`

官方当前没有这套认证文件数据库化存储方案。

意义：

- 这是你当前 fork 最明确的实用增强之一。
- 如果你管理大量认证文件，这比官方原版更贴合你的实际场景。

### 2. 你把认证文件管理做得比官方更重

当前仓库独有：

- `internal/api/handlers/management/auth_delete_stats.go`
- `internal/api/handlers/management/auth_probe_batch.go`
- `internal/api/handlers/management/auth_retry_reset.go`
- 配置项：
  - `auth-auto-delete`
  - `auth-probe-batch`
  - `auth-probe-models`
  - `auth-upload`
  - `retry-model-not-supported`
  - `retry-thinking-validation-error`

这说明你的 fork 不是“简单换皮”，而是明显加强了认证文件运维能力。

### 3. 你本地多了 Qwen 和 IFlow 两个 provider 链路

当前仓库独有：

- `internal/auth/qwen/*`
- `internal/auth/iflow/*`
- `internal/runtime/executor/qwen_executor.go`
- `internal/runtime/executor/iflow_executor.go`
- `sdk/auth/qwen.go`
- `sdk/auth/iflow.go`

官方当前没有这组 provider 代码。

### 4. 你已经做了双管理密钥、可改标题、定制入口路由

当前仓库独有配置：

- `remote-management.operator-secret-key`
- `remote-management.panel-title`

当前仓库改动：

- 管理页路由从官方 `/management.html` 改成了 `/lijinmu`

这属于明显的产品化定制，而不是单纯技术跟随。

### 5. 你已经把管理前端源码直接纳入仓库

当前仓库独有：

- `static/Cli-Proxy-API-Management-Center-main/`
- 自己的前端构建与打包链路

意义：

- 你可以直接按自己的业务节奏改管理页。
- 但代价是后续每次跟官方 UI 同步都会更重。

### 6. 你已经形成了自己的最小部署和发版体系

当前仓库独有：

- `install.sh`
- `installer/`
- `build-binaries.sh`
- `prepare-minimal-package.sh`
- `package-project-zip.sh`
- `push-and-release.sh`
- `.github/workflows/release-minimal.yml`

这套体系和官方 Docker / GoReleaser 路线已经完全不是一条线。

## 分叉最严重、最容易冲突的区域

这些地方如果以后要合并官方更新，冲突概率最高。

### 1. `internal/config/config.go`

原因：

- 官方在这里新增了 session-affinity、antigravity-credits、panel auto-update 等配置。
- 你在这里新增了 sqlite/mongo、operator key、panel title、auth probe、auth upload、auto delete 等配置。
- 双方都在改同一个核心配置文件。

### 2. `internal/api/server.go`

原因：

- 官方新增了 `/v1/images/*`、`/management.html`、Codex direct route 相关能力。
- 你这里改了 `/lijinmu` 路由和管理页策略。

### 3. `sdk/cliproxy/auth/*`

重点文件：

- `sdk/cliproxy/auth/conductor.go`
- `sdk/cliproxy/auth/selector.go`
- `sdk/cliproxy/auth/scheduler.go`
- `sdk/cliproxy/auth/types.go`

原因：

- 官方在这层加了 session cache、credits hint、auto refresh loop。
- 你在这层加了优先级、删除策略、状态测试、重试策略等改动。

### 4. `internal/runtime/executor/*`

原因：

- 官方最近重点在这里加图片生成、credits fallback、CCH signing。
- 你在这里已经有不少自定义辅助逻辑、provider 扩展和运行时改动。

### 5. 管理端与部署链路

重点目录：

- `static/`
- `README.md`
- `.github/workflows/`
- `install.sh`
- `installer/`

原因：

- 这部分已经是“你自己的发行版体系”，和官方不是同一产品形态了。

## 升级难度判断

总体判断：**中高到高**

不是因为官方改得太大，而是因为：

- 官方和你都在改核心配置、认证调度、执行器、管理接口。
- 你已经新增了自己的数据库、管理端、安装器和前端源码。
- 很多文件属于同路径双向演化，不是简单覆盖能解决。

所以不建议：

- 直接整体覆盖官方源码
- 直接暴力 rebase
- 直接把 `CLIProxyAPI-main/` 整个拷过来

## 最合理的升级策略

建议采用“按价值定向并入”的方式，而不是全仓同步。

### 第一优先级：建议尽快并入

1. GPT-5.5 模型注册支持
2. OpenAI / Codex 图片生成链路
3. Session affinity 会话粘性路由
4. Claude CCH signing 新实现

原因：

- 这些直接影响客户端兼容性和运行行为。

### 第二优先级：按你是否实际在用再决定

1. Antigravity credits fallback
2. auth auto refresh loop
3. metadata custom headers
4. websocket toolcall repair

原因：

- 这些有价值，但不是所有部署都立刻需要。

### 第三优先级：可以不并

1. 官方 Docker 相关产物
2. 官方 GoReleaser 路线
3. 官方 `/management.html` 默认入口
4. 官方 panel 自动更新机制

原因：

- 这些你当前明显已经有自己的路线，而且不是你现在的核心痛点。

## 我建议的具体升级顺序

### 路线 A：最实用、风险最低

1. 先只合并 `internal/registry/models/models.json` 的 GPT-5.5 相关内容。
2. 再合并 OpenAI 图片处理链路：
   - `internal/api/server.go`
   - `sdk/api/handlers/openai/openai_images_handlers.go`
   - `internal/runtime/executor/codex_executor.go`
   - 相关 translator / tests
3. 再恢复 session affinity：
   - `internal/config/config.go`
   - `sdk/cliproxy/auth/selector.go`
   - `sdk/cliproxy/auth/session_cache.go`
4. 最后看是否合并 Claude signing 和 Antigravity credits。

适用场景：

- 你优先想补协议兼容，不想大动你自己的管理系统和部署方式。

### 路线 B：如果你后面准备长期跟官方

1. 保留你自己的 `static/`、`installer/`、`install.sh`、`README.md`
2. 核心运行时尽量向官方靠拢：
   - `internal/config`
   - `sdk/cliproxy/auth`
   - `internal/runtime/executor`
   - `internal/registry`
3. 管理端继续保持你自己的二开，不追官方 panel 分发模式

适用场景：

- 你以后还会频繁吃官方运行时修复，但不想丢掉自己这套产品化改造。

## 不建议的操作

- 不建议把官方 README、Docker、workflow 全量覆盖到你仓库。
- 不建议把官方 `internal/config/config.go` 整个替换掉。
- 不建议直接丢掉你现在的 SQLite / Mongo 支持。
- 不建议把你已经维护起来的前端源码再退回到“仅下载 dist 产物”的模式，除非你明确不想自己维护前端。

## 最终结论

这次对照的结果可以归纳成三句话：

- 你的项目在“认证文件管理、数据库存储、最小化部署、前端掌控力”上已经明显走到了官方前面。
- 你的项目在“官方最新协议兼容和运行时细节修复”上，已经落后了一批关键点。
- 最合适的升级方式不是“跟官方全量同步”，而是“把官方高价值运行时能力定向并进来”，优先补：
  - GPT-5.5
  - 图片生成链路
  - session affinity
  - Claude signing

## 本次对照中最值得你立刻关注的 4 个官方缺口

1. `gpt-5.5` 还没进你这边模型表  
2. `/v1/images/*` 和 Codex `image_generation` 还没进你这边  
3. session-affinity 在你这边实际上是缺失状态  
4. Claude CCH signing 仍停留在旧实现

---

如果你要，我下一步可以直接继续做第二份文档：  
`UPGRADE_TODO_FROM_OFFICIAL.md`

内容会是可执行清单，按“先改哪些文件、后改哪些文件、每一步可能冲突在哪里”拆出来。
