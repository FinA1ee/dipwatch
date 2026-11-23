# Builds both Go binaries into one image. Run with `core-api` or `worker` as the command.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/core-api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 app
COPY --from=build /out/ /usr/local/bin/
USER app
EXPOSE 8000
CMD ["core-api", "serve"]
