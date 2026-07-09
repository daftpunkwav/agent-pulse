"use client";

import { useTheme } from "next-themes";
import { Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

/** 浅色 / 深色主题切换 */
export function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return (
      <div className="h-9 w-full rounded-lg bg-slate-100 dark:bg-slate-800/60" aria-hidden />
    );
  }

  const isDark = resolvedTheme === "dark";

  return (
    <button
      type="button"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className="btn btn-ghost w-full justify-center"
      aria-label={isDark ? "切换到浅色模式" : "切换到深色模式"}
      title={theme === "system" ? "当前跟随系统" : isDark ? "浅色模式" : "深色模式"}
    >
      {isDark ? (
        <>
          <Sun className="h-4 w-4" />
          浅色模式
        </>
      ) : (
        <>
          <Moon className="h-4 w-4" />
          深色模式
        </>
      )}
    </button>
  );
}
