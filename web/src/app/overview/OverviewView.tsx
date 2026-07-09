"use client";

import useSWR from "swr";
import { Activity, DollarSign, AlertCircle, CheckCircle } from "lucide-react";
import { createSchemaFetcher } from "@/lib/api";
import {
  clustersResponseSchema,
  costTotalSchema,
} from "@/lib/schemas";
import { useTimeWindow } from "@/lib/hooks/useTimeWindow";
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
      <div className="mb-6">
        <h2 className="text-2xl">Overview</h2>
        <p className="text-sm text-gray mt-1">最近 24 小时 Agent 运行总览</p>
      </div>

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
          <div className="grid grid-cols-4 mb-6">
            <MetricCard
              title="总成本 (24h)"
              value={cost ? `$${cost.total_usd.toFixed(4)}` : "—"}
              icon={<DollarSign className="h-5 w-5 text-green-600" />}
            />
            <MetricCard
              title="总 Token 数"
              value={cost ? cost.total_tokens.toLocaleString() : "—"}
              icon={<Activity className="h-5 w-5 text-blue-600" />}
            />
            <MetricCard
              title="失败聚类"
              value={clusters?.count ?? clusters?.clusters.length ?? 0}
              icon={<AlertCircle className="h-5 w-5 text-orange-600" />}
            />
            <MetricCard
              title="系统状态"
              value="运行中"
              icon={<CheckCircle className="h-5 w-5 text-green-600" />}
            />
          </div>

          <div className="card">
            <div className="card-header">
              <h3 className="card-title">最近失败聚类</h3>
            </div>
            {clusters?.clusters && clusters.clusters.length > 0 ? (
              <table style={{ width: "100%", fontSize: "0.875rem" }}>
                <thead>
                  <tr style={{ borderBottom: "1px solid #e5e7eb" }}>
                    <th style={{ textAlign: "left", padding: "0.5rem 0" }}>聚类名</th>
                    <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Trace 数</th>
                    <th style={{ textAlign: "right", padding: "0.5rem 0" }}>占比</th>
                  </tr>
                </thead>
                <tbody>
                  {clusters.clusters.slice(0, 5).map((c) => (
                    <tr key={c.id} style={{ borderBottom: "1px solid #f3f4f6" }}>
                      <td style={{ padding: "0.5rem 0" }}>{c.name}</td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                        {c.trace_count}
                      </td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
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

function MetricCard({
  title,
  value,
  icon,
}: {
  title: string;
  value: string | number;
  icon: React.ReactNode;
}) {
  return (
    <div className="card">
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm text-gray">{title}</span>
        {icon}
      </div>
      <div className="text-3xl">{value}</div>
    </div>
  );
}
