FROM node:24-alpine AS builder
RUN apk add --no-cache git=2.54.0-r0
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/dist /app/dist-fresh
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["sleep", "infinity"]
