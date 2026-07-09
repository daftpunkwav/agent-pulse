"use client";

import { useEffect } from "react";
import { ErrorState } from "@/components/ErrorState";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // 仅在开发环境打印详细错误，避免生产环境向终端/远程泄露内部信息
    if (process.env.NODE_ENV !== "production") {
      // eslint-disable-next-line no-console
      console.error(error);
    }
  }, [error]);

  return (
    <ErrorState
      message="页面加载失败，请稍后重试。"
      onRetry={reset}
    />
  );
}
