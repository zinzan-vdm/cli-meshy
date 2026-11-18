# The one image that the lighthouse and the empty host run: Alpine, an ssh
# server, the pinned Nebula release, and the recorder that stands in for systemd.
FROM alpine:3.20

# openssh-server supplies sshd. shadow supplies the useradd, usermod, and visudo
# target that the bootstrap script needs, because the busybox useradd does not
# accept --create-home. coreutils supplies stat, install, and sha256sum, and
# findutils supplies the find -printf that the tree checksum reads. iputils
# supplies ping and iproute2 supplies ip.
RUN apk add --no-cache openssh-server sudo shadow coreutils findutils iputils iproute2 \
 && if [ ! -e /usr/sbin/nologin ]; then ln -s /sbin/nologin /usr/sbin/nologin; fi

# The image carries the pinned release, but not at /usr/local/bin/nebula. That
# path stays empty, so meshy install proves the upload of the bytes that
# mesh.yml pins.
COPY .build/nebula /usr/local/lib/meshy/nebula

# /usr/local/bin comes before every other directory on PATH, so the recorder
# answers every systemctl call that bootstrap and apply make.
COPY systemctl /usr/local/bin/systemctl
COPY supervise.sh /usr/local/bin/supervise
COPY entrypoint.sh /usr/local/bin/entrypoint
COPY sshd_config /etc/ssh/sshd_config
RUN chmod 0755 /usr/local/bin/systemctl /usr/local/bin/supervise /usr/local/bin/entrypoint

CMD ["entrypoint"]
