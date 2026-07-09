/** 通用加载态 */
export function LoadingState({ label = "加载中..." }: { label?: string }) {
  return (
    <div className="state-box" role="status" aria-live="polite">
      <div className="mx-auto mb-3 flex w-fit gap-1.5">
        {[0, 1, 2].map((i) => (
          <span
            key={i}
            className="h-2 w-2 animate-pulse rounded-full bg-cyan-400"
            style={{ animationDelay: `${i * 150}ms` }}
          />
        ))}
      </div>
      <p className="state-label">{label}</p>
    </div>
  );
}
