# The node of the rotation scenario: Alpine, the pinned Nebula release, an
# sshd, and the systemctl shim that answers the calls meshy makes. A container
# has no init, so the shim reloads the real nebula process with SIGHUP.
FROM alpine:3.20

# iputils supplies the ping that the scenario runs across the tunnel.
RUN apk add --no-cache iputils openssh sudo

COPY .build/nebula /usr/local/bin/nebula
COPY .build/nebula-cert /usr/local/bin/nebula-cert
COPY systemctl /usr/local/bin/systemctl
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

# The meshy user reaches the node over ssh and runs the two unit commands that
# meshy sends through sudo. The rule names the exact command line.
RUN chmod 0755 /usr/local/bin/nebula /usr/local/bin/nebula-cert /usr/local/bin/systemctl /usr/local/bin/entrypoint.sh \
    && adduser -D -s /bin/sh meshy \
    && echo 'meshy:meshy-e2e-only' | chpasswd \
    && printf '%s\n' \
        'meshy ALL=(root) NOPASSWD: /usr/local/bin/systemctl reload nebula@meshy-rot.service, /usr/local/bin/systemctl restart nebula@meshy-rot.service, /usr/local/bin/systemctl start nebula@meshy-rot.service' \
        > /etc/sudoers.d/meshy \
    && chmod 0440 /etc/sudoers.d/meshy

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
