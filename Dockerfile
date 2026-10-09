FROM golang:1.27-alpine@sha256:738d1cf061836894ff6bb8c33881080ac66de8cf0586615012a0c8f592649cfa AS gobase

RUN apk add --no-cache git
RUN go install github.com/air-verse/air@latest

FROM ghcr.io/jwhumphries/frontend:latest@sha256:e42a64ad2da5fce40cde0f21e6fdc3deb6ff278fd31dbfc670bd9ee7bcaf6e2f AS dev

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
