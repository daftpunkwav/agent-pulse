"use client";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="zh-CN">
      <body>
        <div style={{ padding: "2rem", textAlign: "center" }}>
          <h2>系统错误</h2>
          <p style={{ color: "#dc2626" }}>{error.message}</p>
          <button type="button" onClick={reset} style={{ marginTop: "1rem" }}>
            重试
          </button>
        </div>
      </body>
    </html>
  );
}
