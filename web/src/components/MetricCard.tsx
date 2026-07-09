import clsx from "clsx";
import type { ReactNode } from "react";

type MetricVariant = "cost" | "tokens" | "alert" | "status";

const variantClass: Record<MetricVariant, string> = {
  cost: "metric-card--cost",
  tokens: "metric-card--tokens",
  alert: "metric-card--alert",
  status: "metric-card--status",
};

/** 指标卡 */
export function MetricCard({
  label,
  value,
  icon,
  variant = "cost",
}: {
  label: string;
  value: string | number;
  icon?: ReactNode;
  variant?: MetricVariant;
}) {
  return (
    <div className={clsx("metric-card", variantClass[variant])}>
      <div className="flex items-start justify-between gap-3">
        <span className="metric-label">{label}</span>
        {icon && (
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-600">
            {icon}
          </span>
        )}
      </div>
      <div className="metric-value">{value}</div>
    </div>
  );
}
