"""Per-model prompt transformation — deliberation → family-native format.

Maps each panelist model to its family (claude/gpt/glm/qwen), renders the
canonical task_spec into a system+user prompt using the family's template,
and selects per-family sampling params.

Research sources:
  claude: docs.claude.com/prompt-engineering (XML tags, effort, no prefilling on 4.6+)
  gpt:    cookbook.openai.com/gpt-5 (reasoning model — do NOT prompt CoT, reasoning_effort)
  glm:    github.com/zai-org/glm-5 (OpenAI-compatible, repetition_penalty 1.05, no json_schema)
  qwen:   qwen.readthedocs.io (ChatML, non-thinking temp 0.7/top_p 0.8, never greedy)
"""

from pathlib import Path
from typing import Any

PROMPTS_DIR = Path(__file__).resolve().parent / "prompts"

# Per-family sampling defaults (sourced from official docs).
# Variant-specific overrides (opus→xhigh, haiku→medium, mini→medium) applied in transform.
FAMILY_PARAMS: dict[str, dict[str, Any]] = {
    "claude": {"effort": "high"},        # default; opus→xhigh, haiku→medium
    "gpt":    {"reasoning_effort": "medium"},  # default; gpt-5.5→high, mini→medium
    "glm":    {"temperature": 0.7, "top_p": 0.9, "repetition_penalty": 1.05},
    "qwen":   {"temperature": 0.7, "top_p": 0.8},  # non-thinking mode for 27B
}

# Model variant → effort/reasoning_effort overrides (within a family)
VARIANT_EFFORT: dict[str, str] = {
    "claude-opus-4-8":   "xhigh",
    "claude-sonnet-4-6": "high",
    "claude-haiku-4-5":  "medium",
    "gpt-5.5":           "high",
    "gpt-5.4":           "medium",
    "gpt-5.4-mini":      "medium",
    "gpt-5.3-codex":     "high",
    "gpt-5.2":           "medium",
}


def family_of(model: str) -> str:
    """Detect model family from the model slug.

    Order matters: qwopus contains "opus" so qwen must be checked before claude.
    """
    m = model.lower()
    if "qwen" in m or "qwopus" in m or "ollama" in m or "turboquant" in m:
        return "qwen"
    if "claude" in m or "opus" in m or "sonnet" in m or "haiku" in m:
        return "claude"
    if "gpt" in m:
        return "gpt"
    if "glm" in m:
        return "glm"
    # ponytail: unknown → claude-style (most general XML format) — upgrade: add explicit fallback policy
    return "claude"


def _coerce_str(value: Any) -> str:
    """Render task_spec values to template strings (lists → bullet lines, None → empty)."""
    if value is None:
        return ""
    if isinstance(value, list):
        return "\n".join(f"- {item}" for item in value)
    return str(value)


def load_family_template(family: str) -> str:
    """Load the .tmpl file for a family. Raises FileNotFoundError if missing."""
    path = PROMPTS_DIR / "formats" / f"{family}.tmpl"
    return path.read_text()


def _split_system_user(rendered: str) -> tuple[str, str]:
    """Split a rendered template into (system, user) by the [SYSTEM]/[USER] markers."""
    # ponytail: simple marker split — upgrade: proper template parser if complexity grows
    if "[USER]" in rendered:
        parts = rendered.split("[USER]", 1)
        system_block = parts[0]
        user = parts[1].strip()
    else:
        system_block = rendered
        user = ""

    # Strip the [SYSTEM] marker from the system block
    system = system_block.replace("[SYSTEM]", "", 1).strip()
    return system, user


def transform_for_model(
    task_spec: dict[str, Any],
    model: str,
) -> tuple[str, str, dict[str, Any]]:
    """Render the canonical task_spec into (system, user, params) for the model's family.

    task_spec keys: role, task, context, constraints, output_format, guidance_blocks, input
    Returns (system_prompt, user_prompt, sampling_params).
    """
    family = family_of(model)
    template = load_family_template(family)

    # Build the substitution map — coerce all values to strings
    subs = {
        "role":            _coerce_str(task_spec.get("role", "an expert assistant")),
        "role_expansion":  _coerce_str(task_spec.get("role", "an expert assistant")),
        "task":            _coerce_str(task_spec.get("task", task_spec.get("input", ""))),
        "context":         _coerce_str(task_spec.get("context", "")),
        "constraints":     _coerce_str(task_spec.get("constraints", "")),
        "output_format":   _coerce_str(task_spec.get("output_format", "Respond clearly and concisely.")),
        "guidance_blocks": _coerce_str(task_spec.get("guidance_blocks", "")),
        "input":           _coerce_str(task_spec.get("input", "")),
    }

    # Simple str.format replacement (template uses {key} placeholders)
    try:
        rendered = template.format(**subs)
    except KeyError:
        # ponytail: if template has unknown placeholder, render with safe_map — upgrade: use a proper templating lib
        import re
        rendered = re.sub(r"\{(\w+)\}", lambda m: subs.get(m.group(1), ""), template)

    system, user = _split_system_user(rendered)

    # Build params with variant-specific effort overrides
    params = FAMILY_PARAMS[family].copy()
    if model in VARIANT_EFFORT:
        effort_val = VARIANT_EFFORT[model]
        if family == "claude":
            params["effort"] = effort_val
        elif family == "gpt":
            params["reasoning_effort"] = effort_val

    return system, user, params
