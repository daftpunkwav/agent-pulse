/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // AgentPulse 后端 API 代理（开发环境）
  async rewrites() {
    return [
      {
        source: "/api/backend/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"}/api/v1/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;