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
    <div className="state-box state-box--error" role="alert">
      <p className="state-label state-label--error">{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="btn btn-secondary mt-4">
          重试
        </button>
      )}
    </div>
  );
}
