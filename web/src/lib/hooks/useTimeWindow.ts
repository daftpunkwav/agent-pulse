"use client";

import { useEffect, useState } from "react";
import { timeWindowParams } from "@/lib/validation";

/** 截断到分钟，兼顾 SWR key 稳定与「最近 N 小时」随时间推进 */
function truncateToMinute(date: Date): Date {
  const copy = new Date(date);
  copy.setSeconds(0, 0);
  return copy;
}

/**
 * 稳定的时间窗口查询参数，供 SWR key 使用。
 *
 * 每分钟对齐刷新 `to`，避免页面长时间打开后窗口冻结在首次渲染时刻。
 */
export function useTimeWindow(options: { hours?: number; days?: number } = { hours: 24 }): string {
  const { hours = 24, days } = options;
  const spanKey = days != null ? `d:${days}` : `h:${hours}`;

  const [now, setNow] = useState(() => truncateToMinute(new Date()));

  useEffect(() => {
    const tick = () => setNow(truncateToMinute(new Date()));
    // 对齐到下一分钟再周期刷新，减少无意义 revalidate
    const msToNextMinute = 60_000 - (Date.now() % 60_000);
    let intervalId: ReturnType<typeof setInterval> | undefined;
    const timeoutId = setTimeout(() => {
      tick();
      intervalId = setInterval(tick, 60_000);
    }, msToNextMinute);
    return () => {
      clearTimeout(timeoutId);
      if (intervalId) clearInterval(intervalId);
    };
  }, [spanKey]);

  const to = now;
  const msBack = days != null ? days * 24 * 3600 * 1000 : hours * 3600 * 1000;
  const from = truncateToMinute(new Date(to.getTime() - msBack));
  return timeWindowParams(from, to);
}
