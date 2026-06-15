#!/usr/bin/env bash
# hivemind-fusion — start/stop the model fusion sidecar on :8500
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
PIDFILE="$DIR/.fusion.pid"
LOGFILE="$DIR/logs/fusion.log"

mkdir -p "$DIR/logs"

case "${1:-start}" in
  start)
    if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
      echo "hivemind-fusion already running (PID $(cat "$PIDFILE"))"
      exit 0
    fi
    cd "$DIR"
    nohup python3 -m uvicorn server:app \
      --host 127.0.0.1 --port 8500 \
      >> "$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    echo "hivemind-fusion started (PID $!) on :8500, logs: $LOGFILE"
    ;;
  stop)
    if [ -f "$PIDFILE" ]; then
      PID="$(cat "$PIDFILE")"
      kill "$PID" 2>/dev/null || true
      rm -f "$PIDFILE"
      echo "hivemind-fusion stopped (PID $PID)"
    else
      echo "hivemind-fusion not running"
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
