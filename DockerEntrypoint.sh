#!/bin/bash
# =============================================================================
# l-ui Docker Entrypoint
# =============================================================================
# Signal handling:
#   SIGTERM  → graceful shutdown (l-ui handles this internally)
#   SIGUSR1  → reload Xray config on nodes (handled by agent)
#
# Fail2ban requirements:
#   - LUI_ENABLE_FAIL2BAN=true must be set
#   - Docker container needs NET_ADMIN capability: --cap-add=NET_ADMIN
#   - Volume mounts needed:
#       - /var/log/l-ui   (or LUI_LOG_FOLDER) for fail2ban log files
#       - /etc/l-ui/      for config persistence
# =============================================================================

set -euo pipefail

# =============================================================================
# Setup fail2ban with 3x-ipl jail (optional, NET_ADMIN cap required)
# =============================================================================
setup_fail2ban() {
    if [ "${LUI_ENABLE_FAIL2BAN:-false}" != "true" ]; then
        return 0
    fi
    # Skip if not running as root (fail2ban config paths require root).
    if [ "$(id -u)" != "0" ]; then
        echo "fail2ban setup skipped (not running as root)"
        return 0
    fi

    LOG_FOLDER="${LUI_LOG_FOLDER:-/var/log/l-ui}"
    mkdir -p "$LOG_FOLDER"
    touch "$LOG_FOLDER/3xipl.log" "$LOG_FOLDER/3xipl-banned.log"

    mkdir -p /etc/fail2ban/jail.d /etc/fail2ban/filter.d /etc/fail2ban/action.d

    cat > /etc/fail2ban/jail.d/3x-ipl.conf << 'EOF'
[3x-ipl]
enabled=true
backend=auto
filter=3x-ipl
action=3x-ipl
logpath=%LOG_FOLDER%/3xipl.log
maxretry=1
findtime=32
bantime=30m
EOF

    cat > /etc/fail2ban/filter.d/3x-ipl.conf << 'EOF'
[Definition]
dateformat = ^%%Y/%%m/%%d %%H:%%M:%%S
failregex   = \[LIMIT_IP\]\s*Email\s*=\s*<F-USER>.+</F-USER>\s*\|\|\s*Disconnecting OLD IP\s*=\s*<ADDR>\s*\|\|\s*Timestamp\s*=\s*\d+
ignoreregex =
EOF

    cat > /etc/fail2ban/action.d/3x-ipl.conf << EOF
[INCLUDES]
before = iptables-allports.conf

[Definition]
actionstart = <iptables> -N f2b-<name>
              <iptables> -A f2b-<name> -j <returntype>
              <iptables> -I <chain> -p <protocol> -j f2b-<name>

actionstop = <iptables> -D <chain> -p <protocol> -j f2b-<name>
             <actionflush>
             <iptables> -X f2b-<name>

actioncheck = <iptables> -n -L <chain> | grep -q 'f2b-<name>[ \t]'

actionban = <iptables> -I f2b-<name> 1 -s <ip> -j <blocktype>
            echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   BAN   [Email] = <F-USER> [IP] = <ip> banned for <bantime> seconds." >> $LOG_FOLDER/3xipl-banned.log

actionunban = <iptables> -D f2b-<name> -s <ip> -j <blocktype>
              echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   UNBAN   [Email] = <F-USER> [IP] = <ip> unbanned." >> $LOG_FOLDER/3xipl-banned.log

[Init]
name = default
protocol = tcp
chain = INPUT
EOF

    fail2ban-client -x start
}

# =============================================================================
# Wait for l-ui health check endpoint
# =============================================================================
wait_for_health() {
    HEALTH_URL="${LUI_HEALTH_URL:-http://localhost:2053/healthz}"
    HEALTH_TIMEOUT="${LUI_HEALTH_TIMEOUT:-30}"

    if command -v wget > /dev/null 2>&1; then
        WGET_CMD="wget -q -O- --timeout=1"
    else
        WGET_CMD="curl -s --max-time 1"
    fi

    echo "[entrypoint] Waiting for l-ui health check at $HEALTH_URL (timeout: ${HEALTH_TIMEOUT}s)..."

    start_time=$(date +%s)
    while true; do
        if $WGET_CMD "$HEALTH_URL" > /dev/null 2>&1; then
            echo "[entrypoint] l-ui is ready"
            return 0
        fi

        elapsed=$(($(date +%s) - start_time))
        if [ "$elapsed" -ge "$HEALTH_TIMEOUT" ]; then
            echo "[entrypoint] Health check timeout after ${HEALTH_TIMEOUT}s — proceeding anyway"
            return 0
        fi

        sleep 1
    done
}

# =============================================================================
# Main
# =============================================================================
setup_fail2ban

# Start l-ui in background, wait for health check, then bring to foreground.
/app/l-ui "$@" &
LUI_PID=$!
wait_for_health
wait "$LUI_PID"