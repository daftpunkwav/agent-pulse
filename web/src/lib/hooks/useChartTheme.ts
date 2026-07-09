"use client";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

/** 图表在深浅色主题下的配色 */
export function useChartTheme() {
  const { resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => setMounted(true), []);

  const isDark = mounted && resolvedTheme === "dark";

  return {
    stroke: isDark ? "#22d3ee" : "#0891b2",
    fill: isDark ? "#22d3ee" : "#0891b2",
    grid: isDark ? "#334155" : "#e2e8f0",
    tick: isDark ? "#94a3b8" : "#64748b",
  };
}
