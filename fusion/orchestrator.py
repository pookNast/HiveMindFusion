"""Fusion orchestrator — deliberate → fan-out → judge → synthesize."""

import asyncio
import json
import logging
import re
import time
import uuid
from typing import Any, AsyncGenerator

from backends import call_model, stream_model
from config import get_panel, get_panel_deliberator, load_prompt
from transforms import transform_for_model

log = logging.getLogger("fusion")

MAX_PANELIST_TOKENS = 3000  # truncate panelist responses before judge to avoid context overflow


def _min_quorum(panel: dict[str, Any]) -> int:
    """Minimum panelists that must answer for a real consensus.

    A panel *defined* with one panelist (e.g. improve-recon-local) legitimately needs only
    1; any multi-model panel needs >=2, else the "consensus" has the same blind spot as a
    single-model call — the exact failure mode the panel exists to cross-check. Mirrors the
    bash kernel's floor-of-2 (hivemind-panel.sh). Override per-tier with `min_quorum`.
    """
    n = len(panel.get("panelists", []))
    return int(panel.get("min_quorum", min(2, n)))


def _summary_log(
    tier: str,
    elapsed_ms: int,
    responses: list[dict[str, Any]],
    judge_model: str,
    judge_ok: bool,
    synth_model: str,
) -> None:
    """Emit one structured JSON log line per fusion request for observability."""
    entry = {
        "event": "fusion_complete",
        "tier": tier.replace("hivemind/fusion-", ""),
        "elapsed_ms": elapsed_ms,
        "panelists": [
            {
                "model": r["model"],
                "ok": "error" not in r,
                "tokens": r.get("tokens", 0),
                "latency_ms": r.get("latency_ms", 0),
            }
            for r in responses
        ],
        "succeeded": sum(1 for r in responses if "error" not in r),
        "total": len(responses),
        "judge": {"model": judge_model, "ok": judge_ok},
        "synth": {"model": synth_model},
    }
    log.info(json.dumps(entry))


async def deliberate(
    messages: list[dict[str, str]],
    deliberator_model: str,
    timeout_s: float = 30.0,
) -> dict[str, Any]:
    """Deliberate the raw user prompt into a canonical task_spec.

    Calls the deliberator model (default glm-5.1) to parse intent into JSON.
    Falls back to a minimal spec on any failure — fusion never blocks on deliberation errors.
    """
    raw_input = _extract_question(messages)
    prompt_template = load_prompt("deliberator")
    filled = prompt_template.replace("{input}", raw_input)

    try:
        result = await call_model(
            deliberator_model,
            [{"role": "user", "content": filled}],
            timeout=timeout_s,
            max_tokens=4096,
        )
        if "error" in result or not result.get("content"):
            log.warning("deliberator failed: %s — falling back to raw spec", result.get("error", "empty"))
            return _fallback_spec(raw_input)

        # Extract JSON (model may wrap in fences)
        raw = result["content"].strip()
        if raw.startswith("```"):
            lines = raw.split("\n")
            raw = "\n".join(lines[1:-1] if lines[-1].startswith("```") else lines[1:])
        # Find the first { ... last } — tolerate trailing prose
        start = raw.find("{")
        end = raw.rfind("}")
        if start != -1 and end != -1 and end > start:
            raw = raw[start:end + 1]

        task_spec = json.loads(raw)
        # Ensure required keys exist
        task_spec.setdefault("input", raw_input)
        log.info("deliberator OK (model=%s, tokens=%d)", deliberator_model, result["tokens"])
        return task_spec
    except (json.JSONDecodeError, Exception) as e:
        log.warning("deliberator parse error: %s — falling back to raw spec", str(e)[:200])
        return _fallback_spec(raw_input)


def _fallback_spec(raw_input: str) -> dict[str, Any]:
    """Minimal task_spec when deliberation fails — passes raw input through."""
    return {
        "role": "an expert assistant",
        "task": raw_input,
        "context": "",
        "constraints": [],
        "output_format": "Respond clearly and concisely.",
        "guidance_blocks": "",
        "input": raw_input,
    }


async def fan_out(
    panel: dict[str, Any],
    task_spec: dict[str, Any],
    timeout_s: float,
) -> list[dict[str, Any]]:
    """Call all panelists in parallel, each receiving the task in its native format.

    Per-model transform: renders system+user+params from task_spec via transform_for_model.
    Returns list of result dicts (may include errors).
    """
    async def call_panelist(model: str) -> dict[str, Any]:
        try:
            system, user, params = transform_for_model(task_spec, model)
        except Exception as e:
            return {
                "model": model, "content": "", "tokens": 0, "latency_ms": 0,
                "error": f"transform failed: {str(e)[:200]}",
            }
        return await call_model(
            model,
            [{"role": "user", "content": user}],
            timeout=timeout_s,
            system=system,
            params=params,
        )

    tasks = [call_panelist(model) for model in panel["panelists"]]
    results = await asyncio.gather(*tasks, return_exceptions=True)

    # Normalize exceptions into error dicts
    normalized = []
    for i, result in enumerate(results):
        if isinstance(result, Exception):
            normalized.append({
                "model": panel["panelists"][i],
                "content": "",
                "tokens": 0,
                "latency_ms": 0,
                "error": str(result)[:300],
            })
        else:
            normalized.append(result)

    succeeded = sum(1 for r in normalized if "error" not in r)
    log.info(
        "fan_out complete: %d/%d panelists succeeded (%s)",
        succeeded, len(normalized),
        ", ".join(f"{r['model']}={'ok' if 'error' not in r else 'FAIL'}" for r in normalized),
    )
    return normalized


def _format_responses_for_prompt(responses: list[dict[str, Any]]) -> str:
    """Format panelist responses into a text block for judge/synth prompts."""
    parts = []
    for r in responses:
        if "error" in r or not r.get("content"):
            parts.append(f"### {r['model']}\n[NO RESPONSE — error: {r.get('error', 'empty')}]")
        else:
            content = r["content"]
            if len(content) > MAX_PANELIST_TOKENS * 4:  # rough char estimate
                content = content[:MAX_PANELIST_TOKENS * 4] + "\n[...truncated...]"
            parts.append(f"### {r['model']}\n{content}")
    return "\n\n---\n\n".join(parts)


async def run_judge(
    question: str,
    responses: list[dict[str, Any]],
    judge_model: str,
    timeout_s: float,
) -> tuple[dict[str, Any] | None, bool]:
    """Run the judge pass — structural extraction. Returns (parsed_json_or_None, ok_bool)."""
    prompt_template = load_prompt("judge")
    responses_text = _format_responses_for_prompt(responses)
    filled = prompt_template.replace("{question}", question).replace("{responses}", responses_text)

    # Apply per-model transform so the judge gets its native format too
    # The filled judge prompt is the primary content → goes in "input" ([USER] section)
    judge_spec = {
        "role": "a consensus analyst evaluating panelist responses",
        "task": "Analyze panelist responses and extract consensus, contradictions, and insights.",
        "output_format": "JSON object per the schema in the instructions",
        "input": filled,
    }
    system, user, params = transform_for_model(judge_spec, judge_model)
    result = await call_model(
        judge_model,
        [{"role": "user", "content": user}],
        timeout=timeout_s,
        system=system,
        params=params,
    )

    if "error" in result or not result.get("content"):
        log.warning("judge failed: %s", result.get("error", "empty response"))
        return None, False

    # Extract JSON from the response (model may wrap in ```json ... ```)
    raw = result["content"].strip()
    if raw.startswith("```"):
        # Strip code fence
        lines = raw.split("\n")
        raw = "\n".join(lines[1:-1] if lines[-1].startswith("```") else lines[1:])

    try:
        analysis = json.loads(raw)
        log.info("judge OK (model=%s, tokens=%d)", judge_model, result["tokens"])
        return analysis, True
    except json.JSONDecodeError:
        log.warning("judge returned non-JSON, falling back to raw text")
        return {"raw_text": result["content"], "parse_error": True}, True


async def synthesize_stream(
    question: str,
    responses: list[dict[str, Any]],
    analysis: dict[str, Any] | None,
    synth_model: str,
    timeout_s: float,
) -> AsyncGenerator[str, None]:
    """Stream the synthesizer's output. Yields text deltas."""
    prompt_template = load_prompt("synthesizer")
    responses_text = _format_responses_for_prompt(responses)
    analysis_text = json.dumps(analysis, indent=2) if analysis else "Judge analysis unavailable — synthesize directly from responses."

    filled = (
        prompt_template
        .replace("{question}", question)
        .replace("{responses}", responses_text)
        .replace("{analysis}", analysis_text)
    )

    # Apply per-model transform to the synthesizer
    synth_spec = {
        "role": "a synthesis expert producing the final unified answer",
        "task": "Synthesize the panelist responses and judge analysis into one coherent answer.",
        "output_format": "The refined, coherent final answer in natural prose",
        "input": filled,
    }
    system, user, params = transform_for_model(synth_spec, synth_model)
    async for delta in stream_model(
        synth_model,
        [{"role": "user", "content": user}],
        timeout=timeout_s,
        system=system,
        params=params,
    ):
        yield delta


def _extract_question(messages: list[dict[str, str]]) -> str:
    """Extract the user's question from the messages array."""
    for msg in reversed(messages):
        if msg.get("role") == "user":
            content = msg.get("content", "")
            if isinstance(content, list):
                return " ".join(block.get("text", "") for block in content if isinstance(block, dict))
            return content
    return ""


async def run_fusion(
    tier: str,
    messages: list[dict[str, str]],
) -> dict[str, Any]:
    """Non-streaming fusion — deliberates, fans out, judges, synthesizes."""
    panel = get_panel(tier)
    question = _extract_question(messages)
    timeout_s = panel.get("timeout_ms", 60000) / 1000.0

    start = time.time()

    # 0. Deliberate — parse intent into canonical task_spec (per-model transforms)
    task_spec = await deliberate(messages, get_panel_deliberator(panel))

    # 1. Fan out (each panelist receives task in its native format)
    responses = await fan_out(panel, task_spec, timeout_s)
    valid = [r for r in responses if "error" not in r and r.get("content")]
    quorum = _min_quorum(panel)
    if len(valid) < quorum:
        # Below quorum a "fusion" answer would be one model wearing a consensus label —
        # fail loud (502) rather than silently return a single voice as cross-checked.
        log.warning("quorum not met: %d/%d valid, need %d", len(valid), len(responses), quorum)
        return {
            "error": f"fusion quorum not met: {len(valid)}/{len(responses)} panelists answered, need {quorum}",
            "details": [f"{r['model']}: {r['error'] if 'error' in r else 'empty'}" for r in responses if "error" in r or not r.get("content")],
        }

    # 2. Judge
    analysis, judge_ok = await run_judge(question, valid, panel["judge"], timeout_s)

    # 3. Synthesize (collect full text for non-streaming mode)
    synth_text_parts = []
    async for delta in synthesize_stream(question, valid, analysis, panel["synth"], timeout_s):
        synth_text_parts.append(delta)
    synth_text = "".join(synth_text_parts)

    elapsed_ms = int((time.time() - start) * 1000)
    _summary_log(tier, elapsed_ms, responses, panel["judge"], judge_ok, panel["synth"])

    return {
        "id": f"fusion-{uuid.uuid4().hex[:12]}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": tier,
        "choices": [{
            "index": 0,
            "message": {"role": "assistant", "content": synth_text},
            "finish_reason": "stop",
        }],
        "usage": {
            "prompt_tokens": sum(r.get("tokens", 0) for r in responses),
            "completion_tokens": len(synth_text) // 4,
            "total_tokens": sum(r.get("tokens", 0) for r in responses) + len(synth_text) // 4,
        },
        "_meta": {
            "tier": tier.replace("hivemind/fusion-", ""),
            "elapsed_ms": elapsed_ms,
            "panelists": [{"model": r["model"], "tokens": r.get("tokens", 0), "latency_ms": r.get("latency_ms", 0), "error": r.get("error")} for r in responses],
            "judge": panel["judge"],
            "synth": panel["synth"],
        },
    }


async def run_fusion_stream(
    tier: str,
    messages: list[dict[str, str]],
) -> AsyncGenerator[str, None]:
    """Streaming fusion — yields OpenAI SSE chunks."""
    panel = get_panel(tier)
    question = _extract_question(messages)
    timeout_s = panel.get("timeout_ms", 60000) / 1000.0
    completion_id = f"fusion-{uuid.uuid4().hex[:12]}"
    created = int(time.time())

    start = time.time()

    # 0. Deliberate — parse intent into canonical task_spec
    task_spec = await deliberate(messages, get_panel_deliberator(panel))

    # 1. Fan out (per-model transforms applied)
    responses = await fan_out(panel, task_spec, timeout_s)
    valid = [r for r in responses if "error" not in r and r.get("content")]

    quorum = _min_quorum(panel)
    if len(valid) < quorum:
        log.warning("quorum not met (stream): %d/%d valid, need %d", len(valid), len(responses), quorum)
        msg = f"[Fusion error: quorum not met — {len(valid)}/{len(responses)} panelists answered, need {quorum}]"
        error_chunk = {
            "id": completion_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": tier,
            "choices": [{"index": 0, "delta": {"content": msg}, "finish_reason": None}],
        }
        yield f"data: {json.dumps(error_chunk)}\n\n"
        yield "data: [DONE]\n\n"
        return

    # 2. Judge
    analysis, judge_ok = await run_judge(question, valid, panel["judge"], timeout_s)

    # 3. Stream synthesis
    async for delta in synthesize_stream(question, valid, analysis, panel["synth"], timeout_s):
        chunk = {
            "id": completion_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": tier,
            "choices": [{"index": 0, "delta": {"content": delta}, "finish_reason": None}],
        }
        yield f"data: {json.dumps(chunk)}\n\n"

    # Final chunk
    elapsed_ms = int((time.time() - start) * 1000)
    _summary_log(tier, elapsed_ms, responses, panel["judge"], judge_ok, panel["synth"])
    final_chunk = {
        "id": completion_id,
        "object": "chat.completion.chunk",
        "created": created,
        "model": tier,
        "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
    }
    yield f"data: {json.dumps(final_chunk)}\n\n"
    yield "data: [DONE]\n\n"
