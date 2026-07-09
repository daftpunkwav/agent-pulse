/** 用户输入校验与 URL 安全编码 */

export const AGENT_NAME_PATTERN = /^[a-z0-9-]{1,64}$/;
export const TRACE_ID_PATTERN = /^[a-f0-9]{32}$/i;

/** 校验并规范化 agent 名称，非法时返回 null */
export function sanitizeAgentName(name: string): string | null {
  const trimmed = name.trim().toLowerCase();
  return AGENT_NAME_PATTERN.test(trimmed) ? trimmed : null;
}

/** 校验 trace ID，非法时返回 null */
export function sanitizeTraceId(id: string): string | null {
  const trimmed = id.trim().toLowerCase();
  return TRACE_ID_PATTERN.test(trimmed) ? trimmed : null;
}

/** 构建 agent 路径段（已 encodeURIComponent） */
export function agentPathSegment(agentName: string): string {
  const safe = sanitizeAgentName(agentName);
  if (!safe) {
    throw new Error(
      "Agent 名称仅允许小写字母、数字和连字符，长度 1-64"
    );
  }
  return encodeURIComponent(safe);
}

/** 构建 trace 路径段（已 encodeURIComponent） */
export function tracePathSegment(traceId: string): string {
  const safe = sanitizeTraceId(traceId);
  if (!safe) {
    throw new Error("Trace ID 必须为 32 位十六进制字符串");
  }
  return encodeURIComponent(safe);
}

/** ISO 时间窗口查询参数 */
export function timeWindowParams(from: Date, to: Date): string {
  return `from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`;
}
