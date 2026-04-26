### Frontend build
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web .
RUN npm run build

### Go build
FROM golang:1.26.2-alpine AS go-build
WORKDIR /app
ENV GOTOOLCHAIN=go1.26.2
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -o /out/litetask ./cmd/litetask

### Runtime
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=go-build /out/litetask /app/litetask
COPY --from=web-build /web/dist /app/web/dist
ENV DB_PATH=/data/tasks.db \
    ALLOW_REGISTRATION=true \
    PORT=8080
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/litetask"]
