# W4-S1: OpenRC service file + install script

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S5

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create etc/openrc/hivemind init script — start/stop/restart/status. Depends on: net, localmount. Sources /etc/hivemind/env. Binary at /usr/local/bin/hivemind-gw. PID file at /run/hivemind.pid. Log to /var/log/hivemind/. Create install.sh that builds, copies binary, installs service, creates config dir.

## Files to modify
- etc/openrc/hivemind
- scripts/install.sh

## Acceptance criteria
- [ ] rc-service hivemind start launches gateway
- [ ] rc-service hivemind stop graceful shutdown
- [ ] rc-status shows hivemind
- [ ] install.sh builds and deploys in one command

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && test -f /home/pook/ralph/hivemind/etc/openrc/hivemind && head -5 /home/pook/ralph/hivemind/etc/openrc/hivemind
```

## Commit
```
feat: [W4-S1] OpenRC service file + install script
```
