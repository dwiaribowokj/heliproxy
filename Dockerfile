FROM golang:1.23-alpine AS build

ENV PATH="/usr/local/go/bin:${PATH}"
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/heliproxy .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=build /out/heliproxy /app/heliproxy
RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser
ENV DATA_DIR=/app/data
EXPOSE 18081 18082
VOLUME ["/app/data"]

ENTRYPOINT ["/app/heliproxy"]
