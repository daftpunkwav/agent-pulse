"use client";

import { useMemo, useState } from "react";
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
import { timeWindowParams } from "@/lib/validation";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function CostView() {
  const [days, setDays] = useState(7);

  const windowParams = useMemo(() => {
    const to = new Date();
    to.setMilliseconds(0);
    const from = new Date(to.getTime() - days * 24 * 3600 * 1000);
    from.setMilliseconds(0);
    return timeWindowParams(from, to);
  }, [days]);

  const {
    data: breakdown,
    error: breakdownError,
    isLoading: breakdownLoading,
    mutate: mutateBreakdown,
  } = useSWR(
    `/api/backend/cost/breakdown?${windowParams}&dimensions=user,agent,model`,
    createSchemaFetcher(costBreakdownResponseSchema)
  );

  const {
    data: timeline,
    error: timelineError,
    isLoading: timelineLoading,
    mutate: mutateTimeline,
  } = useSWR(
    `/api/backend/cost/timeline?${windowParams}&granularity=day`,
    createSchemaFetcher(costTimelineResponseSchema)
  );

  const isLoading = breakdownLoading || timelineLoading;
  const error = breakdownError ?? timelineError;

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Cost</h2>
        <p className="text-sm text-gray mt-1">多维度成本归因分析</p>
      </div>

      <div className="card mb-6">
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray">时间窗口：</span>
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
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis
                      dataKey="bucket"
                      tickFormatter={(v) => new Date(v).toLocaleDateString()}
                    />
                    <YAxis />
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
                      stroke="#2563eb"
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
              <table style={{ width: "100%", fontSize: "0.875rem" }}>
                <thead>
                  <tr style={{ borderBottom: "1px solid #e5e7eb" }}>
                    <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Key</th>
                    <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Cost</th>
                    <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Tokens</th>
                    <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Calls</th>
                  </tr>
                </thead>
                <tbody>
                  {bd.items.slice(0, 10).map((item) => (
                    <tr key={item.key} style={{ borderBottom: "1px solid #f3f4f6" }}>
                      <td className="text-mono" style={{ padding: "0.5rem 0" }}>
                        {item.key}
                      </td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                        ${item.cost_usd.toFixed(4)}
                      </td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                        {item.tokens.toLocaleString()}
                      </td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
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
