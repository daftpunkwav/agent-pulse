/** API 响应 Zod Schema */

import { z } from "zod";

export const costTotalSchema = z.object({
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
  type: z.string(),
  name: z.string(),
  status: z.string(),
  start_time: z.string().optional().default(""),
  latency_ms: z.number().optional().default(0),
  model: z.string().optional().default(""),
  prompt_tokens: z.number().optional().default(0),
  completion_tokens: z.number().optional().default(0),
  total_tokens: z.number().optional().default(0),
  cost_usd: z.number().optional().default(0),
});

export const traceResponseSchema = z.object({
  trace: z.object({
    trace_id: z.string(),
    session_id: z.string().optional().default(""),
    user_id: z.string().optional().default(""),
    all_spans: z.array(spanSchema).optional().default([]),
  }),
});

export const costBreakdownSchema = z.object({
  dimension: z.string(),
  items: z.array(
    z.object({
      key: z.string(),
      cost_usd: z.number(),
      tokens: z.number(),
      call_count: z.number(),
    })
  ),
  total_usd: z.number(),
  total_tokens: z.number(),
});

export const costBreakdownResponseSchema = z.object({
  breakdowns: z.array(costBreakdownSchema),
});

export const timelinePointSchema = z.object({
  bucket: z.string(),
  cost_usd: z.number(),
  tokens: z.number(),
  call_count: z.number(),
});

export const costTimelineResponseSchema = z.object({
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
  scores: evalScoresSchema,
});

export const harnessVersionSchema = z.object({
  version: z.number(),
  status: z.string(),
  config_hash: z.string(),
  notes: z.string().optional().default(""),
  traffic_percent: z.number(),
  created_at: z.string(),
  promoted_at: z.string().nullable().optional(),
});

export const harnessVersionsResponseSchema = z.object({
  versions: z.array(harnessVersionSchema),
});
