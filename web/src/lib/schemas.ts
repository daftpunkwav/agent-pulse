/** API 响应 Zod Schema — 与 backend domain JSON 标签对齐 */

import { z } from "zod";

const timeWindowSchema = z.object({
  from: z.string(),
  to: z.string(),
});

export const costTotalSchema = z.object({
  window: timeWindowSchema.optional(),
  total_usd: z.number(),
  total_tokens: z.number(),
});

export const clusterSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional().default(""),
  trace_count: z.number(),
  percentage: z.number(),
  common_pattern: z.string().optional().default(""),
  suggestion: z.string().optional().default(""),
  is_active: z.boolean().optional().default(true),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export const clustersResponseSchema = z.object({
  clusters: z.array(clusterSchema),
  count: z.number().optional(),
});

export const spanSchema = z.object({
  id: z.string(),
  trace_id: z.string(),
  parent_span_id: z.string().optional().default(""),
  session_id: z.string().optional().default(""),
  user_id: z.string().optional().default(""),
  agent_name: z.string().optional().default(""),
  service_name: z.string().optional().default(""),
  environment: z.string().optional().default(""),
  type: z.string(),
  name: z.string(),
  status: z.string(),
  start_time: z.string().optional().default(""),
  end_time: z.string().optional().default(""),
  latency_ms: z.number().optional().default(0),
  model: z.string().optional().default(""),
  prompt_tokens: z.number().optional().default(0),
  completion_tokens: z.number().optional().default(0),
  total_tokens: z.number().optional().default(0),
  cost_usd: z.number().optional().default(0),
  finish_reason: z.string().optional().default(""),
  tool_name: z.string().optional().default(""),
  reasoning_step: z.number().optional().default(0),
  input_preview: z.string().optional().default(""),
  output_preview: z.string().optional().default(""),
  error_message: z.string().optional().default(""),
});

export const traceResponseSchema = z.object({
  trace: z.object({
    trace_id: z.string(),
    session_id: z.string().optional().default(""),
    user_id: z.string().optional().default(""),
    start_time: z.string().optional(),
    end_time: z.string().optional(),
    depth: z.number().optional(),
    all_spans: z.array(spanSchema).optional().default([]),
  }),
});

export const costBreakdownItemSchema = z.object({
  key: z.string(),
  cost_usd: z.number(),
  tokens: z.number(),
  call_count: z.number(),
  rank: z.number().optional(),
});

export const costBreakdownSchema = z.object({
  dimension: z.string(),
  window: timeWindowSchema.optional(),
  items: z.array(costBreakdownItemSchema),
  total_usd: z.number(),
  total_tokens: z.number(),
});

export const costBreakdownResponseSchema = z.object({
  window: timeWindowSchema.optional(),
  breakdowns: z.array(costBreakdownSchema),
});

export const timelinePointSchema = z.object({
  bucket: z.string(),
  cost_usd: z.number(),
  tokens: z.number(),
  call_count: z.number(),
});

export const costTimelineResponseSchema = z.object({
  window: timeWindowSchema.optional(),
  granularity: z.string().optional(),
  points: z.array(timelinePointSchema),
});

export const evalScoresSchema = z.object({
  accuracy: z.number().optional().default(0),
  completeness: z.number().optional().default(0),
  tool_selection: z.number().optional().default(0),
  reasoning_depth: z.number().optional().default(0),
  helpfulness: z.number().optional().default(0),
});

export const evalScoresResponseSchema = z.object({
  agent: z.string().optional(),
  window: timeWindowSchema.optional(),
  scores: evalScoresSchema,
});

export const harnessVersionSchema = z.object({
  id: z.string().optional(),
  agent_name: z.string().optional(),
  version: z.number(),
  status: z.string(),
  config_hash: z.string(),
  config_yaml: z.string().optional(),
  notes: z.string().optional().default(""),
  traffic_percent: z.number(),
  created_by: z.string().optional(),
  created_at: z.string(),
  promoted_at: z.string().nullable().optional(),
});

export const harnessVersionsResponseSchema = z.object({
  agent: z.string().optional(),
  versions: z
    .array(harnessVersionSchema)
    .nullable()
    .transform((v) => v ?? []),
  count: z.number().optional(),
});

// 导出推断类型，供组件使用
export type CostTotal = z.infer<typeof costTotalSchema>;
export type Cluster = z.infer<typeof clusterSchema>;
export type Span = z.infer<typeof spanSchema>;
export type HarnessVersion = z.infer<typeof harnessVersionSchema>;
