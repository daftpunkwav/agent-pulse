"use client";

import { useState } from "react";
import useSWR from "swr";
import { ApiError, postJson, swrFetcher } from "@/lib/api";
import { harnessVersionsResponseSchema } from "@/lib/schemas";
import {
  agentPathSegment,
  sanitizeAgentName,
} from "@/lib/validation";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function HarnessView() {
  const [agentInput, setAgentInput] = useState("interview-agent");
  const [activeAgent, setActiveAgent] = useState("interview-agent");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [promoteError, setPromoteError] = useState<string | null>(null);
  const [promotingVersion, setPromotingVersion] = useState<number | null>(null);

  const safeAgent = sanitizeAgentName(activeAgent);
  const { data, error, isLoading, mutate } = useSWR(
    safeAgent
      ? `/api/backend/harness/${agentPathSegment(safeAgent)}/versions`
      : null,
    async (url: string) =>
      harnessVersionsResponseSchema.parse(await swrFetcher(url))
  );

  const handleAgentChange = (value: string) => {
    setAgentInput(value);
    const safe = sanitizeAgentName(value);
    if (safe) {
      setValidationError(null);
      setActiveAgent(safe);
    } else if (value.trim()) {
      setValidationError("Agent 名称仅允许小写字母、数字和连字符");
    } else {
      setValidationError(null);
    }
  };

  const handlePromote = async (version: number) => {
    if (!safeAgent) return;
    if (!confirm(`确定要将版本 ${version} 提升为 production 吗？`)) return;

    setPromoteError(null);
    setPromotingVersion(version);
    try {
      await postJson(
        `/api/backend/harness/${agentPathSegment(safeAgent)}/versions/${version}/promote`
      );
      await mutate();
    } catch (err) {
      const message =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "提升失败";
      setPromoteError(message);
    } finally {
      setPromotingVersion(null);
    }
  };

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Harness Management</h2>
        <p className="text-sm text-gray mt-1">Agent 配置版本化与灰度发布</p>
      </div>

      <div className="card mb-6">
        <div className="flex items-center gap-2">
          <label className="text-sm text-gray" htmlFor="harness-agent">
            Agent:
          </label>
          <input
            id="harness-agent"
            type="text"
            value={agentInput}
            onChange={(e) => handleAgentChange(e.target.value)}
            className="input"
            style={{ maxWidth: 300 }}
          />
        </div>
        {validationError && (
          <p className="text-sm mt-2" style={{ color: "#dc2626" }}>
            {validationError}
          </p>
        )}
      </div>

      {promoteError && (
        <div className="mb-4">
          <ErrorState message={promoteError} onRetry={() => setPromoteError(null)} />
        </div>
      )}

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => void mutate()}
        />
      )}

      {!isLoading && !error && (
        <div className="card">
          <div className="card-header">
            <h3 className="card-title">版本列表</h3>
          </div>
          {data?.versions && data.versions.length > 0 ? (
            <table style={{ width: "100%", fontSize: "0.875rem" }}>
              <thead>
                <tr style={{ borderBottom: "1px solid #e5e7eb" }}>
                  <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Version</th>
                  <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Status</th>
                  <th style={{ textAlign: "left", padding: "0.5rem 0" }}>Hash</th>
                  <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Traffic</th>
                  <th style={{ textAlign: "right", padding: "0.5rem 0" }}>Action</th>
                </tr>
              </thead>
              <tbody>
                {data.versions.map((v) => (
                  <tr key={v.version} style={{ borderBottom: "1px solid #f3f4f6" }}>
                    <td style={{ padding: "0.5rem 0" }}>
                      <strong>v{v.version}</strong>
                    </td>
                    <td style={{ padding: "0.5rem 0" }}>
                      <span
                        className={`badge ${
                          v.status === "production"
                            ? "badge-success"
                            : v.status === "canary"
                              ? "badge-warn"
                              : "badge-info"
                        }`}
                      >
                        {v.status}
                      </span>
                    </td>
                    <td
                      className="text-mono"
                      style={{ padding: "0.5rem 0", fontSize: "0.75rem" }}
                    >
                      {v.config_hash
                        ? `${v.config_hash.substring(0, 12)}...`
                        : "—"}
                    </td>
                    <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                      {v.traffic_percent}%
                    </td>
                    <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                      {v.status !== "production" && (
                        <button
                          type="button"
                          onClick={() => void handlePromote(v.version)}
                          disabled={promotingVersion !== null}
                          className="btn btn-primary"
                          style={{ fontSize: "0.75rem", padding: "0.25rem 0.5rem" }}
                        >
                          {promotingVersion === v.version ? "处理中..." : "Promote"}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <EmptyState message="暂无版本" />
          )}
        </div>
      )}
    </div>
  );
}
