"""Translate OpenAI chat completion requests to `claude -p` headless CLI invocations."""

import json
import subprocess
import time
import uuid
from typing import Any

CLAUDE_BIN = "/home/pook/.npm-global/bin/claude"

# Map OpenAI-style slugs → Claude CLI model identifiers.
# Pass-through if not found — let the CLI validate.
MODEL_MAP: dict[str, str] = {
    "claude-opus-4-8": "claude-opus-4-8",
    "claude-sonnet-4-6": "claude-sonnet-4-6",
    "claude-haiku-4-5": "claude-haiku-4-5",
}


class CLIError(Exception):
    pass


def _flatten_messages(messages: list[dict[str, Any]]) -> str:
    """Flatten OpenAI messages array into a single prompt string for `claude -p`."""
    parts: list[str] = []
    for msg in messages:
        role = msg.get("role", "user")
        content = msg.get("content", "")
        if isinstance(content, list):
            # Handle multimodal content arrays — extract text parts
            content = "\n".join(
                block.get("text", "") for block in content if isinstance(block, dict) and block.get("type") == "text"
            )
        if role == "system":
            parts.append(f"[System]\n{content}")
        elif role == "user":
            parts.append(f"[User]\n{content}")
        elif role == "assistant":
            parts.append(f"[Assistant]\n{content}")
    return "\n\n".join(parts)


def call_claude(
    model: str,
    messages: list[dict[str, Any]],
    *,
    max_tokens: int | None = None,
    temperature: float | None = None,
    timeout: int = 120,
) -> dict[str, Any]:
    """Invoke `claude -p` headless, return OpenAI-shaped response dict."""
    cli_model = MODEL_MAP.get(model, model)
    prompt = _flatten_messages(messages)

    cmd = [
        CLAUDE_BIN,
        "-p", prompt,
        "--model", cli_model,
        "--output-format", "json",
        "--dangerously-skip-permissions",
    ]
    # ponytail: --max-tokens not a claude CLI flag — skipped. upgrade: use system prompt to constrain length
    start = time.time()
    try:
        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
            env={"HOME": "/home/pook", "PATH": "/home/pook/.npm-global/bin:/usr/local/bin:/usr/bin:/bin"},
        )
    except subprocess.TimeoutExpired:
        raise CLIError(f"claude CLI timed out after {timeout}s")
    except FileNotFoundError:
        raise CLIError(f"claude binary not found at {CLAUDE_BIN}")

    elapsed_ms = int((time.time() - start) * 1000)

    if proc.returncode != 0:
        stderr_tail = proc.stderr[-500:] if proc.stderr else ""
        raise CLIError(f"claude CLI exited {proc.returncode}: {stderr_tail}")

    try:
        result = json.loads(proc.stdout)
    except json.JSONDecodeError:
        raise CLIError(f"claude CLI returned non-JSON output: {proc.stdout[:200]}")

    if result.get("is_error"):
        raise CLIError(f"claude CLI error: {result.get('result', 'unknown')}")

    content = result.get("result", "")
    usage = result.get("usage", {})
    prompt_tokens = usage.get("input_tokens", 0)
    completion_tokens = usage.get("output_tokens", 0)

    return {
        "id": f"chatcmpl-{uuid.uuid4().hex[:12]}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": model,
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": content},
                "finish_reason": "stop",
            }
        ],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "total_tokens": prompt_tokens + completion_tokens,
        },
        "_meta": {
            "elapsed_ms": elapsed_ms,
            "cost_usd": result.get("total_cost_usd", 0),
            "session_id": result.get("session_id", ""),
        },
    }


async def call_claude_streaming(
    model: str,
    messages: list[dict[str, Any]],
    *,
    max_tokens: int | None = None,
    timeout: int = 120,
):
    """Invoke `claude -p` with stream-json output, yield OpenAI SSE chunks."""
    import asyncio

    cli_model = MODEL_MAP.get(model, model)
    prompt = _flatten_messages(messages)

    cmd = [
        CLAUDE_BIN,
        "-p", prompt,
        "--model", cli_model,
        "--output-format", "stream-json",
        "--dangerously-skip-permissions",
        "--verbose",
    ]
    # ponytail: --max-tokens not a claude CLI flag — skipped in streaming mode too
    env = {"HOME": "/home/pook", "PATH": "/home/pook/.npm-global/bin:/usr/local/bin:/usr/bin:/bin"}
    completion_id = f"chatcmpl-{uuid.uuid4().hex[:12]}"
    created = int(time.time())

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        env=env,
    )

    try:
        assert proc.stdout is not None
        async for line in proc.stdout:
            line = line.strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue

            # Claude stream-json emits various event types; extract text content
            text = _extract_stream_text(event)
            if text:
                chunk = {
                    "id": completion_id,
                    "object": "chat.completion.chunk",
                    "created": created,
                    "model": model,
                    "choices": [
                        {
                            "index": 0,
                            "delta": {"content": text},
                            "finish_reason": None,
                        }
                    ],
                }
                yield f"data: {json.dumps(chunk)}\n\n"

        # Final chunk with finish_reason
        final = {
            "id": completion_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "delta": {},
                    "finish_reason": "stop",
                }
            ],
        }
        yield f"data: {json.dumps(final)}\n\n"
        yield "data: [DONE]\n\n"

    finally:
        if proc.returncode is None:
            proc.kill()
        await proc.wait()


def _extract_stream_text(event: dict[str, Any]) -> str | None:
    """Extract text content from a Claude stream-json event."""
    # Result event (final)
    if event.get("type") == "result":
        return None  # handled by finish logic
    # Assistant message events
    if event.get("type") == "assistant":
        message = event.get("message", {})
        for block in message.get("content", []):
            if isinstance(block, dict) and block.get("type") == "text":
                return block.get("text", "")
    # Content block delta events
    if event.get("type") == "content_block_delta":
        delta = event.get("delta", {})
        if delta.get("type") == "text_delta":
            return delta.get("text", "")
    # Stream event with text
    if event.get("type") == "text":
        return event.get("text", "")
    return None
