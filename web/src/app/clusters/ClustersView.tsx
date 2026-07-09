"use client";

import useSWR from "swr";
import { createSchemaFetcher } from "@/lib/api";
import { clustersResponseSchema } from "@/lib/schemas";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function ClustersView() {
  const { data, error, isLoading, mutate } = useSWR(
    `/api/backend/clusters?active_only=true`,
    createSchemaFetcher(clustersResponseSchema)
  );

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl">Failure Clusters</h2>
        <p className="text-sm text-gray mt-1">失败模式聚类与改进建议</p>
      </div>

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => void mutate()}
        />
      )}

      {!isLoading && !error && data?.clusters && data.clusters.length > 0 ? (
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
        !isLoading &&
        !error && (
          <EmptyState message="暂无失败聚类。在 POST /api/v1/clusters/run 触发聚类。" />
        )
      )}
    </div>
  );
}
