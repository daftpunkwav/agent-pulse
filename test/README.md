# AgentPulse 测试套件

> 所有分层测试入口位于本目录 `test/`。  
> Go 包内测试在 `backend/test/`（可访问 `internal/*`），由本目录脚本统一调度。

## 分层

| 层级 | 目录 | 说明 |
|------|------|------|
| 单元 | `test/unit/`、`backend/test/unit/` | 函数级：PII、config、proxy 路径 |
| 模块 | `test/module/`、`backend/test/module/` | 中间件、Container、SDK 集成 |
| 功能 | `test/functional/`、`backend/test/functional/` | OTLP PartialSuccess 等用户可见行为 |
| 集成 | `test/integration/`、`backend/test/integration/` | 鉴权路由、K8s manifest、跨组件 |

## 运行

```powershell
# 仓库根目录
pwsh -File test/run_all.ps1
```

或分项：

```powershell
pwsh -File test/run_backend.ps1
pwsh -File test/run_sdk.ps1
pwsh -File test/run_web.ps1
pwsh -File test/run_deploy.ps1
```
