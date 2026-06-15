# Improve Stage: Recon (Local)

You are performing the recon phase of a codebase audit. Your job is to map the target repository's structure, tooling, and churn patterns.

## Task
Analyze the target repository and produce a structured recon report.

## Deliverables
Return JSON:
```json
{
  "build_commands": ["detected build commands (make, npm, cargo, go build, etc.)"],
  "test_commands": ["detected test commands"],
  "lint_commands": ["detected linters (ruff, eslint, golangci-lint, etc.)"],
  "high_churn_files": ["top 10-20 files by recent commit activity"],
  "dependency_manifests": ["package.json, requirements.txt, Cargo.toml, go.mod, etc."],
  "claude_md_present": true,
  "languages": ["primary languages detected"],
  "loc_total": "approximate total lines of code"
}
```

## Method
- Use git log, git diff --stat, and filesystem inspection
- Detect CI configs (.github/workflows, .gitlab-ci.yml)
- Identify the dominant framework(s) from manifests
- Report only what you can verify — do not speculate
