# AgentPulse 修复计划与进度

> 制定日期: 2026-07-09
> 审查报告: [docs/REVIEW.md](REVIEW.md)
> 修复策略: Critical + High 全部修复,Medium/Low 记录在案

---

## 0. 整体节奏

- **每 2-3 个修复 = 1 个 commit**
- **每个 commit 完成后立即推送 3 远端**(gitee / gitlab / origin)
- **每批结束后写 1 段进度到本文件**
- **不在中途向用户汇报**(根据用户指示)
- **最后生成 [REPORT.md](REPORT.md) 汇总报告**

---

## 1. 修复进度

| # | Commit | 范围 | 状态 |
|---|--------|------|------|
| 1 | `docs(review): add comprehensive code review report` | REVIEW.md | ✅ 已推送 |
| 2 | `docs: align PRD/ARCHITECTURE/API/SDK with actual code` | docs/ | ✅ 已推送 |
| 3 | `fix(backend): critical security hardening (auth/OTLP/DSN/eval)` | config/collector/service | ✅ 已推送 |
| 4 | `fix(backend): api layer hardening (PII/HealthCheck/lifecycle)` | api/app | ✅ 已推送 |
| 5 | `fix(backend): SQL allow-lists, strict window, sentinel errors, cluster fix` | domain/repository | ✅ 已推送 |
| 6 | `fix(sdk-python): rewrite LangChain/LangGraph/AutoGen integrations` | sdk-python/ | ⏳ 进行中 |
| 7 | `fix(sdk-python): endpoint URL builder, instance() guard, sensitive-data redaction` | sdk-python/client/decorators | ⏳ 待开始 |
| 8 | `fix(sdk-python): cleanup dead deps, modernize types, async coverage` | pyproject/tests | ⏳ 待开始 |
| 9 | `fix(web): next.config rewrite safety, RSC boundary, error handling` | web/ | ⏳ 待开始 |
| 10 | `fix(web): URL encoding, Harness POST, deps upgrade, zod validation` | web/ | ⏳ 待开始 |
| 11 | `fix(deploy): defaults, healthchecks, K8s manifests, Dockerfile` | deploy/ | ⏳ 待开始 |
| 12 | `docs: final consistency check, K8s manifest doc, security runbook` | docs/ | ⏳ 待开始 |
| 13 | `docs(report): final summary of all changes` | REPORT.md | ⏳ 待开始 |

---

## 2. 详细修复项清单

### 2.1 后端 (Go) - Critical + High

#### 鉴权 (C-BE-1, H-BE-8) — ✅ Commit 3
- [x] `AuthConfig` 加 `Enabled` / `APIKeys` / `OTLPRequireKey` 字段
- [x] `AuthMiddleware` 实现真正的 SHA-256 + 常量时间比对
- [x] `v1` group 挂载 `AuthMiddleware`
- [x] 配置文件 + 环境变量双通道 (`AGENTPULSE_AUTH_API_KEYS`)

#### OTLP 加固 (C-BE-2, H-BE-5, H-BE-6) — ✅ Commit 3
- [x] OTLP 加 `ValidateAPIKey` (与 API 共享白名单)
- [x] `http.MaxBytesReader` 限制 (默认 10MB,`otlp.max_body_size`)
- [x] 同步 + 异步 panic recover
- [x] http.Server ReadHeader/Read/Write/Idle Timeout
- [x] `OTLPRequireKey` 配置

#### 评估接口 PII (C-BE-3) — ✅ Commit 4
- [x] 新增 `pii.go`: 7 类 PII 正则 (email/phone/credit-card/ID/JWT/bearer/api_key)
- [x] `EvaluateNow` 接入 `RedactPII`
- [x] 客户端仍可见原始 input (response); 仅 LLM 调用时脱敏

#### DSN 脱敏 (C-BE-4) — ✅ Commit 3
- [x] `ClickHouseConfig.MaskedDSN()` / `PostgresConfig.MaskedDSN()` 返回 `****`
- [x] `validate()` 拒绝 release 模式下的默认密码 / 空密码 / 短 key

#### Config 强校验 (H-BE-7, M-BE-3) — ✅ Commit 3
- [x] `server.mode` 默认值改 `release`
- [x] release 模式下强制非默认 password + API key 长度 >= 16

#### API Key 错误信息脱敏 (H-BE-2) — ✅ Commit 4
- [x] `InternalError` 不再回 `err.Error()`
- [x] 新增 `InternalErrorLog(c, log, err)`: 写日志,客户端只见通用消息 + request_id
- [x] 所有 handler 切到新 API

#### SQL 注入防护 (H-BE-3, H-BE-4) — ✅ Commit 5
- [x] `domain.ValidOrderBy` + `IsValidOrderBy`(实际实现为 `ValidOrderBy` + map 查询)
- [x] `domain.ValidSpanTypes` / `ValidSpanStatuses` / `ValidCostDimensions`
- [x] repository 层 `orderByColumnMap` 双保险
- [x] trace_handler parseListOptions 校验所有枚举

#### 默认 window 严格化 (H-BE-1, M-BE-12) — ✅ Commit 5
- [x] from/to 必传,缺失返回 400
- [x] from > to 返回 400
- [x] window 长度上限 90 天
- [x] parseDimensions 白名单校验

#### HealthCheck 真实探活 (H-BE-9) — ✅ Commit 4
- [x] `HealthPinger` interface in service.Container
- [x] `Application` 注入自身到 `Container.HealthPinger`
- [x] `HealthHandler` 区分 `/healthz`(liveness) / `/readyz`(真探活)

#### SpanService 注入 pricingRepo (H-BE-10) — ✅ Commit 3
- [x] `NewSpanService(repo, pricingRepo, log)` 三参构造
- [x] `NewContainer` 正确传入 `repos.Pricing`
- [x] `fillMissingCost` 真正能算 LLM 成本

#### VectorRepo nil 守卫 (C-BE-5) — ✅ Commit 3
- [x] `initInfrastructure` 失败时回滚 (调用 `Shutdown` 清理)
- [x] `Container.VectorRepo` 允许为 nil,service 层显式 nil check

#### Application 启动回滚 (H-BE-8) — ✅ Commit 4
- [x] `New()` 任何步骤失败 → `Shutdown(context.Background())`
- [x] `Application.Serve(ctx)` 接收 ctx,不再依赖 `context()` dead method
- [x] main.go 用 `serveCtx` 协调信号 + Serve

#### CORS 非法组合 (H-BE-8) — ✅ Commit 4
- [x] CORS 用白名单 `cfg.Server.AllowedOrigins`
- [x] 移除 `Allow-Origin: *` + `Allow-Credentials: true` 同时设置
- [x] 动态回显 origin + Vary: Origin

### 2.2 后端 (Go) - Medium (记录在案,本轮优先)

- [ ] M-BE-1: 全面使用 sentinel error (Commit 5 已定义,各 service 替换)
- [x] M-BE-2: gin.SetMode 去重 (Commit 4)
- [x] M-BE-3: mode 默认 release (Commit 3)
- [x] M-BE-5: OTLP server timeout (Commit 3)
- [ ] M-BE-6: service.Container 全部接口化
- [ ] M-BE-7: CostService 抽象 AnalyticsBackend,移除类型断言
- [ ] M-BE-8: domain.Span.Validate() 在 ingest 入口校验
- [ ] M-BE-10: LogConfig 不支持 fatal (api 简单修复)
- [x] M-BE-12: window 上限 (Commit 5)
- [ ] M-BE-14: 业务文案移到 i18n 表
- [ ] M-BE-15: PII 脱敏在 OTLP 入口也启用
- [x] M-BE-25: 日志 Fatal → 返回 error (main.go 改 run 模式)
- [x] M-BE-31: mode 默认 release (Commit 3)
- [x] M-BE-32: 删除 `var _ = fmt.Sprintf` dead pattern (Commit 3)

### 2.3 Python SDK - Critical + High — ⏳ Commit 6-8

#### 集成层 (C-PY-1) — ⏳ Commit 6
- [ ] 重写 `langchain_callback.py` 为真 `BaseCallbackHandler`
  - `on_chain_start/end`, `on_llm_start/end`, `on_tool_start/end`
  - `on_agent_action`, `on_agent_finish`
  - 软依赖探测,缺 `langchain-core` 时不崩
- [ ] 重写 `langgraph.py` 真集成(LangGraph 用 LangChain Callback API)
- [ ] 重写 `autogen.py` 同时 wrap `a_send` / `a_receive` async 路径

#### URL 构建 (C-PY-2) — ⏳ Commit 7
- [ ] `urllib.parse.urlparse` 重写 `_build_otlp_endpoint`
- [ ] 拒绝路径已含 `/v1/traces` 的输入
- [ ] IPv6 支持
- [ ] 单元测试覆盖边界

#### Client.instance() 抛错 (C-PY-3) — ⏳ Commit 7
- [ ] 未 init 时抛 `RuntimeError("AgentPulse client not initialized")`
- [ ] 移除静默 default-instance 行为

#### 依赖清理 (H-PY-1, H-PY-2) — ⏳ Commit 8
- [ ] 删除 `requests` 死依赖
- [ ] `dev` extra 移除 langchain/langgraph(已在各自 extra 中)
- [ ] `pyautogen` 升级到 `autogen-agentchat` 或锁死版本
- [ ] 升级 `requires-python = ">=3.11"` 用 3.11+ 特性

#### 敏感数据脱敏 (H-PY-5, H-PY-7) — ⏳ Commit 7
- [ ] `_safe_serialize` 收窄异常 + 截断加 `...[truncated]` 标记
- [ ] `observe()` 默认 `capture_args=False, capture_result=False`
- [ ] `redact_keys` 参数 (默认覆盖 password/token/secret/api_key)
- [ ] API Key 格式校验(开头 `ap-`,长度 >= 16)

#### 类型注解 (H-PY-4, H-PY-10) — ⏳ Commit 8
- [ ] `Client._tracer: Optional[Tracer] = None`
- [ ] `get_tracer() -> Tracer` 加 assert
- [ ] `init()` 补 `**kwargs` 透传
- [ ] `from_env()` 支持 `OTEL_EXPORTER_OTLP_*`

#### 测试扩充 — ⏳ Commit 8
- [ ] mock OTLPSpanExporter
- [ ] async 装饰器测试
- [ ] endpoint URL 构建边界测试
- [ ] redact 测试

### 2.4 Web (Next.js) - Critical + High — ⏳ Commit 9-10

#### Rewrite 安全 (C-WEB-1) — ⏳ Commit 9
- [ ] `next.config.js` 改用白名单前缀
- [ ] 移除 `NEXT_PUBLIC_API_BASE`(改 `BACKEND_API_BASE` 服务端变量)
- [ ] build 阶段校验环境变量

#### RSC 重构 (C-WEB-2) — ⏳ Commit 9
- [ ] 页面壳转 Server Component
- [ ] 数据获取下沉到 `<DataFetcher />` 客户端组件
- [ ] 移除 `page.tsx:22` 变量名 `window` 遮蔽

#### Harness POST 安全 (C-WEB-3) — ⏳ Commit 10
- [ ] 封装 `lib/api.ts:postJson()`,统一处理
- [ ] Harness promote 加 loading 态 / disabled 防重复
- [ ] 错误反馈给用户

#### URL 编码 (C-WEB-4) — ⏳ Commit 10
- [ ] 所有用户输入 → `encodeURIComponent`
- [ ] agentName / traceId 加白名单正则 `/^[a-z0-9-]{1,64}$/`

#### 错误处理 + zod (H-WEB-1, H-WEB-4) — ⏳ Commit 10
- [ ] `<ErrorState onRetry />` / `<LoadingState />` / `<EmptyState />` 通用组件
- [ ] zod schema 定义 + `fetcher` 内 `schema.parse(json)`
- [ ] `app/error.tsx` / `app/global-error.tsx` / `app/loading.tsx` / `app/not-found.tsx`

#### tsconfig + 依赖 (H-WEB-2, H-WEB-5) — ⏳ Commit 10
- [ ] 启用 `noUncheckedIndexedAccess` / `exactOptionalPropertyTypes` / `noImplicitOverride`
- [ ] React 19.0.0 GA + Next 15.1+
- [ ] `eslint-plugin-react-hooks` / `eslint-plugin-jsx-a11y` / Prettier

### 2.5 Deploy — ⏳ Commit 11

#### 安全基线 (C-DEPLOY-1, C-DEPLOY-3)
- [ ] docker-compose 限 `127.0.0.1` 绑定
- [ ] 强制密码 (CH + PG + Chroma)
- [ ] healthcheck 验证表存在

#### OTLP gRPC 文档清理 (C-DEPLOY-2)
- [ ] 删除 `otlp.grpc_port` 配置项 (Phase 2 再加)
- [ ] 文档说明当前仅 HTTP

#### K8s manifests (H-DEPLOY-1) — Commit 11
- [ ] `deploy/k8s/base/backend.yaml` (Deployment + Service + ConfigMap + Secret)
- [ ] `deploy/k8s/base/postgres.yaml` (StatefulSet + Service + PVC)
- [ ] `deploy/k8s/base/clickhouse.yaml`
- [ ] `deploy/k8s/base/chroma.yaml`
- [ ] `deploy/k8s/base/web.yaml`
- [ ] `deploy/k8s/overlays/production/kustomization.yaml`
- [ ] `deploy/k8s/base/ingress.yaml`
- [ ] `deploy/k8s/base/networkpolicy.yaml`
- [ ] 所有 pod: resources / liveness / readiness / securityContext (non-root, readOnly)

#### Dockerfile (H-DEPLOY-2) — Commit 11
- [ ] `deploy/docker/backend.Dockerfile` (Go multi-stage)
- [ ] `deploy/docker/web.Dockerfile` (Next.js standalone)
- [ ] `.dockerignore`

#### init SQL 改进
- [ ] `attributes JSON DEFAULT '{}'`
- [ ] `evaluations` 加 `overall` 列
- [ ] `idx_evaluations_span_id` / `idx_evaluations_agent_name`

### 2.6 文档最终对齐 — ⏳ Commit 12

- [ ] 复核所有 `docs/*.md` 与最终代码一致
- [ ] 删 SDK.md 中 Go SDK "API 示例"(改 "未实现"占位)
- [ ] 补 docs/SECURITY.md(威胁模型 + 部署清单)
- [ ] 补 docs/RELEASE.md(版本号 / 升级说明)
- [ ] update root README.md 反映最终实现

---

## 3. 已知风险与权衡

| 决策 | 原因 | 风险 |
|------|------|------|
| Auth 用 config 文件白名单(不接 DB) | 用户要求"最小可用方案" | Key 轮换需重启服务;Phase 2 升级到 DB |
| Release 模式拒绝默认密码 | 防止 changeme 上生产 | dev 模式仍可启动;但生产必须配 |
| 不在 SDK 端默认捕获 args/result | 防止 PII 泄露到 OTLP | 用户需手动开启,但更安全 |
| K8s 走 base + overlay 而非单文件 | 符合 Kustomize 习惯 | 引入新工具学习成本 |
| Chroma 不可用时仅 Warn 不退出 | 用户偏好"软依赖" | 失败聚类功能不可用,需监控告警 |

---

## 4. 后续可拓展方向 (P2+)

- mTLS for OTLP
- Token bucket rate limit
- Prometheus / OpenTelemetry self-instrumentation
- API Key CRUD (DB) + JWT
- EvalLoop 迭代工作流 UI
- Trace 火焰图前端
- 多租户 + SaaS 计费
- Trace 回放(Replay)

---

## 5. 沟通约定

- 用户指示: 每 2-3 个修复 1 个 commit;全程不汇报;最后写 REPORT.md
- 当前状态: 正在按 5+5 节奏推进;下一批目标 = Python SDK
