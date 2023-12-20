FROM registry.access.redhat.com/ubi9/go-toolset:1.20 AS builder

USER root
RUN mkdir -p /app
WORKDIR /app
COPY . .
RUN go build -o proxy -ldflags="-s -w" ./cmd/update-proxy


FROM registry.access.redhat.com/ubi9-minimal

WORKDIR /etc/pki/ca-trust/source/anchors
RUN curl -o ROSENRootCA.pem https://crl.roseninspection.net/ROSENRootCA.pem ; curl -o ROSENSubCA.pem https://crl.roseninspection.net/ROSENSubCA.pem ; curl -o ROSENKubeCA.pem https://crl.roseninspection.net/ROSENKubeCA.pem ; update-ca-trust

WORKDIR /
COPY --from=builder /app/proxy /proxy

ENTRYPOINT [ "/proxy" ]
CMD []