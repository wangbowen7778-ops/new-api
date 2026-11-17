#!/usr/bin/env python3
"""TTFT / TPM probe for a large cached prompt.

Builds one fixed prefix, warms the upstream cache with it, then fires waves of
concurrent streaming requests that reuse the same prefix. Reports per-request
TTFT, cached tokens and the achieved TPM, so a TTFT regression under load can be
told apart from a cold-cache or a gateway problem.

Only the standard library is used.

Example:
  python ttft_tpm_probe.py --base https://your-gateway/v1 --key sk-xxx \
      --model kimi-k3 --prefix-tokens 60000 --concurrency 20 --rounds 6
"""

import argparse
import json
import random
import statistics
import sys
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

ZH_WORDS = (
    "网关 缓存 命中 预填 推理 上下文 令牌 延迟 吞吐 调度 队列 并发 上游 渠道 "
    "模型 请求 响应 带宽 集群 分片 副本 熔断 限流 配额 账单 计费 指纹 会话 "
    "文档 段落 摘要 索引 检索 向量 语料 标注 清洗 采样 温度 概率 分布 采样 "
    "观测 指标 采样 追踪 链路上 游服 务器 客户端 传输 协议 编码 解码 压缩 "
).split()

EN_WORDS = (
    "gateway cache prefix prefill inference context token latency throughput "
    "schedule queue concurrency upstream channel model request response "
    "bandwidth cluster shard replica breaker limit quota billing fingerprint "
    "session document paragraph summary index retrieve vector corpus label "
    "clean sample temperature probability distribution observe metric trace "
).split()


def build_prefix(style, approx_tokens, seed):
    """Deterministic pseudo-random text, roughly `approx_tokens` tokens.

    A Chinese character is about one token and a two-character word therefore
    about two, so the zh branch needs half as many words as the en branch.
    The real size is reported back by the API as prompt_tokens.
    """
    rnd = random.Random(seed)
    if style == "zh":
        return "".join(rnd.choice(ZH_WORDS) for _ in range(approx_tokens // 2))
    return " ".join(rnd.choice(EN_WORDS) for _ in range(approx_tokens))


def cached_tokens_from_usage(usage):
    details = usage.get("prompt_tokens_details")
    if isinstance(details, dict) and details.get("cached_tokens") is not None:
        return details["cached_tokens"]
    for key in ("cached_tokens", "prompt_cache_hit_tokens"):
        if usage.get(key) is not None:
            return usage[key]
    return None


def stream_once(base, key, model, messages, max_tokens, timeout):
    """One streaming request. Returns dict with ttft/total/usage/error."""
    body = json.dumps(
        {
            "model": model,
            "messages": messages,
            "stream": True,
            "stream_options": {"include_usage": True},
            "max_tokens": max_tokens,
        }
    ).encode("utf-8")

    request = urllib.request.Request(
        base.rstrip("/") + "/chat/completions",
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": "Bearer " + key,
            # Some upstreams sit behind a WAF that rejects Python's default
            # "Python-urllib/x.y" user agent with 403.
            "User-Agent": "curl/8.7.1",
            "Accept": "text/event-stream",
        },
        method="POST",
    )

    started = time.perf_counter()
    first = None
    usage = None
    error = None

    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            for raw in response:
                line = raw.decode("utf-8", "ignore").strip()
                if not line.startswith("data:"):
                    continue
                payload = line[5:].strip()
                if not payload or payload == "[DONE]":
                    if payload == "[DONE]":
                        break
                    continue
                try:
                    chunk = json.loads(payload)
                except json.JSONDecodeError:
                    continue
                if chunk.get("usage"):
                    usage = chunk["usage"]
                if first is None:
                    choices = chunk.get("choices") or []
                    delta = choices[0].get("delta") if choices else None
                    if delta and (delta.get("content") or delta.get("reasoning_content")):
                        first = time.perf_counter()
    except Exception as exc:  # noqa: BLE001 - probe reports whatever happened
        error = f"{type(exc).__name__}: {str(exc)[:160]}"

    finished = time.perf_counter()
    usage = usage or {}
    return {
        "ttft": None if first is None else first - started,
        "total": finished - started,
        "prompt_tokens": usage.get("prompt_tokens"),
        "cached_tokens": cached_tokens_from_usage(usage),
        "error": error,
    }


def percentile(values, fraction):
    if not values:
        return None
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, int(round(fraction * (len(ordered) - 1)))))
    return ordered[index]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True, help="e.g. https://host/v1")
    parser.add_argument("--key", required=True, help="API key")
    parser.add_argument("--model", required=True)
    parser.add_argument(
        "--prefix-tokens",
        type=int,
        default=60000,
        help="approximate prefix size in tokens (default 60000)",
    )
    parser.add_argument(
        "--style",
        choices=["zh", "en"],
        default="zh",
        help="prefix language; zh is closer to a document-sized prompt",
    )
    parser.add_argument("--concurrency", type=int, default=20)
    parser.add_argument("--rounds", type=int, default=6)
    parser.add_argument("--max-tokens", type=int, default=16)
    parser.add_argument("--timeout", type=int, default=900, help="socket timeout (s)")
    parser.add_argument(
        "--seed",
        type=int,
        default=None,
        help="prefix seed; omit for a fresh prefix on every run",
    )
    args = parser.parse_args()

    seed = args.seed if args.seed is not None else int(time.time())
    prefix = build_prefix(args.style, args.prefix_tokens, seed)
    print(f"prefix seed={seed} chars={len(prefix)} style={args.style}")

    base_messages = [{"role": "user", "content": prefix + "\n\nReply with OK only."}]

    print("\n== warm up (builds the upstream cache) ==")
    for attempt in range(2):
        warm = stream_once(
            args.base, args.key, args.model, base_messages, args.max_tokens, args.timeout
        )
        if warm["error"]:
            print(f"  warm {attempt + 1}: ERROR {warm['error']}")
            if attempt == 1:
                return 1
            continue
        print(
            f"  warm {attempt + 1}: prompt={warm['prompt_tokens']} "
            f"cached={warm['cached_tokens']} ttft={warm['ttft']:.1f}s"
        )
    if not (warm["cached_tokens"] or 0):
        print("  [!] the second warm-up still shows cached=0: the prefix is not being cached")
        print("      (gateway/downstream may not report cached_tokens, or the prefix is too short)")

    print(f"\n== load: {args.rounds} rounds x {args.concurrency} concurrent ==")
    started = time.perf_counter()
    results = []

    for round_index in range(args.rounds):
        messages_batch = []
        for slot in range(args.concurrency):
            # Same prefix for everyone (that is what the cache keys on), unique
            # tail so the requests are not byte-identical.
            messages_batch.append(
                [
                    {
                        "role": "user",
                        "content": prefix
                        + f"\n\n[probe {round_index}-{slot}] Reply with OK only.",
                    }
                ]
            )
        with ThreadPoolExecutor(max_workers=args.concurrency) as pool:
            futures = [
                pool.submit(
                    stream_once,
                    args.base,
                    args.key,
                    args.model,
                    messages,
                    args.max_tokens,
                    args.timeout,
                )
                for messages in messages_batch
            ]
            batch = [future.result() for future in futures]
        results.extend(batch)

        ttfts = [r["ttft"] for r in batch if r["ttft"] is not None]
        cached = [r["cached_tokens"] or 0 for r in batch]
        errors = [r for r in batch if r["error"]]
        print(
            f"  round {round_index + 1}: "
            f"ttft p50={percentile(ttfts, 0.5) or 0:.1f}s "
            f"max={max(ttfts) if ttfts else 0:.1f}s "
            f"cached/min={min(cached) if cached else 0} "
            f"errors={len(errors)}"
        )
        for failed in errors:
            print(f"    ERROR {failed['error']}")

    elapsed = time.perf_counter() - started
    ttfts = [r["ttft"] for r in results if r["ttft"] is not None]
    prompt_tokens = sum(r["prompt_tokens"] or 0 for r in results)
    cached_tokens = sum(r["cached_tokens"] or 0 for r in results)

    print("\n== summary ==")
    if ttfts:
        print(f"  requests        : {len(results)} ok / {len(results) - len(ttfts)} failed")
        print(f"  TTFT p50        : {statistics.median(ttfts):.1f}s")
        print(f"  TTFT p95        : {percentile(ttfts, 0.95):.1f}s")
        print(f"  TTFT max        : {max(ttfts):.1f}s")
        print(f"  TTFT min        : {min(ttfts):.1f}s")
    else:
        print("  no successful first token; check --base/--key/--model")
    print(f"  prompt tokens   : {prompt_tokens}")
    if prompt_tokens:
        ratio = 100.0 * cached_tokens / prompt_tokens
        print(f"  cached tokens   : {cached_tokens} ({ratio:.1f}%)")
    print(f"  elapsed         : {elapsed:.1f}s")
    if elapsed > 0:
        print(f"  achieved TPM    : {prompt_tokens / elapsed * 60:,.0f} tokens/min")
    return 0


if __name__ == "__main__":
    sys.exit(main())
