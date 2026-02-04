FROM ubuntu:24.04

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    bash coreutils ca-certificates tar util-linux \
  && rm -rf /var/lib/apt/lists/*

# Create a uid/gid 1000 user.
# Some bases already have uid/gid 1000; reuse that account.
RUN set -eux; \
  getent group 1000 >/dev/null || groupadd -g 1000 sandbox; \
  if getent passwd 1000 >/dev/null; then \
    :; \
  else \
    useradd -m -u 1000 -g 1000 -s /bin/bash sandbox; \
  fi

RUN mkdir -p /workspace /artifacts /tmp \
  && chown -R 1000:1000 /workspace /artifacts /tmp

USER 1000:1000
WORKDIR /workspace

CMD ["sleep", "infinity"]
