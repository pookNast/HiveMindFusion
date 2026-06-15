#!/usr/bin/env bash
# claude-cli-proxy — start/stop the Claude CLI → OpenAI-compat proxy on :8095
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
PIDFILE="$DIR/.proxy.pid"
LOGFILE="$DIR/logs/proxy.log"

mkdir -p "$DIR/logs"

case "${1:-start}" in
  start)
    if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
      echo "claude-cli-proxy already running (PID $(cat "$PIDFILE"))"
      exit 0
    fi
    # ponytail: no venv for v1 — system Python + pip packages
    # upgrade: create dedicated venv if dependency conflicts arise
    cd "$DIR"
    nohup python3 -m uvicorn server:app \
      --host 127.0.0.1 --port 8095 \
      >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    echo "claude-cli-proxy started (PID $!) on :8095, logs: $LOGFILE"
    ;;
  stop)
    if [ -f "$PIDFILE" ]; then
      PID="$(cat "$PIDFILE")"
      kill "$PID" 2>/dev/null || true
      rm -f "$PIDFILE"
      echo "claude-cli-proxy stopped (PID $PID)"
    else
      echo "claude-cli-proxy not running"
    fi
    ;;
  restart)
    "$DIR/restart.sh" stop
    sleep 1
    "$DIR/restart.sh" start
    ;;
  status)
    if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
      echo "running (PID $(cat "$PIDFILE"))"
    else
      echo "stopped"
    fi
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
