.PHONY: docker-build build-frontend dev

IMAGE ?= litetask:latest

docker-build:
	docker build -t $(IMAGE) .

build-frontend:
	cd web && npm run build

dev:
	cd web && npm run dev
