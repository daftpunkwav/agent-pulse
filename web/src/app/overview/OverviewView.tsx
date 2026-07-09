"use client";

import useSWR from "swr";
import { Activity, DollarSign, AlertCircle, CheckCircle } from "lucide-react";
import { createSchemaFetcher } from "@/lib/api";
import {
  clustersResponseSchema,
  costTotalSchema,
} from "@/lib/schemas";
import { useTimeWindow } from "@/lib/hooks/useTimeWindow";
import { PageHeader } from "@/components/PageHeader";
import { MetricCard } from "@/components/MetricCard";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function OverviewView() {
  const window = useTimeWindow({ hours: 24 });

  const {
    data: cost,
    error: costError,
    isLoading: costLoading,
    mutate: mutateCost,
  } = useSWR(`/api/backend/cost/total?${window}`, createSchemaFetcher(costTotalSchema));

  const {
    data: clusters,
    error: clustersError,
    isLoading: clustersLoading,
    mutate: mutateClusters,
  } = useSWR(
    `/api/backend/clusters?active_only=true`,
    createSchemaFetcher(clustersResponseSchema)
  );

  const isLoading = costLoading || clustersLoading;
  const error = costError ?? clustersError;

  return (
    <div>
      <PageHeader title="Overview" subtitle="最近 24 小时 Agent 运行总览" />

      {isLoading ? (
        <LoadingState />
      ) : error ? (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => {
            void mutateCost();
            void mutateClusters();
          }}
        />
      ) : (
        <>
          <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label="总成本 (24h)"
              value={cost ? `$${cost.total_usd.toFixed(4)}` : "—"}
              variant="cost"
              icon={<DollarSign className="h-4 w-4 text-emerald-600" />}
            />
            <MetricCard
              label="总 Token 数"
              value={cost ? cost.total_tokens.toLocaleString() : "—"}
              variant="tokens"
              icon={<Activity className="h-4 w-4 text-cyan-600" />}
            />
            <MetricCard
              label="失败聚类"
              value={clusters?.count ?? clusters?.clusters.length ?? 0}
              variant="alert"
              icon={<AlertCircle className="h-4 w-4 text-amber-600" />}
            />
            <MetricCard
              label="系统状态"
              value="运行中"
              variant="status"
              icon={<CheckCircle className="h-4 w-4 text-emerald-500" />}
            />
          </div>

          <div className="card">
            <div className="card-header">
              <h3 className="card-title">最近失败聚类</h3>
            </div>
            {clusters?.clusters && clusters.clusters.length > 0 ? (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>聚类名</th>
                    <th className="text-right">Trace 数</th>
                    <th className="text-right">占比</th>
                  </tr>
                </thead>
                <tbody>
                  {clusters.clusters.slice(0, 5).map((c) => (
                    <tr key={c.id}>
                      <td className="font-medium text-slate-800 dark:text-slate-200">{c.name}</td>
                      <td className="text-right font-mono tabular-nums">
                        {c.trace_count}
                      </td>
                      <td className="text-right font-mono tabular-nums">
                        {(c.percentage * 100).toFixed(1)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <EmptyState message="暂无失败聚类" />
            )}
          </div>
        </>
      )}
    </div>
  );
}
