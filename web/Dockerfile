# Build context: repository root. Produces a Caddy image that serves the web app
# and proxies /api and /auth to the gateway.
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /repo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY gateway/package.json gateway/
COPY web/package.json web/
RUN pnpm install --frozen-lockfile --filter @dipwatch/web
COPY web web
RUN pnpm --filter @dipwatch/web build

FROM caddy:2-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /repo/web/dist /srv
