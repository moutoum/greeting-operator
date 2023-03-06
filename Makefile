CLUSTER_NAME ?= edb-cluster
PORT ?= 80

build:
	go build

BIND ?= ":80"
run-greeting:
	go run cmd/greeting-server/main.go -bind $(BIND)

greeting-image:
	docker build -t greeting:latest -f greeting.Dockerfile .

operator-image:
	docker build -t greeting-operator:latest -f operator.Dockerfile .

images: greeting-image operator-image

import-images: images
	k3d image import -c $(CLUSTER_NAME) greeting:latest greeting-operator:latest

start: stop
	k3d cluster create $(CLUSTER_NAME) -p "$(PORT):80@loadbalancer" --k3s-arg "--disable=traefik@server:*"
	make import-images
	kubectl apply -f k8s

stop:
	k3d cluster delete $(CLUSTER_NAME)