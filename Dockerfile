### Frontend build
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web .
RUN npm run build

### Go build
FROM golang:1.25.1-alpine AS go-build
WORKDIR /app
RUN apk add --no-cache build-base sqlite-dev
ENV GOTOOLCHAIN=go1.25.1
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -o /out/litetask ./cmd/litetask

### Runtime
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=go-build /out/litetask /app/litetask
COPY --from=web-build /web/dist /app/web/dist
ENV DB_PATH=/data/tasks.db \
    ALLOW_REGISTRATION=true \
    PORT=8080
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/litetask"]
