"use client";

import { useState } from "react";
import useSWR from "swr";
import { RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, ResponsiveContainer } from "recharts";

const fetcher = (url: string) => fetch(url).then((r) => r.json());

interface EvalScores {
  accuracy: number;
  completeness: number;
  tool_selection: number;
  reasoning_depth: number;
  helpfulness: number;
}

export default function EvalPage() {
  const [agentName, setAgentName] = useState("interview-agent");
  const windowParams = `from=${encodeURIComponent(new Date(Date.now() - 7 * 24 * 3600 * 1000).toISOString())}&to=${encodeURIComponent(new Date().toISOString())}`;

  const { data } = useSWR<{ scores: EvalScores }>(
    `/api/backend/eval/agents/${agentName}/scores?${windowParams}`,
    fetcher
  );

  const radarData = data?.scores
    ? [
        { dimension: "Accuracy", value: data.scores.accuracy || 0 },
        { dimension: "Completeness", value: data.scores.completeness || 0 },
        { dimension: "Tool Selection", value: data.scores.tool_selection || 0 },
        { dimension: "Reasoning", value: data.scores.reasoning_depth || 0 },
        { dimension: "Helpfulness", value: data.scores.helpfulness || 0 },
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
          <label className="text-sm text-gray">Agent:</label>
          <input
            type="text"
            value={agentName}
            onChange={(e) => setAgentName(e.target.value)}
            className="input"
            style={{ maxWidth: 300 }}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 mb-6">
        {/* 雷达图 */}
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
                    name={agentName}
                    dataKey="value"
                    stroke="#2563eb"
                    fill="#2563eb"
                    fillOpacity={0.3}
                  />
                </RadarChart>
              </ResponsiveContainer>
            ) : (
              <p className="text-sm text-gray">暂无数据</p>
            )}
          </div>
        </div>

        {/* 数字详情 */}
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
                      <strong>{(value as number).toFixed(3)}</strong>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="text-sm text-gray">暂无数据</p>
          )}
        </div>
      </div>
    </div>
  );
}