#!/usr/bin/env python3
"""长时间漂移探针：观察首 token 延迟是否随时间变差。

每隔 --interval 秒发一批并发请求，把每次的 ttft / cached tokens / request id
写进 CSV。挂一两个小时再用 Excel 画一下，就能看出「越到后面越慢」是真漂移，
还是上游的偶发抖动。

只用标准库，无第三方依赖。

    python tools/ttft_drift_probe.py \
        --base http://39.105.13.175:8888/v1 \
        --key sk-xxx \
        --interval 30 --duration 3600

输出：logs/ttft_drift_<时间戳>.csv
"""

import argparse
import csv
import json
import os
import statistics
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

FILLER = (
    "网关 缓存 命中 预填 推理 上下文 令牌 延迟 吞吐 调度 队列 并发 上游 渠道 模型 请求 "
    "响应 带宽 集群 分片 副本 熔断 限流 配额 账单 计费 指纹 会话 文档 段落 摘要 索引"
).split()

# 该上游的 WAF 会拒绝 Python-urllib 的默认 UA，这里伪装成 curl。
UA = "curl/8.7.1"


def build_prompt(prefix_tokens, seed):
    if prefix_tokens <= 0:
        return "Reply with OK only."
    import random

    rnd = random.Random(seed)
    # 中文词平均约 1 token/字，取 2 字/词的近似
    body = "".join(rnd.choice(FILLER) for _ in range(prefix_tokens // 2))
    return body + "\n\nReply with OK only."


def call(base, key, model, messages, max_tokens, timeout):
    payload = {
        "model": model,
        "messages": messages,
        "stream": True,
        "stream_options": {"include_usage": True},
        "max_tokens": max_tokens,
    }
    req = urllib.request.Request(
        base.rstrip("/") + "/chat/completions",
        data=json.dumps(payload).encode(),
        headers={
            "User-Agent": UA,
            "Content-Type": "application/json",
            "Authorization": "Bearer " + key,
            "Accept": "text/event-stream",
        },
        method="POST",
    )

    started = time.perf_counter()
    ttft = None
    usage = None
    rid = ""
    status = 200
    error = ""
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            rid = resp.headers.get("X-Oneapi-Request-Id", "")
            for raw in resp:
                line = raw.decode("utf-8", "ignore").strip()
                if not line.startswith("data:"):
                    continue
                data = line[5:].strip()
                if data == "[DONE]":
                    break
                try:
                    chunk = json.loads(data)
                except Exception:
                    continue
                if chunk.get("usage"):
                    usage = chunk["usage"]
                if ttft is None:
                    choices = chunk.get("choices") or []
                    delta = choices[0].get("delta") if choices else None
                    if delta and (delta.get("content") or delta.get("reasoning_content")):
                        ttft = time.perf_counter() - started
    except urllib.error.HTTPError as exc:
        status = exc.code
        error = (exc.read() or b"").decode("utf-8", "ignore")[:120].replace("\n", " ")
    except Exception as exc:
        status = -1
        error = f"{type(exc).__name__}: {exc}"[:120]

    usage = usage or {}
    details = usage.get("prompt_tokens_details") or {}
    cached = details.get("cached_tokens")
    if cached is None:
        cached = usage.get("cached_tokens")

    return {
        "ttft": ttft,
        "total": time.perf_counter() - started,
        "prompt": usage.get("prompt_tokens"),
        "cached": cached,
        "rid": rid,
        "status": status,
        "error": error,
    }


def main():
    parser = argparse.ArgumentParser(description="首 token 长时间漂移探针")
    parser.add_argument("--base", required=True, help="例如 http://39.105.13.175:8888/v1")
    parser.add_argument("--key", required=True)
    parser.add_argument("--model", default="kimi-k3")
    parser.add_argument("--interval", type=int, default=30, help="每轮间隔秒数")
    parser.add_argument("--duration", type=int, default=3600, help="总时长秒数")
    parser.add_argument("--concurrency", type=int, default=4, help="每轮并发数")
    parser.add_argument("--prefix-tokens", type=int, default=0, help="0 表示小请求")
    parser.add_argument("--max-tokens", type=int, default=2)
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--out", default="", help="CSV 输出路径")
    args = parser.parse_args()

    os.makedirs("logs", exist_ok=True)
    out_path = args.out or os.path.join(
        "logs", time.strftime("ttft_drift_%Y%m%d_%H%M%S.csv")
    )

    messages = [{"role": "user", "content": build_prompt(args.prefix_tokens, 20261020)}]

    fields = [
        "elapsed_s", "batch", "seq", "ttft_s", "total_s",
        "prompt_tokens", "cached_tokens", "status", "request_id", "error",
    ]

    started = time.time()
    batch = 0
    print(f"写入 {out_path}")
    print(f"每 {args.interval}s 一轮 x {args.concurrency} 并发，共 {args.duration}s")
    print(f"{'elapsed':>8} {'p50':>7} {'min':>7} {'max':>7} {'cached':>10} {'数量':>5}")

    with open(out_path, "w", newline="", encoding="utf-8-sig") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        while time.time() - started < args.duration:
            batch += 1
            with ThreadPoolExecutor(max_workers=args.concurrency) as pool:
                results = list(
                    pool.map(
                        lambda _: call(
                            args.base, args.key, args.model,
                            messages, args.max_tokens, args.timeout,
                        ),
                        range(args.concurrency),
                    )
                )
            elapsed = time.time() - started
            for seq, res in enumerate(results, 1):
                writer.writerow({
                    "elapsed_s": round(elapsed, 1),
                    "batch": batch,
                    "seq": seq,
                    "ttft_s": "" if res["ttft"] is None else round(res["ttft"], 3),
                    "total_s": round(res["total"], 3),
                    "prompt_tokens": res["prompt"],
                    "cached_tokens": res["cached"],
                    "status": res["status"],
                    "request_id": res["rid"],
                    "error": res["error"],
                })
            fh.flush()

            good = [r["ttft"] for r in results if r["ttft"]]
            cached = [r["cached"] for r in results if r["cached"] is not None]
            if good:
                print(
                    f"{elapsed:8.0f} {statistics.median(good):7.2f} "
                    f"{min(good):7.2f} {max(good):7.2f} "
                    f"{(statistics.median(cached) if cached else 0):10.0f} {len(good):5d}"
                )
            else:
                codes = {}
                for res in results:
                    codes[res["status"]] = codes.get(res["status"], 0) + 1
                print(f"{elapsed:8.0f} 全部失败 {codes}")

            sleep_for = args.interval - (time.time() - started - elapsed)
            if sleep_for > 0:
                time.sleep(sleep_for)

    print(f"\n完成，共 {batch} 轮。CSV: {out_path}")
    print("判读：p50 逐段抬升 = 真漂移；只有零星尖刺 = 上游偶发抖动。")


if __name__ == "__main__":
    main()
