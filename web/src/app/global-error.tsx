"use client";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  // global-error.tsx 在 app shell 之外渲染，必须自带 <html> 与 <head>。
  return (
    <html lang="zh-CN">
      <head>
        <title>系统错误 · AgentPulse</title>
      </head>
      <body>
        <div style={{ padding: "2rem", textAlign: "center" }}>
          <h2>系统错误</h2>
          <p style={{ color: "#dc2626" }}>应用出现了意外错误，请刷新或返回首页重试。</p>
          {error.digest && (
            <p style={{ color: "#64748b", fontSize: "0.75rem" }}>
              错误 ID: {error.digest}
            </p>
          )}
          <button type="button" onClick={reset} style={{ marginTop: "1rem" }}>
            重试
          </button>
        </div>
      </body>
    </html>
  );
}
