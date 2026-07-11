"""截取 AgentPulse dashboard 截图给用户预览效果"""

import asyncio
import os
import time
from playwright.async_api import async_playwright

PAGES = [
    ("overview", "http://localhost:3202/"),
    ("cost",     "http://localhost:3202/cost"),
    ("traces",   "http://localhost:3202/traces"),
    ("eval",     "http://localhost:3202/eval"),
    ("clusters", "http://localhost:3202/clusters"),
]

OUT = r"D:\daftpunkwav\04-MyProjects\AgentPulse\docs\screenshots"
os.makedirs(OUT, exist_ok=True)


async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        ctx = await browser.new_context(viewport={"width": 1440, "height": 900})
        page = await ctx.new_page()

        for name, url in PAGES:
            print(f"--- {name} {url}")
            try:
                await page.goto(url, wait_until="domcontentloaded", timeout=30000)
                # 等 Next.js dev 模式下 CSS/JS chunks 注入完成
                try:
                    await page.wait_for_function("document.fonts.ready.then(() => true)", timeout=10000)
                except Exception:
                    pass
                # 等网络空闲 + 加载 spinner 消失
                try:
                    await page.wait_for_load_state("networkidle", timeout=15000)
                except Exception:
                    pass
                await page.wait_for_timeout(6000)
                out = os.path.join(OUT, f"{name}.png")
                await page.screenshot(path=out, full_page=True)
                title = await page.title()
                visible_text = await page.evaluate("document.body.innerText.slice(0, 400)")
                print(f"   title: {title!r}")
                print(f"   body[:400]: {visible_text!r}")
                print(f"   saved: {out}")
            except Exception as e:
                print(f"   FAILED: {e}")
            print()

        await browser.close()


asyncio.run(main())
