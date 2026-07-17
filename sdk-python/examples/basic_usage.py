"""examples/basic_usage.py - SDK 基础使用示例。

运行方式：
1. 启动 AgentPulse 后端（docker compose up -d）
2. 启动 Go 后端（cd backend && go run cmd/server/main.go）
3. python examples/basic_usage.py
"""

import os
import time

from agentpulse import init, observe, session, shutdown, trace


# 1. 初始化（在应用启动时调用一次）
# 未传参时自动读取 AGENTPULSE_API_KEY / AGENTPULSE_ENDPOINT 等环境变量；
# 本地开发可显式传合法 ap- 前缀 key（与后端 auth.api_keys 对齐）。
init(
    api_key=os.environ.get("AGENTPULSE_API_KEY", "ap-demo-local-key-01"),
    endpoint=os.environ.get("AGENTPULSE_ENDPOINT", "http://localhost:8080"),
    service_name="example-app",
    environment="development",
)


# 2. 装饰器方式：自动追踪函数调用
@observe(agent_name="example-agent", span_type="agent")
def answer_question(question: str) -> str:
    """模拟 LLM 调用（实际场景中替换为真实 LLM 调用）。"""
    # 模拟 LLM 调用并记录 token 信息
    with trace("llm_call", span_type="llm") as t:
        t.set_input(question)
        # 模拟 LLM 处理时间
        time.sleep(0.1)
        result = f"这是关于 '{question}' 的示例回答。"
        t.set_output(result)
        # 记录 token 与成本（实际从 LLM 响应获取）
        t.set_llm(
            model="gpt-4o-mini",
            prompt_tokens=len(question) // 2,
            completion_tokens=len(result) // 2,
            cost_usd=0.0001,
        )
    return result


# 3. 工具调用追踪
@observe(agent_name="example-agent", span_type="tool")
def search_web(query: str) -> str:
    """模拟 web 搜索。"""
    time.sleep(0.05)
    return f"搜索结果: 找到 {len(query)} 个相关结果"


# 4. 主流程
def main():
    print("=== AgentPulse SDK 基础示例 ===\n")

    # 创建会话（设置用户上下文）
    with session(
        user_id="user-001",
        session_id="session-demo-001",
        agent_name="example-agent",
        metadata={"plan": "free", "version": "1.0"},
    ) as sess:
        print(f"会话 ID: {sess.session_id}")

        # 调用被装饰的函数
        answer = answer_question("什么是 Agent？")
        print(f"回答: {answer}\n")

        # 调用工具
        search_result = search_web("AgentPulse")
        print(f"搜索: {search_result}\n")

        # 直接使用 trace Context Manager
        with trace("custom_operation", span_type="agent") as t:
            t.set_input("自定义操作")
            time.sleep(0.02)
            t.set_output("完成")
            t.set_attribute("ap.custom.metric", 42)

    # 关闭客户端（刷新缓冲区）
    print("\n关闭 SDK...")
    shutdown()
    print("完成！请到 AgentPulse Dashboard 查看 Trace。")


if __name__ == "__main__":
    main()