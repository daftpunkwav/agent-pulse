"use client";

import useSWR from "swr";

const fetcher = (url: string) => fetch(url).then((r) => r.json());

interface Cluster {
  id: string;
  name: string;
  description: string;
  trace_count: number;
  percentage: number;
  common_pattern: string;
  suggestion: string;
}

export default function ClustersPage() {
  const { data } = useSWR<{ clusters: Cluster[] }>(
    `/api/backend/clusters?active_only=true`,
    fetcher
  );

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Failure Clusters</h2>
        <p className="text-sm text-gray mt-1">失败模式聚类与改进建议</p>
      </div>

      {data?.clusters && data.clusters.length > 0 ? (
        data.clusters.map((c) => (
          <div className="card mb-4" key={c.id}>
            <div className="card-header">
              <h3 className="card-title">{c.name}</h3>
              <div className="flex gap-2">
                <span className="badge badge-info">{c.trace_count} traces</span>
                <span className="badge badge-warn">
                  {(c.percentage * 100).toFixed(1)}%
                </span>
              </div>
            </div>
            <p className="text-sm mb-4">{c.description}</p>
            {c.common_pattern && (
              <div className="mb-4">
                <div className="text-xs text-gray mb-2">COMMON PATTERN</div>
                <pre
                  className="text-mono text-xs"
                  style={{
                    background: "#f9fafb",
                    padding: "0.5rem",
                    borderRadius: 4,
                    overflow: "auto",
                  }}
                >
                  {c.common_pattern}
                </pre>
              </div>
            )}
            {c.suggestion && (
              <div
                style={{
                  background: "#ecfdf5",
                  padding: "0.75rem",
                  borderRadius: 4,
                  fontSize: "0.875rem",
                }}
              >
                💡 <strong>建议:</strong> {c.suggestion}
              </div>
            )}
          </div>
        ))
      ) : (
        <div className="card">
          <p className="text-sm text-gray">暂无失败聚类。在 <code>POST /api/v1/clusters/run</code> 触发聚类。</p>
        </div>
      )}
    </div>
  );
}