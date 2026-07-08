"use client";

import { useState } from "react";
import useSWR from "swr";
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";

const fetcher = (url: string) => fetch(url).then((r) => r.json());

interface CostBreakdown {
  dimension: string;
  items: Array<{ key: string; cost_usd: number; tokens: number; call_count: number }>;
  total_usd: number;
  total_tokens: number;
}

interface TimelinePoint {
  bucket: string;
  cost_usd: number;
  tokens: number;
  call_count: number;
}

export default function CostPage() {
  const [days, setDays] = useState(7);

  const fromIso = new Date(Date.now() - days * 24 * 3600 * 1000).toISOString();
  const toIso = new Date().toISOString();
  const windowParams = `from=${encodeURIComponent(fromIso)}&to=${encodeURIComponent(toIso)}`;

  const { data: breakdown } = useSWR<{ breakdowns: CostBreakdown[] }>(
    `/api/backend/cost/breakdown?${windowParams}&dimensions=user,agent,model`,
    fetcher
  );
  const { data: timeline } = useSWR<{ points: TimelinePoint[] }>(
    `/api/backend/cost/timeline?${windowParams}&granularity=day`,
    fetcher
  );

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Cost</h2>
        <p className="text-sm text-gray mt-1">多维度成本归因分析</p>
      </div>

      {/* 时间窗口选择 */}
      <div className="card mb-6">
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray">时间窗口：</span>
          {[1, 7, 30].map((d) => (
            <button
              key={d}
              onClick={() => setDays(d)}
              className={`btn ${days === d ? "btn-primary" : "btn-secondary"}`}
            >
              最近 {d} 天
            </button>
          ))}
        </div>
      </div>

      {/* 成本时间序列 */}
      <div className="card mb-6">
        <div className="card-header">
          <h3 className="card-title">成本时间序列</h3>
        </div>
        <div style={{ height: 300 }}>
          {timeline?.points && timeline.points.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={timeline.points}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="bucket" tickFormatter={(v) => new Date(v).toLocaleDateString()} />
                <YAxis />
                <Tooltip
                  labelFormatter={(v) => new Date(v).toLocaleString()}
                  formatter={(value: number) => [`$${value.toFixed(4)}`, "Cost"]}
                />
                <Line type="monotone" dataKey="cost_usd" stroke="#2563eb" strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <p className="text-sm text-gray">暂无数据</p>
          )}
        </div>
      </div>

      {/* 五维归因表 */}
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
                  <td className="text-mono" style={{ padding: "0.5rem 0" }}>{item.key}</td>
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
    </div>
  );
}