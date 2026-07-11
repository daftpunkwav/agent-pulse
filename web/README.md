# AgentPulse Web Dashboard

Next.js 15 + React 19 + TypeScript 前端。

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 访问 http://localhost:3000
```

## 构建

```bash
npm run build
npm start
```

## 环境变量

复制 `.env.example` 到 `.env.local`：

```bash
cp .env.example .env.local
```

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `BACKEND_API_BASE` | `http://localhost:8080` | AgentPulse 后端地址（服务端变量，不暴露给浏览器） |

## 目录结构

```
src/
├── app/
│   ├── layout.tsx        # 全局布局 + Sidebar
│   ├── globals.css       # 全局样式
│   ├── page.tsx          # 首页（Overview）
│   ├── traces/page.tsx   # Trace Viewer
│   ├── cost/page.tsx     # 成本 Dashboard
│   ├── eval/page.tsx     # 评估 Dashboard
│   ├── clusters/page.tsx # 失败聚类
│   └── harness/page.tsx  # Harness 管理
└── components/
    └── Sidebar.tsx       # 侧边导航
```

## 技术栈

- **Next.js 15** (App Router)
- **React 19**
- **TypeScript 5.7**
- **SWR 2.2** (数据获取)
- **Recharts 2.13** (可视化)
- **Lucide React** (图标)
- **Tailwind CSS v4** (样式)
- **Zod** (运行时校验)
- **next-themes** (主题切换)

## API 代理

开发环境通过 Next.js rewrites 代理 `/api/backend/*` 到 AgentPulse 后端 `/api/v1/*`：

```js
// next.config.js
rewrites: [
  {
    source: "/api/backend/:path*",
    destination: "http://localhost:8080/api/v1/:path*",
  },
]
```

这样前端只需调用 `/api/backend/cost/total`，避免 CORS 问题。

## 当前 MVP 范围

- ✅ 总览页（4 个指标卡片 + 最近失败聚类）
- ✅ Trace Viewer（按 ID 查询 + Span 列表）
- ✅ Cost Dashboard（时间序列折线图 + 五维归因表）
- ✅ Eval Dashboard（雷达图 + 维度分数）
- ✅ Clusters 列表
- ✅ Harness 版本管理

后续可扩展：
- Trace 火焰图
- 成本桑基图
- A/B 测试可视化
- EvalLoop 迭代工作流编辑器
- 实时刷新（SSE）