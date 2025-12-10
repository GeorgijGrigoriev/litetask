.PHONY: docker-build

IMAGE ?= litetask:latest

docker-build:
	docker build -t $(IMAGE) .
