"""demo_agent.py - 上传一组 demo trace 到 AgentPulse。

跑完后端可以在 dashboard 上看到数据。
"""

import os
import random
import time

from agentpulse import init, observe, session, shutdown, trace

init(
    api_key=os.environ.get("AGENTPULSE_API_KEY", "ap-demo-local-key-0001"),
    endpoint=os.environ.get("AGENTPULSE_ENDPOINT", "http://localhost:8080"),
    service_name="demo-app",
    environment="development",
)

MODELS = ["gpt-4o-mini", "gpt-4o", "claude-sonnet-4", "deepseek-chat"]
AGENTS = ["research-agent", "code-agent", "qa-agent"]
USERS = ["user-001", "user-002", "user-003", "user-004"]
TOOLS = ["web_search", "code_runner", "file_reader", "sql_query"]


@observe(agent_name="research-agent", span_type="agent")
def research(query: str) -> str:
    with trace("llm_call", span_type="llm") as t:
        t.set_input(query)
        time.sleep(0.05)
        out = f"关于 '{query}' 的研究回答"
        t.set_output(out)
        model = random.choice(MODELS)
        t.set_llm(
            model=model,
            prompt_tokens=random.randint(100, 800),
            completion_tokens=random.randint(50, 400),
            cost_usd=round(random.uniform(0.0001, 0.005), 4),
        )
    return out


@observe(agent_name="code-agent", span_type="agent")
def write_code(task: str) -> str:
    with trace("llm_call", span_type="llm") as t:
        t.set_input(task)
        time.sleep(0.04)
        out = f"def {task}(): pass"
        t.set_output(out)
        model = random.choice(MODELS)
        t.set_llm(
            model=model,
            prompt_tokens=random.randint(300, 1500),
            completion_tokens=random.randint(80, 600),
            cost_usd=round(random.uniform(0.001, 0.02), 4),
        )
    with trace("tool_exec", span_type="tool") as t:
        t.set_input({"tool": "code_runner", "code": out})
        time.sleep(0.05)
        t.set_output("ok")
        if random.random() < 0.2:
            t.set_attribute("ap.error", "syntax_error")
    return out


@observe(agent_name="qa-agent", span_type="agent")
def answer(question: str) -> str:
    tool = random.choice(TOOLS)
    with trace("tool_call", span_type="tool") as t:
        t.set_input({"tool": tool, "query": question})
        time.sleep(0.03)
        out = f"{tool} 返回 {random.randint(1, 10)} 条结果"
        t.set_output(out)
    with trace("llm_call", span_type="llm") as t:
        t.set_input(question + " " + out)
        time.sleep(0.04)
        answer_text = f"基于工具结果回答: {question[:20]}"
        t.set_output(answer_text)
        model = random.choice(MODELS)
        t.set_llm(
            model=model,
            prompt_tokens=random.randint(200, 1000),
            completion_tokens=random.randint(100, 300),
            cost_usd=round(random.uniform(0.0005, 0.008), 4),
        )
    return answer_text


def main():
    print("=== 上报 demo trace 到 AgentPulse ===\n")
    rng = random.Random(42)

    for sess_idx in range(15):
        session_id = f"session-demo-{sess_idx:03d}"
        user_id = rng.choice(USERS)
        agent = rng.choice(AGENTS)
        with session(
            user_id=user_id,
            session_id=session_id,
            agent_name=agent,
            metadata={"plan": rng.choice(["free", "pro"])},
        ):
            n_calls = rng.randint(2, 5)
            for _ in range(n_calls):
                try:
                    if agent == "research-agent":
                        research(f"query-{rng.randint(1, 100)}")
                    elif agent == "code-agent":
                        write_code(f"task_{rng.randint(1, 100)}")
                    else:
                        answer(f"q-{rng.randint(1, 100)}")
                except Exception as e:
                    pass
            time.sleep(0.02)

    print("\n关闭 SDK...")
    shutdown()
    print("完成。")


if __name__ == "__main__":
    main()
