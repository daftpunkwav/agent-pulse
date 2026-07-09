"use client";

interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
}

/** 通用错误展示 */
export function ErrorState({
  message = "加载失败，请稍后重试",
  onRetry,
}: ErrorStateProps) {
  return (
    <div
      className="card"
      style={{ borderColor: "#fecaca", background: "#fef2f2" }}
      role="alert"
    >
      <p className="text-sm" style={{ color: "#dc2626" }}>
        {message}
      </p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="btn btn-secondary mt-4"
        >
          重试
        </button>
      )}
    </div>
  );
}
