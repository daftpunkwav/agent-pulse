"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { createSchemaFetcher } from "@/lib/api";
import {
  costBreakdownResponseSchema,
  costTimelineResponseSchema,
} from "@/lib/schemas";
import { useTimeWindow } from "@/lib/hooks/useTimeWindow";
import { useChartTheme } from "@/lib/hooks/useChartTheme";
import { PageHeader } from "@/components/PageHeader";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function CostView() {
  const [days, setDays] = useState(7);
  const windowParams = useTimeWindow({ days });

  const {
    data: breakdown,
    error: breakdownError,
    isLoading: breakdownLoading,
    mutate: mutateBreakdown,
  } = useSWR(
    `/cost/breakdown?${windowParams}&dimensions=user,agent,model`,
    createSchemaFetcher(costBreakdownResponseSchema)
  );

  const {
    data: timeline,
    error: timelineError,
    isLoading: timelineLoading,
    mutate: mutateTimeline,
  } = useSWR(
    `/cost/timeline?${windowParams}&granularity=day`,
    createSchemaFetcher(costTimelineResponseSchema)
  );

  const isLoading = breakdownLoading || timelineLoading;
  const error = breakdownError ?? timelineError;
  const chart = useChartTheme();

  return (
    <div>
      <PageHeader title="Cost" subtitle="多维度成本归因分析" />

      <div className="card mb-6">
        <div className="flex items-center gap-4">
          <span className="text-sm text-slate-500">时间窗口：</span>
          {[1, 7, 30].map((d) => (
            <button
              key={d}
              type="button"
              onClick={() => setDays(d)}
              className={`btn ${days === d ? "btn-primary" : "btn-secondary"}`}
            >
              最近 {d} 天
            </button>
          ))}
        </div>
      </div>

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => {
            void mutateBreakdown();
            void mutateTimeline();
          }}
        />
      )}

      {!isLoading && !error && (
        <>
          <div className="card mb-6">
            <div className="card-header">
              <h3 className="card-title">成本时间序列</h3>
            </div>
            <div style={{ height: 300 }}>
              {timeline?.points && timeline.points.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={timeline.points}>
                    <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
                    <XAxis
                      dataKey="bucket"
                      tickFormatter={(v) => new Date(v).toLocaleDateString()}
                      tick={{ fill: chart.tick, fontSize: 12 }}
                    />
                    <YAxis tick={{ fill: chart.tick, fontSize: 12 }} />
                    <Tooltip
                      labelFormatter={(v) => new Date(v).toLocaleString()}
                      formatter={(value: number) => [
                        `$${Number(value).toFixed(4)}`,
                        "Cost",
                      ]}
                    />
                    <Line
                      type="monotone"
                      dataKey="cost_usd"
                      stroke={chart.stroke}
                      strokeWidth={2}
                    />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <EmptyState message="暂无数据" />
              )}
            </div>
          </div>

          {breakdown?.breakdowns?.map((bd) => (
            <div className="card mb-6" key={bd.dimension}>
              <div className="card-header">
                <h3 className="card-title">
                  按 {bd.dimension} 归因 · 总计 ${bd.total_usd.toFixed(4)}
                </h3>
              </div>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Key</th>
                    <th className="text-right">Cost</th>
                    <th className="text-right">Tokens</th>
                    <th className="text-right">Calls</th>
                  </tr>
                </thead>
                <tbody>
                  {bd.items.slice(0, 10).map((item) => (
                    <tr key={item.key}>
                      <td className="font-mono text-xs">{item.key}</td>
                      <td className="text-right font-mono tabular-nums">
                        ${item.cost_usd.toFixed(4)}
                      </td>
                      <td className="text-right font-mono tabular-nums">
                        {item.tokens.toLocaleString()}
                      </td>
                      <td className="text-right font-mono tabular-nums">
                        {item.call_count}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </>
      )}
    </div>
  );
}
