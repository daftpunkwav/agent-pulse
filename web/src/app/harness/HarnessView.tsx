"use client";

import { useEffect, useState } from "react";
import useSWR from "swr";
import { ApiError, createSchemaFetcher, postJson } from "@/lib/api";
import { harnessVersionsResponseSchema } from "@/lib/schemas";
import {
  agentPathSegment,
  sanitizeAgentName,
} from "@/lib/validation";
import { PageHeader } from "@/components/PageHeader";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

const AGENT_DEBOUNCE_MS = 400;

export function HarnessView() {
  const [agentInput, setAgentInput] = useState("interview-agent");
  const [activeAgent, setActiveAgent] = useState("interview-agent");
  const [validationError, setValidationError] = useState<string | null>(null);
  const [promoteError, setPromoteError] = useState<string | null>(null);
  const [promotingVersion, setPromotingVersion] = useState<number | null>(null);

  useEffect(() => {
    const safe = sanitizeAgentName(agentInput);
    const timer = setTimeout(() => {
      if (safe) {
        setValidationError(null);
        setActiveAgent(safe);
      } else if (agentInput.trim()) {
        setValidationError("Agent 名称仅允许小写字母、数字和连字符");
        setActiveAgent("");
      } else {
        setValidationError(null);
        setActiveAgent("");
      }
    }, AGENT_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [agentInput]);

  const safeAgent = sanitizeAgentName(activeAgent);
  const { data, error, isLoading, mutate } = useSWR(
    safeAgent
      ? `/harness/${agentPathSegment(safeAgent)}/versions`
      : null,
    createSchemaFetcher(harnessVersionsResponseSchema)
  );

  const handleAgentChange = (value: string) => {
    setAgentInput(value);
  };

  const handlePromote = async (version: number) => {
    if (!safeAgent) return;
    if (!confirm(`确定要将版本 ${version} 提升为 production 吗？`)) return;

    setPromoteError(null);
    setPromotingVersion(version);
    try {
      await postJson(
        `/harness/${agentPathSegment(safeAgent)}/versions/${version}/promote`
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
      <PageHeader title="Harness Management" subtitle="Agent 配置版本化与灰度发布" />

      <div className="card mb-6">
        <div className="flex items-center gap-2">
          <label className="text-sm text-slate-500" htmlFor="harness-agent">
            Agent:
          </label>
          <input
            id="harness-agent"
            type="text"
            value={agentInput}
            onChange={(e) => handleAgentChange(e.target.value)}
            className="input max-w-xs"
          />
        </div>
        {validationError && (
          <p className="mt-2 text-sm text-red-600">{validationError}</p>
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
            <table className="data-table">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Status</th>
                  <th>Hash</th>
                  <th className="text-right">Traffic</th>
                  <th className="text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                {data.versions.map((v) => (
                  <tr key={v.version}>
                    <td className="font-semibold text-slate-800">v{v.version}</td>
                    <td>
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
                    <td className="font-mono text-xs text-slate-500">
                      {v.config_hash ? `${v.config_hash.substring(0, 12)}...` : "—"}
                    </td>
                    <td className="text-right font-mono tabular-nums">
                      {v.traffic_percent}%
                    </td>
                    <td className="text-right">
                      {v.status !== "production" && (
                        <button
                          type="button"
                          onClick={() => void handlePromote(v.version)}
                          disabled={promotingVersion !== null}
                          className="btn btn-primary px-2.5 py-1 text-xs"
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
