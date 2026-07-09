/** 统一 API 请求封装 */

import { z } from "zod";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** SWR 通用 fetcher，校验 HTTP 状态 */
export async function swrFetcher(url: string): Promise<unknown> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new ApiError(response.status, `请求失败 (${response.status})`);
  }
  return response.json();
}

/** 带 Zod 校验的 SWR fetcher 工厂 */
export function createSchemaFetcher<T>(schema: z.ZodType<T>) {
  return async (url: string): Promise<T> => {
    const data = await swrFetcher(url);
    return schema.parse(data);
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
  const init: RequestInit = {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  };
  if (options?.body !== undefined) {
    init.body = JSON.stringify(options.body);
  }
  const response = await fetch(url, init);

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
}
