FROM registry.access.redhat.com/ubi9/go-toolset:1.19 AS builder

USER root
RUN mkdir -p /app
WORKDIR /app
COPY . .
RUN go build -o proxy -ldflags="-s -w" ./cmd/update-proxy


FROM registry.access.redhat.com/ubi9-minimal

WORKDIR /etc/pki/ca-trust/source/anchors
RUN curl -o ROSENRootCA.pem https://crl.roseninspection.net/ROSENRootCA.pem ; \
    curl -o ROSENSubCA.pem https://crl.roseninspection.net/ROSENSubCA.pem ; \
    curl -o ROSENKubeCA.pem https://crl.roseninspection.net/ROSENKubeCA.pem ; \
    curl -o gw-lin.roseninspection.net.pem https://crl.roseninspection.net/gw-lin.roseninspection.net.pem; \
    curl -o pa-ca.roseninspection.net.pem https://crl.roseninspection.net/pa-ca.roseninspection.net.pem ; \
    curl -o pan-ca.roseninspection.net.pem https://crl.roseninspection.net/pan-ca.roseninspection.net.pem ; \
    update-ca-trust

WORKDIR /
COPY config.default.yaml /config.yaml
COPY --from=builder /app/proxy /proxy

ENTRYPOINT [ "/proxy" ]
CMD []