FROM registry.access.redhat.com/ubi9/go-toolset:1.19 as builder

USER root
RUN mkdir -p /app
WORKDIR /app
COPY . .
RUN go build -o proxy -ldflags="-s -w" ./cmd/update-proxy

FROM registry.access.redhat.com/ubi9-minimal

ENV TZ=Europe/Berlin

COPY --from=builder /app/proxy /proxy

ENTRYPOINT [ "/proxy" ]
CMD []