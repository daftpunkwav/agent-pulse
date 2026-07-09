"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  Radar,
  ResponsiveContainer,
} from "recharts";
import { swrFetcher } from "@/lib/api";
import { evalScoresResponseSchema } from "@/lib/schemas";
import {
  agentPathSegment,
  sanitizeAgentName,
  timeWindowParams,
} from "@/lib/validation";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function EvalView() {
  const [agentInput, setAgentInput] = useState("interview-agent");
  const [activeAgent, setActiveAgent] = useState("interview-agent");
  const [validationError, setValidationError] = useState<string | null>(null);

  const windowParams = timeWindowParams(
    new Date(Date.now() - 7 * 24 * 3600 * 1000),
    new Date()
  );

  const safeAgent = sanitizeAgentName(activeAgent);
  const { data, error, isLoading, mutate } = useSWR(
    safeAgent
      ? `/api/backend/eval/agents/${agentPathSegment(safeAgent)}/scores?${windowParams}`
      : null,
    async (url: string) =>
      evalScoresResponseSchema.parse(await swrFetcher(url))
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

  const radarData = data?.scores
    ? [
        { dimension: "Accuracy", value: data.scores.accuracy ?? 0 },
        { dimension: "Completeness", value: data.scores.completeness ?? 0 },
        { dimension: "Tool Selection", value: data.scores.tool_selection ?? 0 },
        { dimension: "Reasoning", value: data.scores.reasoning_depth ?? 0 },
        { dimension: "Helpfulness", value: data.scores.helpfulness ?? 0 },
      ]
    : [];

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Evaluation</h2>
        <p className="text-sm text-gray mt-1">LLM-as-Judge 五维评分</p>
      </div>

      <div className="card mb-6">
        <div className="flex items-center gap-2">
          <label className="text-sm text-gray" htmlFor="eval-agent">
            Agent:
          </label>
          <input
            id="eval-agent"
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

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => void mutate()}
        />
      )}

      {!isLoading && !error && (
        <div className="grid grid-cols-2 mb-6">
          <div className="card">
            <div className="card-header">
              <h3 className="card-title">维度雷达图</h3>
            </div>
            <div style={{ height: 400 }}>
              {radarData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <RadarChart data={radarData}>
                    <PolarGrid />
                    <PolarAngleAxis dataKey="dimension" />
                    <PolarRadiusAxis angle={90} domain={[0, 1]} />
                    <Radar
                      name={activeAgent}
                      dataKey="value"
                      stroke="#2563eb"
                      fill="#2563eb"
                      fillOpacity={0.3}
                    />
                  </RadarChart>
                </ResponsiveContainer>
              ) : (
                <EmptyState message="暂无数据" />
              )}
            </div>
          </div>

          <div className="card">
            <div className="card-header">
              <h3 className="card-title">分维度平均分</h3>
            </div>
            {data?.scores ? (
              <table style={{ width: "100%", fontSize: "0.875rem" }}>
                <tbody>
                  {Object.entries(data.scores).map(([key, value]) => (
                    <tr key={key} style={{ borderBottom: "1px solid #f3f4f6" }}>
                      <td style={{ padding: "0.5rem 0" }}>{key}</td>
                      <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                        <strong>{Number(value).toFixed(3)}</strong>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <EmptyState message="暂无数据" />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
