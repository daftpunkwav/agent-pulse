"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Activity, DollarSign, Gauge, ListChecks, FlaskConical, Beaker } from "lucide-react";
import clsx from "clsx";

const navigation = [
  { name: "Overview", href: "/", icon: Activity },
  { name: "Traces", href: "/traces", icon: ListChecks },
  { name: "Cost", href: "/cost", icon: DollarSign },
  { name: "Evaluation", href: "/eval", icon: Gauge },
  { name: "Clusters", href: "/clusters", icon: FlaskConical },
  { name: "Harness", href: "/harness", icon: Beaker },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-60 border-r border-gray-200 bg-white">
      <div className="flex h-16 items-center border-b border-gray-200 px-6">
        <h1 className="text-xl font-bold text-gray-900">
          Agent<span className="text-blue-600">Pulse</span>
        </h1>
      </div>
      <nav className="space-y-1 px-3 py-4">
        {navigation.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={clsx(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-blue-50 text-blue-700"
                  : "text-gray-700 hover:bg-gray-100 hover:text-gray-900"
              )}
            >
              <Icon className="h-4 w-4" />
              {item.name}
            </Link>
          );
        })}
      </nav>
      <div className="absolute bottom-4 left-6 text-xs text-gray-400">
        <div>v0.1.0</div>
        <div className="mt-1">AgentOps Platform</div>
      </div>
    </aside>
  );
}