/** 通用加载态 */
export function LoadingState({ label = "加载中..." }: { label?: string }) {
  return <p className="text-sm text-gray">{label}</p>;
}
