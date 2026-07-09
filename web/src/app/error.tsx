"use client";

import { ErrorState } from "@/components/ErrorState";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <ErrorState
      message={error.message || "页面发生错误"}
      onRetry={reset}
    />
  );
}
