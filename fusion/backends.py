"""Backend client — single AsyncOpenAI pointing at HiveMind :8400."""

import asyncio
import time
from typing import Any

from openai import AsyncOpenAI

from config import get_hivemind_endpoint

# Single shared client — HiveMind handles all auth/routing per model slug
_client: AsyncOpenAI | None = None


def get_client() -> AsyncOpenAI:
    global _client
    if _client is None:
        _client = AsyncOpenAI(
            base_url=get_hivemind_endpoint(),
            api_key="hivemind-fusion",  # HiveMind consumer auth, not provider auth
        )
    return _client


async def call_model(
    model: str,
    messages: list[dict[str, str]],
    *,
    timeout: float = 60.0,
    max_tokens: int | None = None,
    system: str | None = None,
    params: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Call a single model through HiveMind. Returns {content, tokens, latency_ms, error?}.

    system: if provided, prepended as a {"role":"system"} message.
    params: per-family sampling params (temperature, top_p, reasoning_effort, etc.)
            merged into the request body — gateway passes them through.
    """
    client = get_client()
    start = time.time()
    try:
        full_messages = list(messages)
        if system:
            full_messages = [{"role": "system", "content": system}] + full_messages
        kwargs: dict[str, Any] = {
            "model": model,
            "messages": full_messages,
        }
        if max_tokens:
            kwargs["max_tokens"] = max_tokens
        # ponytail: pass non-standard params via extra_body — the OpenAI SDK rejects
        # unknown top-level kwargs (repetition_penalty, effort), but extra_body merges
        # into the request JSON for the gateway to route per-backend.
        # upgrade: map known params to SDK-native fields when the SDK adds support
        if params:
            kwargs["extra_body"] = dict(params)

        resp = await asyncio.wait_for(
            client.chat.completions.create(**kwargs),
            timeout=timeout,
        )
        elapsed_ms = int((time.time() - start) * 1000)
        choice = resp.choices[0]
        # Reasoning models (GLM, Qwen3, DeepSeek) may emit reasoning_content
        # while leaving content empty. Fall back so the panelist's answer isn't lost.
        content = choice.message.content or ""
        if not content:
            content = getattr(choice.message, "reasoning_content", None) or ""
        return {
            "model": model,
            "content": content,
            "tokens": resp.usage.total_tokens if resp.usage else 0,
            "latency_ms": elapsed_ms,
        }
    except Exception as e:
        elapsed_ms = int((time.time() - start) * 1000)
        return {
            "model": model,
            "content": "",
            "tokens": 0,
            "latency_ms": elapsed_ms,
            "error": str(e)[:300],
        }


async def stream_model(
    model: str,
    messages: list[dict[str, str]],
    *,
    timeout: float = 60.0,
    system: str | None = None,
    params: dict[str, Any] | None = None,
):
    """Stream a model's output through HiveMind. Yields text deltas.

    On error, yields a single bracketed error marker so the caller doesn't crash.
    """
    client = get_client()
    full_messages = list(messages)
    if system:
        full_messages = [{"role": "system", "content": system}] + full_messages
    create_kwargs: dict[str, Any] = {
        "model": model,
        "messages": full_messages,
        "stream": True,
    }
    if params:
        create_kwargs["extra_body"] = dict(params)
    try:
        stream = await asyncio.wait_for(
            client.chat.completions.create(**create_kwargs),
            timeout=timeout,
        )
        async for chunk in stream:
            if chunk.choices and chunk.choices[0].delta.content:
                yield chunk.choices[0].delta.content
    except Exception as e:
        yield f"[stream error: {str(e)[:200]}]"
