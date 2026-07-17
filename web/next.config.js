/** @type {import('next').NextConfig} */

// API 代理已迁移至 app/api/backend/[...path]/route.ts（BFF 注入 X-AgentPulse-Key）。
// 此处仅做生产环境 BACKEND_API_BASE 启动期校验。

const nextConfig = {
  reactStrictMode: true,
  // 构建时校验：生产必须配置后端地址（密钥在运行时读取，不强制 build 时存在）
  webpack: undefined,
};

// next.config 在 build 时加载；生产构建强制 BACKEND_API_BASE
if (process.env.NODE_ENV === "production" && !process.env.BACKEND_API_BASE) {
  // 允许 type-check / 部分 CI 跳过：仅 next build 时 NODE_ENV=production
  if (process.env.npm_lifecycle_event === "build") {
    throw new Error(
      "生产构建必须设置 BACKEND_API_BASE 环境变量（服务端专用，勿使用 NEXT_PUBLIC_）"
    );
  }
}

module.exports = nextConfig;
