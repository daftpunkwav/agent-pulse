/** 通用空数据态 */
export function EmptyState({ message }: { message: string }) {
  return (
    <div className="state-box">
      <p className="state-label">{message}</p>
    </div>
  );
}
