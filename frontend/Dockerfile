# Multi-stage Dockerfile for React Frontend
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package manifests and install dependencies
COPY package.json package-lock.json* ./
RUN npm install

# Copy application source code and configuration
COPY tsconfig.json vite.config.ts index.html ./
COPY src ./src

# Ensure test file compiles under DOM/ES2020 lib in strict TypeScript mode
RUN sed -i 's/global\.fetch/(globalThis as any).fetch/g' src/test/unit.test.ts

# Build production bundle
RUN npm run build

# Runtime stage with Nginx
FROM nginx:1.25-alpine

RUN apk --no-cache add wget curl

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=5s --timeout=3s --retries=5 --start-period=5s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:80/health || exit 1

CMD ["nginx", "-g", "daemon off;"]
