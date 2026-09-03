FROM tailscale/tailscale:v1.102.3

USER root
COPY lab/tls/tls.crt /usr/local/share/ca-certificates/headscale.crt
RUN update-ca-certificates
