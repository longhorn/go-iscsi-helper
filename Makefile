MACHINE := longhorn
# Define the target platforms that can be used across the ecosystem.
DEFAULT_PLATFORMS := linux/amd64,linux/arm64

export SRC_BRANCH := master
export SRC_TAG := $(shell git tag --points-at HEAD | head -n 1)

.PHONY: validate test ci
validate:
	docker buildx build --build-arg SRC_BRANCH="$(SRC_BRANCH)" --build-arg SRC_TAG="$(SRC_TAG)" --target validate -f Dockerfile .

test:
	docker buildx build --build-arg SRC_BRANCH="$(SRC_BRANCH)" --build-arg SRC_TAG="$(SRC_TAG)" --target base -t go-iscsi-helper-test -f Dockerfile .
	docker run --privileged -v /dev:/host/dev -v /proc:/host/proc go-iscsi-helper-test ./scripts/test

ci:
	docker buildx build --build-arg SRC_BRANCH="$(SRC_BRANCH)" --build-arg SRC_TAG="$(SRC_TAG)" --target ci-artifacts --output type=local,dest=. -f Dockerfile .

.PHONY: buildx-machine
buildx-machine:
	@docker buildx create --name=$(MACHINE) --platform=$(DEFAULT_PLATFORMS) 2>/dev/null || true
	docker buildx inspect $(MACHINE)

.DEFAULT_GOAL := ci
