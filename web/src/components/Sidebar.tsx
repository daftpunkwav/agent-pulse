"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import useSWR from "swr";
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
import { ThemeToggle } from "@/components/ThemeToggle";

const navigation = [
  { name: "Overview", href: "/", icon: Activity },
  { name: "Traces", href: "/traces", icon: ListChecks },
  { name: "Cost", href: "/cost", icon: DollarSign },
  { name: "Evaluation", href: "/eval", icon: Gauge },
  { name: "Failure Clusters", href: "/clusters", icon: FlaskConical },
  { name: "Harness", href: "/harness", icon: Beaker },
];

async function healthFetcher(url: string): Promise<{ ok: boolean }> {
  try {
    const res = await fetch(url, { cache: "no-store" });
    return { ok: res.ok };
  } catch {
    return { ok: false };
  }
}

export function Sidebar() {
  const pathname = usePathname();
  const { data: health } = useSWR("/api/backend/healthz", healthFetcher, {
    refreshInterval: 30_000,
    revalidateOnFocus: true,
    shouldRetryOnError: false,
  });

  const statusLabel =
    health == null ? "检测中…" : health.ok ? "系统在线" : "系统离线";

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="sidebar-logo-icon">
          <HeartPulse className="h-5 w-5 text-cyan-600 dark:text-cyan-400" strokeWidth={2.25} />
        </div>
        <div>
          <h1 className="sidebar-brand">
            Agent<span className="text-cyan-600 dark:text-cyan-400">Pulse</span>
          </h1>
          <p className="text-[10px] font-medium uppercase tracking-widest text-[color:var(--ap-fg-subtle)]">
            AgentOps
          </p>
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 px-3 py-5">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={clsx(
                "group sidebar-nav-link",
                isActive && "sidebar-nav-link--active"
              )}
            >
              <Icon
                className={clsx(
                  "h-4 w-4 shrink-0 transition-colors",
                  isActive
                    ? "text-cyan-600 dark:text-cyan-400"
                    : "text-[color:var(--ap-fg-subtle)] group-hover:text-[color:var(--ap-sidebar-fg-hover)]"
                )}
              />
              {item.name}
            </Link>
          );
        })}
      </nav>

      <div className="sidebar-footer">
        <ThemeToggle />
        <div className="flex items-center gap-2 px-1">
          <span
            className={clsx(
              "pulse-dot",
              health?.ok === false && "opacity-40 grayscale"
            )}
          />
          <span className="text-xs text-[color:var(--ap-fg-subtle)]">{statusLabel}</span>
        </div>
        <p className="px-1 font-mono text-[11px] text-[color:var(--ap-fg-subtle)]">v0.1.0</p>
      </div>
    </aside>
  );
}
