/** @type {import('next').NextConfig} */

/** 允许代理到后端的 API 路径前缀白名单 */
const ALLOWED_API_PREFIXES = [
  "traces",
  "cost",
  "eval",
  "clusters",
  "harness",
];

function getBackendBase() {
  const base = process.env.BACKEND_API_BASE || "http://localhost:8080";
  if (process.env.NODE_ENV === "production" && !process.env.BACKEND_API_BASE) {
    throw new Error(
      "生产构建必须设置 BACKEND_API_BASE 环境变量（服务端专用，勿使用 NEXT_PUBLIC_）"
    );
  }
  return base.replace(/\/$/, "");
}

const nextConfig = {
  reactStrictMode: true,
  output: "standalone",
  async rewrites() {
    const backend = getBackendBase();
    const rules = ALLOWED_API_PREFIXES.flatMap((prefix) => [
      {
        source: `/api/backend/${prefix}`,
        destination: `${backend}/api/v1/${prefix}`,
      },
      {
        source: `/api/backend/${prefix}/:path*`,
        destination: `${backend}/api/v1/${prefix}/:path*`,
      },
    ]);
    return rules;
  },
};

module.exports = nextConfig;
