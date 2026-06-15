"""Config loader — panels.toml + prompt files."""

import tomllib
from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent
PANELS_FILE = BASE_DIR / "panels.toml"
PROMPTS_DIR = BASE_DIR / "prompts"


def load_panels() -> dict[str, Any]:
    with open(PANELS_FILE, "rb") as f:
        data = tomllib.load(f)
    return data


def load_prompt(name: str) -> str:
    return (PROMPTS_DIR / f"{name}.md").read_text()


def get_panel(tier: str) -> dict[str, Any]:
    panels = load_panels()
    tier_key = tier.replace("hivemind/fusion-", "")
    if tier_key not in panels.get("panels", {}):
        raise ValueError(f"unknown fusion tier: {tier}")
    return panels["panels"][tier_key]


def get_hivemind_endpoint() -> str:
    return load_panels().get("hivemind", {}).get("endpoint", "http://127.0.0.1:8400/v1")


def get_panel_deliberator(panel: dict[str, Any]) -> str:
    """Get the deliberator model for a panel (default glm-5.1)."""
    return panel.get("deliberator", "glm-5.1")
