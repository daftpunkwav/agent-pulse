"use client";

import { useState } from "react";
import useSWR from "swr";
import { Search } from "lucide-react";

const fetcher = (url: string) => fetch(url).then((r) => r.json());

interface Span {
  id: string;
  trace_id: string;
  parent_span_id: string;
  session_id: string;
  user_id: string;
  agent_name: string;
  type: string;
  name: string;
  status: string;
  start_time: string;
  latency_ms: number;
  model: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_usd: number;
}

export default function TracesPage() {
  const [traceId, setTraceId] = useState("");
  const [searchTerm, setSearchTerm] = useState("");

  const { data, error, isLoading } = useSWR<{ trace: any }>(
    traceId ? `/api/backend/traces/${traceId}` : null,
    fetcher
  );

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Traces</h2>
        <p className="text-sm text-gray mt-1">查询 Agent 调用 Trace</p>
      </div>

      <div className="card mb-6">
        <div className="flex gap-2">
          <input
            type="text"
            value={traceId}
            onChange={(e) => setTraceId(e.target.value)}
            placeholder="输入 Trace ID 查询完整调用树..."
            className="input"
          />
          <button
            onClick={() => setSearchTerm(traceId)}
            className="btn btn-primary"
          >
            <Search className="h-4 w-4" />
            查询
          </button>
        </div>
      </div>

      {isLoading && <p className="text-sm text-gray">加载中...</p>}
      {error && <p className="text-sm" style={{ color: "#dc2626" }}>查询失败</p>}

      {data?.trace && (
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Trace: {data.trace.trace_id}</h3>
            <span className="badge badge-info">{data.trace.all_spans?.length} Spans</span>
          </div>
          <div className="text-sm text-gray mb-4">
            Session: {data.trace.session_id} · User: {data.trace.user_id}
          </div>

          {/* Span 列表 */}
          <table style={{ width: "100%", fontSize: "0.875rem" }}>
            <thead>
              <tr style={{ borderBottom: "1px solid #e5e7eb" }}>
                <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Type</th>
                <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Name</th>
                <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Status</th>
                <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Latency</th>
                <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Tokens</th>
                <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Cost</th>
              </tr>
            </thead>
            <tbody>
              {data.trace.all_spans?.map((span: Span) => (
                <tr key={span.id} style={{ borderBottom: "1px solid #f3f4f6" }}>
                  <td style={{ padding: "0.5rem 0" }}>
                    <span className="badge badge-info">{span.type}</span>
                  </td>
                  <td className="text-mono" style={{ padding: "0.5rem 0", fontSize: "0.75rem" }}>
                    {span.name}
                  </td>
                  <td style={{ padding: "0.5rem 0" }}>
                    <span
                      className={`badge ${
                        span.status === "ok" ? "badge-success" : "badge-error"
                      }`}
                    >
                      {span.status}
                    </span>
                  </td>
                  <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                    {span.latency_ms}ms
                  </td>
                  <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                    {span.total_tokens || "—"}
                  </td>
                  <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                    ${span.cost_usd?.toFixed(6) || "0"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}