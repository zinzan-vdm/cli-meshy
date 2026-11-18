# The one image that every service in the e2e mesh runs: Alpine, the pinned
# Nebula release, and the harness. CI builds it once and tags it meshy-e2e:local.
FROM alpine:3.20

# iputils supplies the ping that the first assertion runs.
RUN apk add --no-cache iputils

COPY .build/nebula /usr/local/bin/nebula
COPY .build/harness /usr/local/bin/harness

CMD ["nebula", "-config", "/etc/nebula/meshy-lab/config.yml"]
