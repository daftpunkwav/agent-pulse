# AgentPulse 修复计划与进度

> 制定日期: 2026-07-09
> 审查报告: [docs/REVIEW.md](REVIEW.md)
> 完成报告: [docs/REPORT.md](REPORT.md)
> 修复策略: Critical + High 全部修复,Medium/Low 记录在案

---

## 1. 修复进度

| # | Commit | 范围 | 状态 |
|---|--------|------|------|
| 1 | `docs(review): add comprehensive code review report` | REVIEW.md | ✅ |
| 2 | `docs: align PRD/ARCHITECTURE/API/SDK with actual code` | docs/ | ✅ |
| 3 | `fix(backend): critical security hardening` | config/collector/service | ✅ |
| 4 | `fix(backend): api layer hardening` | api/app | ✅ |
| 5 | `fix(backend): SQL allow-lists, strict window` | domain/repository | ✅ |
| 6 | `fix(sdk-python): langchain_callback BaseCallbackHandler` | sdk-python/ | ✅ |
| 7 | `fix(sdk-python): langgraph + autogen async` | sdk-python/ | ✅ |
| 8 | `fix(sdk-python): client URL/instance guard` | sdk-python/client | ✅ |
| 9 | `fix(sdk-python): redaction + deps + tests` | sdk-python/ | ✅ |
| 10 | `fix(web): next.config + api lib` | web/ | ✅ |
| 11 | `fix(web): RSC + zod + error handling` | web/ | ✅ |
| 12 | `fix(deploy): compose + k8s + dockerfile` | deploy/ | ✅ |
| 13 | `docs: SECURITY.md + REPORT.md` | docs/ | ✅ |

---

## 2. 遗留 Medium 项 (P2)

详见 [REVIEW.md](REVIEW.md) §1.3 / §2.3 / §3.3，下一迭代处理。

---

## 3. 后续可拓展方向 (P2+)

- mTLS for OTLP
- Token bucket rate limit
- API Key CRUD (DB) + JWT
- Trace 火焰图前端
- Go SDK (Phase 3)
