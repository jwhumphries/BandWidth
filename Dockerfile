FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS gobase

RUN apk add --no-cache git
RUN go install github.com/air-verse/air@latest

FROM ghcr.io/jwhumphries/frontend:latest@sha256:7270e8fbe7f4629906764604b12b77182e3714474ccee5b14aa01ccc6b8a5b53 AS dev

WORKDIR /app

COPY --from=gobase /usr/local/go /usr/local/go
COPY --from=gobase /go/bin/air /usr/local/bin/air

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV GOCACHE=/go-build-cache
ENV GOMODCACHE=/go/pkg/mod

COPY scripts/develop.sh /develop.sh
RUN chmod +x /develop.sh

EXPOSE 8080 3000

CMD ["/develop.sh"]
