.PHONY: test test-go build push deploy lint

MODULES := lb-core ms-flight ms-ticket ms-bonus ms-gateway
NAMESPACE ?= rsoi
IMAGE_TAG ?= latest
DOCKER_REGISTRY ?= local

test: test-go

test-go:
	@for module in $(MODULES); do \
		echo "Running Go tests in $$module"; \
		(cd $$module && GOWORK=off go test ./... && GOWORK=off go vet ./...) || exit 1; \
	done

build:
	DOCKER_REGISTRY=$(DOCKER_REGISTRY) IMAGE_TAG=$(IMAGE_TAG) docker compose build

push:
	DOCKER_REGISTRY=$(DOCKER_REGISTRY) IMAGE_TAG=$(IMAGE_TAG) docker compose push ms-flight ms-ticket ms-bonus ms-gateway

lint:
	helm lint helm/postgres
	@for values in helm/microservice/values-ms-*.yaml; do \
		helm lint helm/microservice -f $$values; \
	done

deploy:
	helm upgrade --install postgres helm/postgres --namespace $(NAMESPACE) --create-namespace --wait --timeout 3m
	@for values in helm/microservice/values-ms-*.yaml; do \
		release=$${values##*/values-}; release=$${release%.yaml}; \
		helm upgrade --install $$release helm/microservice --namespace $(NAMESPACE) --wait --timeout 3m \
			-f $$values --set image.repository=$(DOCKER_REGISTRY)/rsoi-lab4-$$release --set image.tag=$(IMAGE_TAG); \
	done
