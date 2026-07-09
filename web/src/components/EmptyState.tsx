/** 通用空数据态 */
export function EmptyState({ message }: { message: string }) {
  return (
    <div className="card">
      <p className="text-sm text-gray">{message}</p>
    </div>
  );
}
