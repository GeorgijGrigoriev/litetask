.PHONY: docker-build docker-build-amd64 build-frontend build build-amd64 dev

IMAGE ?= litetask:latest
GHCR_IMAGE ?= ghcr.io/georgijgrigoriev/litetask:latest

docker-build:
	docker build -t $(IMAGE) .

docker-build-amd64:
	docker buildx build --platform linux/amd64 -t $(GHCR_IMAGE) --push .

build-frontend:
	cd web && npm run build

build:
	go build -o litetask ./cmd/litetask

build-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o litetask-amd64 ./cmd/litetask

dev:
	cd web && npm run dev
