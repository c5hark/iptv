# --- сборка ---
FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/iptv ./cmd/iptv

# --- запуск ---
FROM alpine:3

RUN adduser -D -u 10001 app && mkdir /data && chown app /data
COPY --from=build /out/iptv /usr/local/bin/iptv

USER app
ENV IPTV_ADDR=:8080 \
    IPTV_DB_PATH=/data/iptv.db
EXPOSE 8080

ENTRYPOINT ["iptv"]