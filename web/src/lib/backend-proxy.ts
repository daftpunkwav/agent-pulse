/**
 * 服务端 BFF 代理：浏览器只访问同源 /api/backend/*，
 * 由 Next 服务端注入 X-AgentPulse-Key，密钥绝不下发到客户端。
 */

/** 允许转发到后端 /api/v1 的路径首段白名单 */
export const ALLOWED_API_PREFIXES = [
  "traces",
  "cost",
  "eval",
  "clusters",
  "harness",
  "abtests",
] as const;

/** 无鉴权探活路径（后端根路径，非 /api/v1） */
export const HEALTH_PATHS = new Set(["healthz", "readyz"]);

const PATH_SEGMENT = /^[a-zA-Z0-9._~\-]+$/;

export function getBackendBase(): string {
  const base = process.env.BACKEND_API_BASE || "http://localhost:8080";
  if (process.env.NODE_ENV === "production" && !process.env.BACKEND_API_BASE) {
    throw new Error(
      "生产环境必须设置 BACKEND_API_BASE（服务端专用，勿使用 NEXT_PUBLIC_）"
    );
  }
  return base.replace(/\/$/, "");
}

/** 服务端 API Key；未配置时不注入（兼容本地 auth.enabled=false） */
export function getBackendApiKey(): string | undefined {
  const key =
    process.env.BACKEND_API_KEY ||
    process.env.AGENTPULSE_API_KEY ||
    process.env.AGENTPULSE_AUTH_API_KEYS?.split(",")[0]?.trim();
  return key || undefined;
}

/**
 * 校验并拆分客户端 path 段。
 * 返回 null 表示非法路径。
 */
export function parseProxyPath(
  segments: string[]
): { kind: "health" | "api"; path: string } | null {
  if (!segments.length || segments.length > 32) {
    return null;
  }
  for (const seg of segments) {
    if (!seg || seg === ".." || seg === "." || !PATH_SEGMENT.test(seg)) {
      return null;
    }
  }

  const head = segments[0]!;
  if (HEALTH_PATHS.has(head) && segments.length === 1) {
    return { kind: "health", path: head };
  }

  if (!(ALLOWED_API_PREFIXES as readonly string[]).includes(head)) {
    return null;
  }

  return { kind: "api", path: segments.join("/") };
}

/** 构造上游 URL 与是否注入鉴权头 */
export function resolveUpstream(
  segments: string[],
  search: string
): { url: string; injectAuth: boolean } | null {
  const parsed = parseProxyPath(segments);
  if (!parsed) {
    return null;
  }
  const base = getBackendBase();
  const qs = search.startsWith("?") || search === "" ? search : `?${search}`;
  if (parsed.kind === "health") {
    return { url: `${base}/${parsed.path}${qs}`, injectAuth: false };
  }
  return { url: `${base}/api/v1/${parsed.path}${qs}`, injectAuth: true };
}
