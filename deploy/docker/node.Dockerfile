# syntax=docker/dockerfile:1.7
#
# Simulated 1Panel slave "server": just the agent + sshd, so the master's
# EnrollViaSSH (SSH in -> write /etc/1panel/bootstrap/* -> `systemctl
# restart 1panel-agent`) works against it. A `systemctl` shim actually
# bounces the agent (no systemd in a container).
FROM ubuntu:24.04

# docker.io: the agent aborts at startup (before its logger inits, so
# silently) if it can't reach Docker. The socket is bind-mounted at run.
RUN apt-get update && apt-get install -y --no-install-recommends \
      openssh-server ca-certificates coreutils iproute2 procps docker.io \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/sshd \
    && echo 'root:nodepass123' | chpasswd \
    && sed -ri 's/^#?PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config \
    && sed -ri 's/^#?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config

# The agent binary, lifted from the panel image we already built.
COPY --from=1panel/local:dev /usr/local/bin/1panel-agent /usr/local/bin/1panel-agent

COPY deploy/docker/node-entrypoint.sh /usr/local/bin/node-entrypoint.sh
COPY deploy/docker/systemctl-shim.sh  /usr/local/bin/systemctl
RUN chmod +x /usr/local/bin/node-entrypoint.sh /usr/local/bin/systemctl /usr/local/bin/1panel-agent \
    && ln -sf /usr/local/bin/systemctl /usr/local/bin/service

EXPOSE 22 9999
ENTRYPOINT ["/usr/local/bin/node-entrypoint.sh"]
