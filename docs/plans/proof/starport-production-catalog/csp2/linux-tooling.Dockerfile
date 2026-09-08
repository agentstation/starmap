FROM ubuntu@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517
RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates curl git && rm -rf /var/lib/apt/lists/*
RUN install -d -m 0700 -o 65532 -g 65532 /home/nonroot
RUN dpkg-query -W bash ca-certificates curl git libc6 > /tooling-versions.txt
USER 65532:65532
