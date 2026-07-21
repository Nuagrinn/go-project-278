# Build backend
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache git
WORKDIR /build/code

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /build/app .

# Build frontend
FROM node:24-alpine AS frontend-builder
WORKDIR /build/frontend

COPY package.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm install --omit=dev

RUN cp -R node_modules/@hexlet/project-url-shortener-frontend/dist ./dist

# Runtime
FROM caddy:2-alpine
RUN apk add --no-cache bash ca-certificates libcap \
    && (setcap -r /usr/bin/caddy || true)

WORKDIR /app

COPY --from=backend-builder /build/app /app/bin/app
COPY --from=frontend-builder /build/frontend/dist /app/public

COPY --from=backend-builder /build/code/db/migrations /app/db/migrations
COPY --from=backend-builder /go/bin/goose /usr/local/bin/goose

COPY Caddyfile /app/Caddyfile
COPY bin/run.sh /app/bin/run.sh
RUN chmod +x /app/bin/run.sh

EXPOSE 8080

CMD ["/app/bin/run.sh"]
