# AgentPulse 全面代码 Review 报告

> 审查日期: 2026-07-09
> 审查范围: 后端 (Go) / Python SDK / Web (Next.js) / Deploy / Docs 全量
> 审查维度: 现代性、规范性、安全性、可维护性、易拓展性
> 状态: Critical+High 修复中,Medium/Low 记录在案

---

## 0. 总体评估

| 维度 | 评分 | 说明 |
|------|------|------|
| 现代性 | 6/10 | Go 1.25 / Next 15 / Python 3.10+ 起步正确,但未充分利用 Generics/slog/zod/RSC |
| 规范性 | 5/10 | 缺 ESLint/Prettier/pytest 覆盖;部分手写工具类本应使用 stdlib |
| 安全性 | **2/10** | **生产前必修**:无认证、CORS 错配、OTLP 无限制、密码明文、DSN 日志泄漏、敏感数据默认捕获 |
| 可维护性 | 5/10 | 分层清晰但 Container 大杂烩、Service 重复 reverse-dep 具体实现 |
| 易拓展性 | 6/10 | Repository 模式标准、Judge 接口可替换、SDK Adapter 已成雏形 |
| **综合** | **5/10** | MVP 早期可接受,距离生产标准差距大 |

**问题总数**:Critical 18 项 / High 30+ 项 / Medium 60+ 项 / Low 30+ 项,合计约 **140+ 项**。

---

## 1. 后端 (Go) Review

### 1.1 CRITICAL 级别

#### C-BE-1 [CRITICAL] `/api/v1/*` 完全无鉴权
- **位置**: `backend/internal/api/router.go:37-78`
- **问题**: `v1 := r.Group("/api/v1")` 之后未挂载 `AuthMiddleware`(已实现但从未注册),所有端点对公网开放,可读写全部 Trace/成本/评估/Harness 数据。
- **修复**: 配置加载 API Key 列表(sha256 hash),`AuthMiddleware` 真正校验,挂到 v1 group。

#### C-BE-2 [CRITICAL] OTLP HTTP 接收端无认证/无大小限制/无超时
- **位置**: `backend/internal/collector/otlp_http.go:53-101` + `backend/internal/app/app.go:144-148`
- **问题**: OTLP server 零中间件,`io.ReadAll` 无限制,任何匿名客户端可推送任意大 payload 撑爆内存。
- **修复**: 加 `MaxBytesReader` 限制(默认 10MB),加 X-AgentPulse-Key 校验,加 read/write timeout,加 panic recover。

#### C-BE-3 [CRITICAL] `EvaluateNow` 端点无鉴权将 PII 上送 OpenAI
- **位置**: `backend/internal/api/eval_handler.go:83-107`
- **问题**: 任意人调用触发 LLM 评估,Span 中的 `input_preview`/`output_preview` 原样送 Judge,造成 PII 泄露。
- **修复**: 鉴权 + PII 脱敏 (email/phone/token) 后再送 Judge。

#### C-BE-4 [CRITICAL] DSN 含明文密码,日志可泄漏
- **位置**: `backend/internal/config/config.go:67-82, 97-107`
- **问题**: `PostgresConfig.DSN()` 与 `ClickHouseConfig.DSN()` 直接拼接密码,任何包含 DSN 的错误日志都泄漏。
- **修复**: DSN 改用 `url.URL` + `Redacted()` 方法,日志只显示 `user:****@host` 形式。

#### C-BE-5 [CRITICAL] 默认密码 `changeme` 无启动校验
- **位置**: `backend/internal/config/config.go:226` + `deploy/docker-compose.yml:56`
- **问题**: 生产环境忘改密码直接启动,PG 5432 端口对公网开放。
- **修复**: `Mode=release` 时若 password 为 `changeme`/空,启动报错。

#### C-BE-6 [CRITICAL] `VectorRepo` 可能为 nil,运行时 panic
- **位置**: `backend/internal/service/cluster_service.go:50-87`
- **问题**: Chroma 连不上时 `Container.VectorRepo = nil`,service 直接 deref。
- **修复**: 显式 nil 守卫 + 健康降级。

#### C-BE-7 [CRITICAL] `H-10: SpanService 未注入 pricingRepo,cost_usd 永远为 0`
- **位置**: `backend/internal/service/container.go:24-37` + `backend/internal/service/span_service.go:96-120`
- **问题**: `NewSpanService` 没接收 pricingRepo,`fillMissingCost` 永远早退,五维归因全 0。
- **修复**: 注入 pricingRepo,真正计算成本。

### 1.2 HIGH 级别

#### H-BE-1 [HIGH] 默认 `from/to` 缺失时静默提供 24h,可拉爆 DB
- **位置**: `backend/internal/api/cost_handler.go:108-127`
- **修复**: 缺失参数返回 400,或 `max_window` 配置限制。

#### H-BE-2 [HIGH] `InternalError` 直接返回 `err.Error()` 泄漏 schema/SQL
- **位置**: `backend/internal/api/response.go:31-37`
- **修复**: 只返回通用消息 + request_id,详细 err 写日志。

#### H-BE-3 [HIGH] `OrderBy`/`Status`/`Type` 无白名单校验
- **位置**: `backend/internal/api/trace_handler.go:108-142` + `backend/internal/repository/span_clickhouse.go:316-335`
- **修复**: 枚举白名单,非法值 400。

#### H-BE-4 [HIGH] ClickHouse SQL 用 fmt.Sprintf 拼接 OrderBy
- **位置**: `backend/internal/repository/span_clickhouse.go:316-335`
- **修复**: SQL builder 只接受白名单 enum。

#### H-BE-5 [HIGH] OTLP 写入 goroutine 无 panic recover、无重试、无错误反馈
- **位置**: `backend/internal/collector/otlp_http.go:92-98`
- **修复**: defer recover + OTLP `PartialSuccess` 反馈。

#### H-BE-6 [HIGH] `Application.context()` 永远返回 Background(),Serve 永不退出
- **位置**: `backend/internal/app/app.go:232-234`
- **修复**: 删除该方法,`Serve` 接收 `ctx context.Context`。

#### H-BE-7 [HIGH] `HealthHandler` 静态返回,不调 HealthCheck
- **位置**: `backend/internal/api/middleware.go:117-132`
- **修复**: 真正调用依赖 Ping,区分 live/ready。

#### H-BE-8 [HIGH] CORS `Allow-Origin: *` + `Allow-Credentials: true` 非法组合
- **位置**: `backend/internal/api/middleware.go:80-94`
- **修复**: 配置 `AllowedOrigins` 白名单,凭证场景动态回显。

#### H-BE-9 [HIGH] `cluster_service.go:64-69` 用 `userID=""` 拉全量,实际只匹配空字符串
- **位置**: `backend/internal/service/cluster_service.go:64-69`
- **修复**: 增加 `ListAllInWindow` 显式接口。

#### H-BE-10 [HIGH] `mode` 默认值 `debug`,生产忘改直接 debug 启动
- **位置**: `backend/internal/config/config.go:204`
- **修复**: 默认 `release`。

### 1.3 MEDIUM(摘录)

| ID | 问题 | 位置 |
|------|------|------|
| M-BE-1 | 无 sentinel error,无法 `errors.Is` | service 多处 |
| M-BE-2 | `gin.SetMode` 重复调用 | app.go:135 + router.go:22 |
| M-BE-3 | `mode="debug"` 默认值生产风险 | config.go:204 |
| M-BE-4 | `parseInt` 非法回退默认值不报错 | response.go:40-45 |
| M-BE-5 | OTLP server 无 Read/Write/Idle Timeout | app.go:145-148 |
| M-BE-6 | `Service.Container` 接口/具体混用 | service/container.go |
| M-BE-7 | `CostService` 类型断言偷取 ClickHouse 客户端 | cost_service.go:37-42 |
| M-BE-8 | `domain.Span.Validate()` 缺失 | domain.go |
| M-BE-9 | `markComplete` 内部 `time.Now()` 不可重放 | domain.go:132-137 |
| M-BE-10 | `LogConfig` 不支持 fatal 级别 | logger.go:21-29 |
| M-BE-11 | `var _ = fmt.Sprintf` 是 dead pattern | otlp_http.go:354 |
| M-BE-12 | 硬编码 `version=0.1.0` 多处 | main.go:47 + middleware.go:129 |
| M-BE-13 | `_ = ctx` 无意义语句 | middleware.go:118-126 |
| M-BE-14 | 提示文案中文硬编码在 Go 代码 | cluster_service.go:259-272 |

### 1.4 Go 1.25 现代性专项

| 特性 | 现状 | 评级 |
|------|------|------|
| Generics | 未使用,部分 util 可简化 | medium |
| `slog` | 未使用,绕道 zerolog | low (个人偏好) |
| `errors.Is/As` | 仅 2 处使用 | high — 见 M-BE-1 |
| `context.Context` | 大部分链路正确,`Background()` leak | medium |
| `maps.Clone` / `slices` | 全部手写 | low |
| Validator v10 | 仅配置 | medium |
| PGX v5 / OTel proto | 最新 | OK |

### 1.5 依赖审计

| 包 | 当前 | 风险 |
|------|------|------|
| `quic-go v0.59.0` | 旧 | 多个 GHSA,需 govulncheck |
| `golang.org/x/crypto` | 0.53.0 | 需 govulncheck |
| `go.opentelemetry.io/proto/otlp` | v1.10.0 | 可升 v1.x 最新 |
| `gin-gonic/gin` | v1.12.0 | OK |
| `sashabaranov/go-openai` | v1.41.2 | OK |

---

## 2. Python SDK Review

### 2.1 CRITICAL 级别

#### C-PY-1 [CRITICAL] `langchain_callback.py` 自引用 import,集成完全损坏
- **位置**: `sdk-python/src/agentpulse/integrations/langchain_callback.py:9`
- **问题**: 文件应包含 `AgentPulseCallback` 实现,实际是 `from agentpulse.integrations.langchain_callback import AgentPulseCallback` 自引用,加载即崩。
- **修复**: 重写为 `BaseCallbackHandler` 真实实现,覆盖 on_chain_start/end、on_llm_start/end、on_tool_start/end。

#### C-PY-2 [CRITICAL] `_build_otlp_endpoint` URL 处理脆弱
- **位置**: `sdk-python/src/agentpulse/client.py:175-184`
- **问题**: 用户传入 `https://collector.example.com/v1/traces` 时,既不含 4318 也不含 4317,被错误再拼 `:4318`。
- **修复**: 用 `urllib.parse.urlparse` 解析,基于 `parsed.port` 决定补端口。

#### C-PY-3 [CRITICAL] `Client.instance()` 未初始化时静默创建默认实例
- **位置**: `sdk-python/src/agentpulse/client.py:81-87`
- **问题**: 用户未 init 就调用,SDK 静默用 localhost 配置上报,生产环境无意识。
- **修复**: 抛 `RuntimeError("AgentPulse not initialized; call init() first")`。

### 2.2 HIGH 级别

| ID | 问题 | 位置 |
|------|------|------|
| H-PY-1 | `requests` 死依赖 | pyproject.toml:27-32 |
| H-PY-2 | `dev` extra 混入 langchain/langgraph;`pyautogen` 已弃用 | pyproject.toml:36-42 |
| H-PY-3 | API Key 通过 `X-AgentPulse-Key` 头传递,debug 日志可泄漏 | client.py:115-118 |
| H-PY-4 | `_safe_serialize` 静默吞 `Exception` + 无截断提示 | decorators.py:38-56 |
| H-PY-5 | `observe()` 默认捕获 args/result 含 PII/密码风险 | decorators.py:69-110 |
| H-PY-6 | `integrations/__init__.py` 直接 import 强制 LangChain 依赖 | integrations/__init__.py:9 |
| H-PY-7 | `version` 硬编码 3 处 | client.py:142 / pyproject.toml / `__version__` |
| H-PY-8 | LangGraph 集成是空壳 `return None` 静默失败 | langgraph.py:16-27 |
| H-PY-9 | AutoGen 集成仅 wrap 同步 send/receive,async 路径未覆盖 | autogen.py:33-63 |
| H-PY-10 | 类型注解缺失,`mypy strict` 不通过 | client.py:74-78 等 |

### 2.3 MEDIUM(摘录)

| ID | 问题 | 位置 |
|------|------|------|
| M-PY-1 | DCL 在无 GIL 环境不安全 | client.py:71-87 |
| M-PY-2 | `SpanWrapper.span_id` 在无效时返回 `"0000000000000000"` 而非 None | spans.py:125-132 |
| M-PY-3 | `Session` 不是 dataclass | spans.py:52-69 |
| M-PY-4 | `trace()` CM 内 `str(value)` 强转,数字/布尔失真 | spans.py:293-294 |
| M-PY-5 | `_execute_traced_sync/async` 重复 90% | decorators.py:134, 167 |
| M-PY-6 | `init()` 缺 `max_queue_size` 等高级参数 | client.py:190-224 |
| M-PY-7 | `init()` info 日志泄漏 endpoint | client.py:146-150 |
| M-PY-8 | `from_env()` 不支持 `OTEL_EXPORTER_OTLP_*` | client.py:215-223 |
| M-PY-9 | `ClientConfig` 缺 `protocol: http/grpc` 切换 | client.py:120-123 |
| M-PY-10 | `api.shutdown()` 收 broad `Exception` | client.py:152-160 |

### 2.4 易拓展性 / 适配器评估

| 维度 | 评估 |
|------|------|
| 适配器入口 | 损坏,需重写 |
| 钩子机制 | `set_attribute`/`set_input`/`set_llm`/`set_tool` 设计良好,缺用户钩子 |
| 插件发现 | 无 entry_points |
| Exporter 抽象 | 硬编码 HTTP,gRPC 需改 config |
| Sampler | `sample_rate` 字段 dead |
| Fork 安全性 | 未处理 |

---

## 3. Web (Next.js 15) Review

### 3.1 CRITICAL 级别

#### C-WEB-1 [CRITICAL] `next.config.js` rewrite 路径拼接 + 默认 localhost
- **位置**: `web/next.config.js:8-9`
- **修复**: 用白名单前缀 + 改用服务端 `BACKEND_API_BASE` + 启动时校验。

#### C-WEB-2 [CRITICAL] 全部 6 个 page 是 `"use client"`,违背 RSC 设计
- **位置**: 所有 `app/**/page.tsx:1`
- **修复**: 页面壳转 RSC,数据获取下沉到 `<DataFetcher />` 客户端组件。

#### C-WEB-3 [CRITICAL] `harness/promote` POST 缺鉴权、CSRF、错误处理
- **位置**: `web/src/app/harness/page.tsx:28-32`
- **修复**: 封装 `lib/api.ts:postJson()`,统一处理。

#### C-WEB-4 [CRITICAL] `agentName`/`traceId` 未 `encodeURIComponent` 拼 URL
- **位置**: `web/src/app/eval/page.tsx:22`, `web/src/app/traces/page.tsx:33`
- **修复**: URL 编码 + 白名单校验。

### 3.2 HIGH 级别

| ID | 问题 | 位置 |
|------|------|------|
| H-WEB-1 | 5 个数据页面无 error/loading 状态展示 | traces/cost/eval/clusters/page.tsx |
| H-WEB-2 | `tsconfig.json` strict 子项不全 (noUncheckedIndexedAccess 等) | tsconfig.json:7 |
| H-WEB-3 | React 19 RC + Next 15.0.3 旧 | package.json |
| H-WEB-4 | 缺 zod/valibot 运行时校验 | 全局 |
| H-WEB-5 | 无 `app/error.tsx` / `loading.tsx` / `not-found.tsx` | 全局 |
| H-WEB-6 | `traces/page.tsx:29` `useSWR<{ trace: any }>` 用 any | traces/page.tsx |
| H-WEB-7 | `page.tsx:22` 变量名 `window` 遮蔽浏览器全局 | page.tsx:22 |
| H-WEB-8 | `traces/page.tsx:29-30` `searchTerm` state 死代码 | traces/page.tsx |
| H-WEB-9 | fetcher 重复定义 6 处,无统一封装 | 各 page.tsx |
| H-WEB-10 | harness `config_hash.substring(0,12)` 无空值保护 | harness/page.tsx:92 |

### 3.3 MEDIUM(摘录)

| ID | 问题 | 位置 |
|------|------|------|
| M-WEB-1 | CSS 工具类与 Tailwind 命名混用,语义混乱 | globals.css + 各组件 |
| M-WEB-2 | `Sidebar.tsx:30` 精确匹配,嵌套路由失效 | Sidebar.tsx |
| M-WEB-3 | `Sidebar.tsx:48` 版本号硬编码 v0.1.0 | Sidebar.tsx |
| M-WEB-4 | `eval/page.tsx:96` `(value as number).toFixed(3)` 无校验 | eval/page.tsx |
| M-WEB-5 | `cost/page.tsx:76` formatter 假定 number | cost/page.tsx |
| M-WEB-6 | `<html lang="en">` 与中文 UI 不一致 | layout.tsx:16 |
| M-WEB-7 | `package.json` 缺 `engines` / `packageManager` | package.json |
| M-WEB-8 | 无测试框架 | 全局 |

---

## 4. Deploy / Docs Review

### 4.1 CRITICAL 级别

#### C-DEPLOY-1 [CRITICAL] docker-compose 默认无密码 + 公网绑定
- **位置**: `deploy/docker-compose.yml:22-80`
- **修复**: 限制 `127.0.0.1` 绑定,设置强制密码,文档明示风险。

#### C-DEPLOY-2 [CRITICAL] OTLP gRPC 端口声明但代码未实现
- **位置**: `docs/API.md:298-306` + `backend/internal/app/app.go:144-148`
- **修复**: 删除 gRPC 相关配置,统一声明"OTLP 仅 HTTP"。

#### C-DEPLOY-3 [CRITICAL] ARCHITECTURE 声称 AuthMiddleware 占位,实际未挂载
- **位置**: `docs/ARCHITECTURE.md:283-284`
- **修复**: 与 C-BE-1 一并修,文档同步"已实现"。

### 4.2 HIGH 级别

| ID | 问题 | 位置 |
|------|------|------|
| H-DEPLOY-1 | K8s manifests 缺失 | deploy/k8s/ |
| H-DEPLOY-2 | Dockerfile 缺失 | deploy/Dockerfile |
| H-DEPLOY-3 | docker-compose 缺 backend 服务 | docker-compose.yml |
| H-DEPLOY-4 | API.md 路由路径与代码部分不一致 | API.md (实际 router.go 注释错) |
| H-DEPLOY-5 | PRD.md 列出 15+ 不存在的 API 端点 | PRD.md:610-664 |
| H-DEPLOY-6 | SDK.md Go SDK 章节展示不存在的 API | SDK.md:82-101 |
| H-DEPLOY-7 | `.env.example` 缺 12+ 配置项 | .env.example |
| H-DEPLOY-8 | CORS 允许 `*` + credentials 非法组合 | backend/api/middleware.go:82-83 |
| H-DEPLOY-9 | ClickHouse 启动 healthcheck 不验证表存在 | docker-compose.yml:37-42 |
| H-DEPLOY-10 | Chroma 容器无 healthcheck | docker-compose.yml:70-80 |

### 4.3 MEDIUM(摘录)

| ID | 问题 | 位置 |
|------|------|------|
| M-DEPLOY-1 | ClickHouse init SQL 注释符混用 | deploy/init/clickhouse/01-init.sql |
| M-DEPLOY-2 | `attributes String` 应为 `JSON` 类型 | deploy/init/clickhouse/01-init.sql:59 |
| M-DEPLOY-3 | Postgres `evaluations` 缺 `overall` 列 | deploy/init/postgres/01-init.sql |
| M-DEPLOY-4 | Harness Promote 不降级旧 production | harness_handler.go:106-124 |
| M-DEPLOY-5 | arxiv id 2607.xxxxx 引用不存在的论文 | ARCHITECTURE.md:357-364 |
| M-DEPLOY-6 | panic 错误码 `internal_server_error` 与文档 `internal_error` 不一致 | recovery middleware |

---

## 5. 修复优先级矩阵

| 优先级 | 类别 | 数量 | 目标 |
|--------|------|------|------|
| **P0** (本次) | Critical 全部 | 18 | 安全基线 + 集成可工作 |
| **P1** (本次) | High 全部 | 30+ | 生产质量关键 |
| **P2** (下次迭代) | Medium | 60+ | 规范 + 维护性 |
| **P3** (持续) | Low | 30+ | 风格 + 优化 |

---

## 6. 修复进度追踪

### Commit 1: docs(review) — 添加本 REVIEW 报告 ✅
### Commit 2: docs(api) — 文档与代码对齐 (待)
### Commit 3: fix(deploy) — 部署安全基线 (待)
### Commit 4: feat(backend) — 真实 API Key 鉴权 (待)
### Commit 5: fix(backend) — OTLP 认证 + body limit + panic recover (待)
### Commit 6: fix(backend) — 错误信息脱敏 + SQL 注入防护 (待)
### Commit 7: fix(backend) — HealthCheck 真实探活 + SpanService 注入 pricingRepo (待)
### Commit 8: refactor(backend) — Medium 规范修复 (待)
### Commit 9: fix(sdk-python) — 重写 LangChain/LangGraph/AutoGen 集成 (待)
### Commit 10: fix(sdk-python) — endpoint 解析 + 敏感数据脱敏 + 类型注解 (待)
### Commit 11: refactor(sdk-python) — Medium 修复 (待)
### Commit 12: fix(web) — rewrite 安全 + RSC 拆分 (待)
### Commit 13: fix(web) — Harness POST + URL encode + zod 校验 (待)
### Commit 14: fix(web) — error/loading 状态 + 依赖升级 (待)
### Commit 15: feat(deploy) — K8s manifests + Dockerfile (待)
### Commit 16: docs — 最终文档一致性检查 (待)

---

## 7. 测试建议

| 层 | 工具 | 优先级 |
|------|------|--------|
| Go | go test + testify + mock | P1 |
| Go | govulncheck + gosec | P1 |
| Python | pytest + pytest-mock + pytest-asyncio | P1 |
| Python | mypy strict + ruff | P1 |
| Web | vitest + @testing-library/react | P2 |
| Web | playwright E2E | P2 |
| CI | GitHub Actions matrix (3.10/3.11/3.12/3.13) | P1 |

---

## 8. 文档对齐待办

| 文档 | 待修改项 |
|------|---------|
| README.md (root) | 与代码最终实现对齐 |
| docs/PRD.md | 标注已实现 / Phase 2 章节,删除未实现端点 |
| docs/ARCHITECTURE.md | 鉴权已实现、删除 gRPC、SDK-Go 标 Phase 3 |
| docs/API.md | 路径与方法校验,补 Error code 统一表 |
| docs/SDK.md | Go SDK 章节明确"未实现",补 LangChain/LangGraph 真实示例 |

---

**审查完成**。所有结论基于实际源码逐行分析,未访问外部资源(除 OTel/Next.js 官方文档核对 API)。
