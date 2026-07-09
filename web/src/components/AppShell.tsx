"use client";

import { Sidebar } from "@/components/Sidebar";

/** 应用外壳：侧栏 + 主内容区 */
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen bg-surface">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-7xl px-6 py-8 lg:px-10 lg:py-10">{children}</div>
      </main>
    </div>
  );
}
