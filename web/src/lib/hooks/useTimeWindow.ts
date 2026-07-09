import { useMemo } from "react";
import { timeWindowParams } from "@/lib/validation";

/** 截断到秒级，避免 toISOString 毫秒变化导致 SWR key 不稳定 */
function truncateToSeconds(date: Date): Date {
  const copy = new Date(date);
  copy.setMilliseconds(0);
  return copy;
}

/** 稳定的时间窗口查询参数，供 SWR key 使用 */
export function useTimeWindow(options: { hours?: number; days?: number } = { hours: 24 }): string {
  const { hours = 24, days } = options;
  return useMemo(() => {
    const to = truncateToSeconds(new Date());
    const msBack = days != null ? days * 24 * 3600 * 1000 : hours * 3600 * 1000;
    const from = truncateToSeconds(new Date(to.getTime() - msBack));
    return timeWindowParams(from, to);
  }, [days ?? hours]);
}
