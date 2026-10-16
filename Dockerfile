FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS gobase

RUN apk add --no-cache git
RUN go install github.com/air-verse/air@latest

FROM ghcr.io/jwhumphries/frontend:latest@sha256:d8923d9071c21dd38e76b297f3afe50f400d2d177413500747374bafb3c89548 AS dev

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
