# Build stage
FROM oven/bun:alpine AS builder

WORKDIR /app

# Copy lock files and workspace config
COPY bun.lock package.json ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy source and gen
COPY . .

# Final stage
FROM oven/bun:alpine

WORKDIR /app

COPY --from=builder /app /app

ENV NODE_ENV=production
ENV NATS_URL=nats://nats:4222

CMD ["bun", "run", "./src/bootstrap.ts"]
