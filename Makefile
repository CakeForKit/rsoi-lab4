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

