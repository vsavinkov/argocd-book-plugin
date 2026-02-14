IMAGE_REPO ?= argocd-book-plugin
IMAGE_TAG  ?= latest
IMAGE      := $(IMAGE_REPO):$(IMAGE_TAG)

.PHONY: build test lint docker-build docker-push deploy clean

## Build the Go backend binary
build:
	cd backend && go build -o ../bin/server ./cmd/server

## Run all tests
test:
	cd backend && go test ./...

## Run linters
lint:
	cd backend && go vet ./...

## Build Docker image (multi-stage: UI + backend)
docker-build:
	docker build -t $(IMAGE) .

## Push Docker image
docker-push:
	docker push $(IMAGE)

## Deploy manifests to the current kubectl context
deploy:
	kubectl apply -f manifests/serviceaccount.yaml
	kubectl apply -f manifests/rbac.yaml
	kubectl apply -f manifests/deployment.yaml
	kubectl apply -f manifests/service.yaml
	@echo "---"
	@echo "Apply ArgoCD patches manually:"
	@echo "  kubectl patch cm argocd-cmd-params-cm -n argocd --patch-file manifests/argocd-patches/argocd-cmd-params-cm-patch.yaml"
	@echo "  kubectl patch cm argocd-cm -n argocd --patch-file manifests/argocd-patches/argocd-cm-patch.yaml"
	@echo "  Update argocd-server deployment with init container from manifests/argocd-patches/argocd-server-patch.yaml"

## Remove deployed resources
clean:
	kubectl delete -f manifests/service.yaml --ignore-not-found
	kubectl delete -f manifests/deployment.yaml --ignore-not-found
	kubectl delete -f manifests/rbac.yaml --ignore-not-found
	kubectl delete -f manifests/serviceaccount.yaml --ignore-not-found
