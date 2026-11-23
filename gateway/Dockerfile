# Build context: repository root.
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /repo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY gateway/package.json gateway/
COPY web/package.json web/
RUN pnpm install --frozen-lockfile --filter @dipwatch/gateway
COPY gateway gateway
RUN pnpm --filter @dipwatch/gateway build \
 && pnpm --filter @dipwatch/gateway deploy --prod /out

FROM node:24-alpine
WORKDIR /app
COPY --from=build /out/package.json ./
COPY --from=build /out/node_modules ./node_modules
COPY --from=build /repo/gateway/dist ./dist
USER node
EXPOSE 3000
CMD ["node", "dist/server.js"]
