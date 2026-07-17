/**
 * BFF：/api/backend/* → 后端 API / 探活
 *
 * - 浏览器永不接触 BACKEND_API_KEY
 * - 仅白名单前缀可代理，防止 SSRF
 */
import { NextRequest, NextResponse } from "next/server";
import {
  getBackendApiKey,
  resolveUpstream,
} from "@/lib/backend-proxy";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const HOP_BY_HOP = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailers",
  "transfer-encoding",
  "upgrade",
  "host",
  "content-length",
]);

async function proxy(
  req: NextRequest,
  context: { params: Promise<{ path: string[] }> }
): Promise<NextResponse> {
  const { path: segments } = await context.params;
  const upstream = resolveUpstream(segments ?? [], req.nextUrl.search);
  if (!upstream) {
    return NextResponse.json(
      { error: "bad_request", message: "path not allowed" },
      { status: 400 }
    );
  }

  const headers = new Headers();
  req.headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (HOP_BY_HOP.has(lower)) return;
    // 禁止客户端伪造上游鉴权
    if (lower === "x-agentpulse-key" || lower === "authorization") return;
    headers.set(key, value);
  });

  if (upstream.injectAuth) {
    const apiKey = getBackendApiKey();
    if (apiKey) {
      headers.set("X-AgentPulse-Key", apiKey);
    }
  }

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  let body: BodyInit | null = null;
  if (hasBody) {
    const buf = Buffer.from(await req.arrayBuffer());
    body = buf;
    if (buf.length > 0 && !headers.has("content-type")) {
      headers.set("content-type", "application/json");
    }
  }

  const init: RequestInit = {
    method: req.method,
    headers,
    body,
    redirect: "manual",
  };

  let resp: Response;
  try {
    resp = await fetch(upstream.url, init);
  } catch (err) {
    const message = err instanceof Error ? err.message : "upstream unreachable";
    return NextResponse.json(
      { error: "bad_gateway", message },
      { status: 502 }
    );
  }

  const outHeaders = new Headers();
  resp.headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (HOP_BY_HOP.has(lower)) return;
    outHeaders.set(key, value);
  });

  return new NextResponse(resp.body, {
    status: resp.status,
    statusText: resp.statusText,
    headers: outHeaders,
  });
}

export async function GET(
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> }
) {
  return proxy(req, ctx);
}

export async function POST(
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> }
) {
  return proxy(req, ctx);
}

export async function PUT(
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> }
) {
  return proxy(req, ctx);
}

export async function PATCH(
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> }
) {
  return proxy(req, ctx);
}

export async function DELETE(
  req: NextRequest,
  ctx: { params: Promise<{ path: string[] }> }
) {
  return proxy(req, ctx);
}
