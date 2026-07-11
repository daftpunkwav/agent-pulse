# AgentPulse 安全指南

> 版本: v0.1.x | 更新: 2026-07-09

## 1. 威胁模型

| 威胁 | 影响 | 缓解措施 |
|------|------|----------|
| 未授权 API 访问 | 数据泄露/篡改 | API Key 鉴权 (`AuthMiddleware`) |
| 未授权 OTLP 注入 | 存储撑爆/垃圾数据 | OTLP Key 校验 + Body 大小限制 |
| PII 泄露到 Judge | 合规风险 | `RedactPII` 脱敏后再送 LLM |
| 默认密码上生产 | 数据库被攻破 | release 模式启动校验 |
| CORS 错配 | CSRF/凭证泄漏 | `AllowedOrigins` 白名单 |
| SQL 注入 | 数据泄露 | OrderBy/枚举白名单 |
| SDK 默认捕获参数 | PII 进入 Trace | `capture_args=False` 默认 |

## 2. 部署安全清单

### 2.1 后端 (Go)

- [ ] `AGENTPULSE_SERVER_MODE=release`
- [ ] `AGENTPULSE_AUTH_ENABLED=true`
- [ ] `AGENTPULSE_AUTH_API_KEYS` 设置 >=16 字符的 key，前缀建议 `ap-`
- [ ] `AGENTPULSE_POSTGRES_PASSWORD` 非空且非 `changeme`
- [ ] `AGENTPULSE_CLICKHOUSE_PASSWORD` 非默认
- [ ] `AGENTPULSE_SERVER_ALLOWED_ORIGINS` 仅包含可信前端域名
- [ ] `AGENTPULSE_OTLP_MAX_BODY_SIZE` 保持默认 10MB 或更低
- [ ] 日志级别 `info`，避免 debug 泄漏 DSN

### 2.2 Python SDK

- [ ] 应用启动时调用 `init(api_key="ap-...")`
- [ ] 仅在必要时开启 `capture_args=True`
- [ ] 使用 `redact_keys` 覆盖自定义敏感字段

### 2.3 Web (Next.js)

- [ ] 生产构建设置 `BACKEND_API_BASE`（服务端变量，非 `NEXT_PUBLIC_`）
- [ ] 不将 API Key 放入前端代码
- [ ] 通过 Next.js rewrite 代理，不直连后端公网地址

### 2.4 Docker Compose（开发）

- [ ] 复制 `deploy/.env.example` 为 `deploy/.env`
- [ ] 所有端口已绑定 `127.0.0.1`
- [ ] 不使用默认 `changeme` 密码

### 2.5 Kubernetes（生产）

- [ ] 替换 `agentpulse-secrets` 中所有占位密码
- [ ] 启用 `NetworkPolicy`
- [ ] Pod `runAsNonRoot` + `readOnlyRootFilesystem`（backend 已配置）
- [ ] Ingress TLS 终止

## 3. API Key 管理

当前版本 (v0.1.x) API Key 通过配置文件/环境变量白名单管理：

```bash
export AGENTPULSE_AUTH_API_KEYS="ap-prod-key-xxxxxxxx,ap-backup-yyyyyyyy"
```

**限制**: Key 轮换需重启服务。Phase 2 计划支持 DB 管理 + JWT。

传输方式：
- REST API: `X-AgentPulse-Key` Header
- OTLP HTTP: 同 Header（`AGENTPULSE_AUTH_OTLP_REQUIRE_KEY=true` 时）
- OTLP gRPC: `x-agentpulse-key` metadata（同样受 `AGENTPULSE_AUTH_OTLP_REQUIRE_KEY` 控制）

## 4. 敏感数据处理

### 4.1 后端 PII 脱敏

`EvaluateNow` 在送 Judge 前调用 `RedactPII`，覆盖：
- email / phone / credit-card / ID / JWT / bearer token / api_key

### 4.2 SDK 脱敏

`observe()` 装饰器默认不捕获参数/返回值。开启后自动 redact `password`、`token`、`secret` 等键名。

## 5. 漏洞报告

如发现安全问题，请通过 GitHub/GitLab Private Security Advisory 私下报告，勿公开 Issue。
