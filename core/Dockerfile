# Two images from one Dockerfile:
#   --target core    (default) core-api and worker binaries on Alpine, ~50 MB
#   --target worker  worker plus Node, Chromium and Playwright MCP for the browsing agent
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/core-api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/source-smoke ./cmd/source-smoke

# Debian-based, because Playwright's Chromium needs glibc and its system libraries.
FROM node:24-bookworm-slim AS worker
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    PLAYWRIGHT_MCP_COMMAND="playwright-mcp --no-sandbox"
RUN npm install --global --registry=https://registry.npmjs.org/ @playwright/mcp@0.0.86 \
 && playwright-mcp install-browser chromium --with-deps \
 && rm -rf /root/.npm /var/lib/apt/lists/* \
 && useradd --create-home --uid 10001 app
# source-smoke lets you test a store from inside the container:
#   docker compose exec worker source-smoke -source amazon-us -browser-only
COPY --from=build /out/worker /out/source-smoke /usr/local/bin/
USER app
CMD ["worker"]

FROM alpine:3.22 AS core
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 app
COPY --from=build /out/core-api /out/worker /usr/local/bin/
USER app
EXPOSE 8000
CMD ["core-api", "serve"]
