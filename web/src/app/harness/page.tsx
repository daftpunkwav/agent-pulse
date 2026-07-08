"use client";

import { useState } from "react";
import useSWR from "swr";

const fetcher = (url: string) => fetch(url).then((r) => r.json());

interface HarnessVersion {
  version: number;
  status: string;
  config_hash: string;
  notes: string;
  traffic_percent: number;
  created_at: string;
  promoted_at: string | null;
}

export default function HarnessPage() {
  const [agentName, setAgentName] = useState("interview-agent");

  const { data, mutate } = useSWR<{ versions: HarnessVersion[] }>(
    `/api/backend/harness/${agentName}/versions`,
    fetcher
  );

  const handlePromote = async (version: number) => {
    if (!confirm(`确定要将版本 ${version} 提升为 production 吗？`)) return;
    await fetch(`/api/backend/harness/${agentName}/versions/${version}/promote`, {
      method: "POST",
    });
    mutate();
  };

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Harness Management</h2>
        <p className="text-sm text-gray mt-1">Agent 配置版本化与灰度发布</p>
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
                    {v.config_hash.substring(0, 12)}...
                  </td>
                  <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                    {v.traffic_percent}%
                  </td>
                  <td style={{ textAlign: "right", padding: "0.5rem 0" }}>
                    {v.status !== "production" && (
                      <button
                        onClick={() => handlePromote(v.version)}
                        className="btn btn-primary"
                        style={{ fontSize: "0.75rem", padding: "0.25rem 0.5rem" }}
                      >
                        Promote
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="text-sm text-gray">暂无版本</p>
        )}
      </div>
    </div>
  );
}