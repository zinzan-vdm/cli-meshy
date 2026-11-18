# The runner of the empty-host run: the meshy binary that the job built, the
# signing tool, the verified release, and an ssh client. It drives the host and
# the lighthouse over ssh and holds the only private key.
FROM alpine:3.20

# openssh-client supplies ssh and ssh-keyscan. coreutils supplies sha256sum and
# cut.
RUN apk add --no-cache openssh-client coreutils

COPY .build/meshy /usr/local/bin/meshy
COPY .build/nebula-cert /usr/local/bin/nebula-cert
COPY .build/release/SHASUM256.txt /work/release/SHASUM256.txt
COPY .build/release/nebula-linux-amd64.tar.gz /work/release/nebula-linux-amd64.tar.gz
COPY mesh.yml /work/mesh/mesh.yml
COPY run.sh /usr/local/bin/run

RUN chmod 0755 /usr/local/bin/meshy /usr/local/bin/nebula-cert /usr/local/bin/run

WORKDIR /work
CMD ["run"]
