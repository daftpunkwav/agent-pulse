# AgentPulse Documentation

## 目录

| 文档 | 内容 |
|------|------|
| [PRD.md](PRD.md) | 产品需求文档（业务定义、功能清单、MVP 范围） |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 系统架构（分层、数据流、设计决策、扩展点） |
| [API.md](API.md) | REST API 参考（所有端点、参数、响应格式） |
| [SDK.md](SDK.md) | SDK 使用指南（Python/Go、集成示例、性能调优） |
| [SECURITY.md](SECURITY.md) | 安全指南（威胁模型、部署清单、Key 管理） |
| [REVIEW.md](REVIEW.md) | 全面代码 review 报告 |
| [REPORT.md](REPORT.md) | Review 修复完成报告 |
| [PLAN.md](PLAN.md) | 修复计划与进度追踪 |

## 子项目文档

| 子项目 | 文档 |
|--------|------|
| Backend (Go) | [`/backend`](../backend) — API 入口见 `internal/api/router.go` |
| Web (Next.js) | [`/web/README.md`](../web/README.md) |
| Python SDK | [`/sdk-python/README.md`](../sdk-python/README.md) |
| Go SDK | [`/sdk-go/`](../sdk-go) — Phase 3 占位,当前请用 OTLP 官方 SDK |
| 部署 | [`/deploy/README.md`](../deploy/README.md) |

## 快速链接

- **快速开始**：[主 README](../README.md)
- **架构总览**：[ARCHITECTURE.md](ARCHITECTURE.md)
- **API 速查**：[API.md](API.md)
- **SDK 速查**：[SDK.md](SDK.md)

## 版本

| 版本 | 日期 | 状态 |
|------|------|------|
| 0.1.0 | 2026-07-09 | MVP 初版 |