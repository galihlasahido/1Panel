#!/bin/sh
# Minimal systemctl/service replacement for the simulated node.
#
# The agent itself runs read-only queries during init (e.g.
# `systemctl is-active/status 1panel-agent`). We must NOT kill it for
# those — only for an explicit restart/start ACTION, which is what the
# master issues to apply the bootstrap:
#   systemctl restart 1panel-agent   (argv: restart 1panel-agent)
#   service 1panel-agent restart     (argv: 1panel-agent restart)
# On a real bounce the supervisor loop relaunches the agent, which then
# re-reads /etc/1panel/bootstrap/* and comes up in mTLS slave mode.

want_agent=0
want_action=0
for a in "$@"; do
    case "$a" in
        1panel-agent|1panel-agent.service) want_agent=1 ;;
        restart|start|try-restart|reload-or-restart|force-reload) want_action=1 ;;
    esac
done

if [ "$want_agent" = 1 ] && [ "$want_action" = 1 ]; then
    pkill -x 1panel-agent 2>/dev/null || true
    echo "[systemctl-shim] bounced 1panel-agent"
fi

# Everything else (status/is-active/is-enabled/show/daemon-reload/…)
# is a no-op success so the agent's init queries don't disturb it.
exit 0
