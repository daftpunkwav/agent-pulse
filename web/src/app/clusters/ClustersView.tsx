"use client";

import useSWR from "swr";
import { createSchemaFetcher } from "@/lib/api";
import { clustersResponseSchema } from "@/lib/schemas";
import { PageHeader } from "@/components/PageHeader";
import { ErrorState } from "@/components/ErrorState";
import { LoadingState } from "@/components/LoadingState";
import { EmptyState } from "@/components/EmptyState";

export function ClustersView() {
  const { data, error, isLoading, mutate } = useSWR(
    `/clusters?active_only=true`,
    createSchemaFetcher(clustersResponseSchema)
  );

  return (
    <div>
      <PageHeader title="Failure Clusters" subtitle="失败模式聚类与改进建议" />

      {isLoading && <LoadingState />}
      {error && (
        <ErrorState
          message={error instanceof Error ? error.message : "加载失败"}
          onRetry={() => void mutate()}
        />
      )}

      {!isLoading && !error && data?.clusters && data.clusters.length > 0 ? (
        <div className="space-y-4">
          {data.clusters.map((c) => (
            <div className="card" key={c.id}>
              <div className="card-header">
                <h3 className="card-title">{c.name}</h3>
                <div className="flex gap-2">
                  <span className="badge badge-info">{c.trace_count} traces</span>
                  <span className="badge badge-warn">
                    {(c.percentage * 100).toFixed(1)}%
                  </span>
                </div>
              </div>
              <p className="mb-4 text-sm text-[color:var(--ap-fg-muted)]">{c.description}</p>
              {c.common_pattern && (
                <div className="mb-4">
                  <div className="mb-2 text-xs font-medium uppercase tracking-wide text-[color:var(--ap-fg-subtle)]">
                    Common Pattern
                  </div>
                  <pre className="code-block">
                    {c.common_pattern}
                  </pre>
                </div>
              )}
              {c.suggestion && (
                <div className="hint-box">
                  <strong>建议：</strong> {c.suggestion}
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        !isLoading &&
        !error && (
          <EmptyState message="暂无失败聚类。在 POST /api/v1/clusters/run 触发聚类。" />
        )
      )}
    </div>
  );
}
