# Multi-stage build for Next.js frontend using Bun
FROM oven/bun:alpine AS deps
WORKDIR /app
COPY package.json bun.lock* ./
RUN bun ci || bun install

FROM oven/bun:alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules /app/node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN bun run build

FROM oven/bun:alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /app/.next /app/.next
COPY --from=builder /app/package.json /app/package.json
COPY --from=builder /app/public /app/public
EXPOSE 3000
ENTRYPOINT ["bun", "run", "start"]
