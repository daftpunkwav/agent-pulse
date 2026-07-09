"use client";

import { useState } from "react";
import useSWR from "swr";
import { Search } from "lucide-react";
import { swrFetcher } from "@/lib/api";
import { traceResponseSchema } from "@/lib/schemas";
import { sanitizeTraceId, tracePathSegment } from "@/lib/validation";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function TracesView() {
  const [traceIdInput, setTraceIdInput] = useState("");
  const [activeTraceId, setActiveTraceId] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  const { data, error, isLoading, mutate } = useSWR(
    activeTraceId
      ? `/api/backend/traces/${tracePathSegment(activeTraceId)}`
      : null,
    async (url: string) => traceResponseSchema.parse(await swrFetcher(url))
  );

  const handleSearch = () => {
    const safe = sanitizeTraceId(traceIdInput);
    if (!safe) {
      setValidationError("Trace ID 必须为 32 位十六进制字符串");
      setActiveTraceId(null);
      return;
    }
    setValidationError(null);
    setActiveTraceId(safe);
  };

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
            value={traceIdInput}
            onChange={(e) => setTraceIdInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            placeholder="输入 Trace ID 查询完整调用树..."
            className="input"
          />
          <button type="button" onClick={handleSearch} className="btn btn-primary">
            <Search className="h-4 w-4" />
            查询
          </button>
        </div>
        {validationError && (
          <p className="text-sm mt-2" style={{ color: "#dc2626" }}>
            {validationError}
          </p>
        )}
      </div>

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "查询失败"}
          onRetry={() => void mutate()}
        />
      )}

      {!isLoading && !error && activeTraceId && !data?.trace && (
        <EmptyState message="未找到 Trace 数据" />
      )}

      {data?.trace && (
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">Trace: {data.trace.trace_id}</h3>
            <span className="badge badge-info">
              {data.trace.all_spans?.length ?? 0} Spans
            </span>
          </div>
          <div className="text-sm text-gray mb-4">
            Session: {data.trace.session_id} · User: {data.trace.user_id}
          </div>

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
              {data.trace.all_spans?.map((span) => (
                <tr key={span.id} style={{ borderBottom: "1px solid #f3f4f6" }}>
                  <td style={{ padding: "0.5rem 0" }}>
                    <span className="badge badge-info">{span.type}</span>
                  </td>
                  <td
                    className="text-mono"
                    style={{ padding: "0.5rem 0", fontSize: "0.75rem" }}
                  >
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
                    ${(span.cost_usd ?? 0).toFixed(6)}
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
