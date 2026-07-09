"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Activity,
  DollarSign,
  Gauge,
  ListChecks,
  FlaskConical,
  Beaker,
  HeartPulse,
} from "lucide-react";
import clsx from "clsx";

const navigation = [
  { name: "Overview", href: "/", icon: Activity },
  { name: "Traces", href: "/traces", icon: ListChecks },
  { name: "Cost", href: "/cost", icon: DollarSign },
  { name: "Evaluation", href: "/eval", icon: Gauge },
  { name: "Failure Clusters", href: "/clusters", icon: FlaskConical },
  { name: "Harness", href: "/harness", icon: Beaker },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="relative flex w-64 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-slate-300">
      {/* Logo */}
      <div className="flex h-16 items-center gap-2.5 border-b border-sidebar-border px-6">
        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-cyan-500/10 ring-1 ring-cyan-500/30">
          <HeartPulse className="h-5 w-5 text-cyan-400" strokeWidth={2.25} />
        </div>
        <div>
          <h1 className="text-base font-semibold tracking-tight text-white">
            Agent<span className="text-cyan-400">Pulse</span>
          </h1>
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">
            AgentOps
          </p>
        </div>
      </div>

      {/* 导航 */}
      <nav className="flex-1 space-y-0.5 px-3 py-5">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={clsx(
                "group flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-all",
                isActive
                  ? "bg-cyan-500/10 text-cyan-300 ring-1 ring-cyan-500/20"
                  : "text-slate-400 hover:bg-sidebar-hover hover:text-slate-200"
              )}
            >
              <Icon
                className={clsx(
                  "h-4 w-4 shrink-0 transition-colors",
                  isActive ? "text-cyan-400" : "text-slate-500 group-hover:text-slate-300"
                )}
              />
              {item.name}
            </Link>
          );
        })}
      </nav>

      {/* 版本信息 */}
      <div className="border-t border-sidebar-border px-6 py-4">
        <div className="flex items-center gap-2">
          <span className="pulse-dot" />
          <span className="text-xs text-slate-500">系统在线</span>
        </div>
        <p className="mt-1.5 font-mono text-[11px] text-slate-600">v0.1.0</p>
      </div>
    </aside>
  );
}
