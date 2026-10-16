.PHONY: test test-go test-api test-failover-local build push deploy lint

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

test-api:
	docker compose up -d
	WAIT_PORTS=8080,8070,8060,8050 ./scripts/wait-script.sh
	newman run --delay-request=100 --folder=success -e v1/postman/environment.json v1/postman/collection.json

test-failover-local:
	docker compose up -d
	WAIT_PORTS=8080,8070,8060,8050 ./scripts/wait-script.sh
	./scripts/test-compose.sh v1 ms-bonus 8050

