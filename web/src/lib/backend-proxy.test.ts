/**
 * 轻量断言测试（无 jest 依赖，由 node 直接运行）。
 * 运行：npx tsx src/lib/backend-proxy.test.ts
 * 或：node --experimental-strip-types（Node 22+）
 */
import {
  parseProxyPath,
  resolveUpstream,
  ALLOWED_API_PREFIXES,
} from "./backend-proxy";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}

// parseProxyPath
assert(parseProxyPath(["cost", "total"])?.kind === "api", "cost/total api");
assert(parseProxyPath(["healthz"])?.kind === "health", "healthz");
assert(parseProxyPath(["readyz"])?.kind === "health", "readyz");
assert(parseProxyPath(["../etc"]) === null, "reject ..");
assert(parseProxyPath(["secret"]) === null, "reject unknown prefix");
assert(parseProxyPath(["traces", "abc"])?.path === "traces/abc", "nested path");

// resolveUpstream
process.env.BACKEND_API_BASE = "http://backend:8080";
const up = resolveUpstream(["eval", "agents", "a", "scores"], "?days=7");
assert(up !== null, "upstream not null");
assert(
  up!.url === "http://backend:8080/api/v1/eval/agents/a/scores?days=7",
  `url=${up!.url}`
);
assert(up!.injectAuth === true, "inject auth for api");

const hz = resolveUpstream(["healthz"], "");
assert(hz !== null && hz.url === "http://backend:8080/healthz", "health url");
assert(hz!.injectAuth === false, "no auth for healthz");

assert(ALLOWED_API_PREFIXES.includes("harness"), "harness allowed");

console.log("backend-proxy tests passed");
