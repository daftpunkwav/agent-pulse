/** 统一 API 请求封装 */

import { z } from "zod";

/** API 基础路径（通过 NEXT_PUBLIC_API_BASE 环境变量配置） */
const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/backend";

/** 单次请求超时（毫秒） */
const FETCH_TIMEOUT_MS = 30_000;

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** 创建带超时的 AbortController */
function withTimeout(signal?: AbortSignal): { controller: AbortController; timeoutId: ReturnType<typeof setTimeout> } {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  // 如果外部信号已触发，立即取消超时
  if (signal?.aborted) {
    clearTimeout(timeoutId);
    controller.abort();
  }
  return { controller, timeoutId };
}

/** 构建完整 URL */
function resolveUrl(path: string): string {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  if (!path.startsWith("/")) {
    path = "/" + path;
  }
  return API_BASE + path;
}

/** SWR 通用 fetcher，校验 HTTP 状态 */
export async function swrFetcher(url: string): Promise<unknown> {
  const { controller, timeoutId } = withTimeout();
  try {
    const response = await fetch(resolveUrl(url), { signal: controller.signal });
    if (!response.ok) {
      throw new ApiError(response.status, `请求失败 (${response.status})`);
    }
    return response.json();
  } finally {
    clearTimeout(timeoutId);
  }
}

/** 带 Zod 校验的 SWR fetcher 工厂 */
export function createSchemaFetcher<T>(schema: z.ZodType<T>) {
  return async (url: string): Promise<T> => {
    const data = await swrFetcher(url);
    const parsed = schema.safeParse(data);
    if (!parsed.success) {
      throw new Error(`响应格式不匹配: ${parsed.error.message}`);
    }
    return parsed.data;
  };
}

/** 带 Zod 校验的 JSON GET */
export async function fetchJson<T>(
  url: string,
  schema: z.ZodType<T>
): Promise<T> {
  return createSchemaFetcher(schema)(url);
}

/** 统一 POST JSON 请求 */
export async function postJson<T = void>(
  url: string,
  options?: {
    body?: unknown;
    schema?: z.ZodType<T>;
  }
): Promise<T> {
  const { controller, timeoutId } = withTimeout();
  try {
    const init: RequestInit = {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      signal: controller.signal,
    };
    if (options?.body !== undefined) {
      init.body = JSON.stringify(options.body);
    }
    const response = await fetch(resolveUrl(url), init);

  if (!response.ok) {
    let detail = "";
    try {
      const errBody = (await response.json()) as {
        message?: string;
        error?: string;
        request_id?: string;
      };
      detail = errBody.message ?? errBody.error ?? "";
      if (errBody.request_id) {
        detail = detail ? `${detail} (request_id: ${errBody.request_id})` : errBody.request_id;
      }
    } catch {
      // 忽略解析错误
    }
    throw new ApiError(
      response.status,
      detail || `POST 请求失败 (${response.status})`
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json();
  if (options?.schema) {
    return options.schema.parse(json);
  }
  return json as T;
} finally {
  clearTimeout(timeoutId);
}
}
