# syntax=docker/dockerfile:1.22.0
FROM golangci/golangci-lint:v2.12.2 AS golangci-lint
FROM registry.suse.com/bci/golang:1.25 AS base

ARG TARGETARCH
ARG http_proxy
ARG https_proxy

ENV ARCH=${TARGETARCH}
ENV GOFLAGS=-mod=vendor

ARG SRC_BRANCH=master
ARG SRC_TAG

# Install packages
RUN zypper update -y && \
    zypper -n install glibc gcc cmake curl awk xsltproc docbook-xsl-stylesheets open-iscsi e2fsprogs jq && \
    rm -rf /var/cache/zypp/*

# needed for ${!var} substitution
RUN rm -f /bin/sh && ln -s /bin/bash /bin/sh

# Copy golangci-lint binary from official image
COPY --from=golangci-lint /usr/bin/golangci-lint /usr/local/bin/golangci-lint

RUN git clone https://github.com/longhorn/dep-versions.git -b ${SRC_BRANCH} /usr/src/dep-versions && \
    cd /usr/src/dep-versions && \
    if [ -n "${SRC_TAG}" ] && git show-ref --tags ${SRC_TAG} > /dev/null 2>&1; then \
        echo "Checking out tag ${SRC_TAG}"; \
        cd /usr/src/dep-versions && git checkout tags/${SRC_TAG}; \
    fi

# Build liblonghorn
RUN export REPO_OVERRIDE="" && \
    export COMMIT_ID_OVERRIDE="" && \
    bash /usr/src/dep-versions/scripts/build-liblonghorn.sh "${REPO_OVERRIDE}" "${COMMIT_ID_OVERRIDE}"

# Build tgt
RUN export REPO_OVERRIDE="" && \
    export COMMIT_ID_OVERRIDE="" && \
    bash /usr/src/dep-versions/scripts/build-tgt.sh "${REPO_OVERRIDE}" "${COMMIT_ID_OVERRIDE}"

WORKDIR /go/src/github.com/longhorn/go-iscsi-helper
COPY . .

FROM base AS validate
RUN ./scripts/validate && touch /validate.done

FROM scratch AS ci-artifacts
COPY --from=validate /validate.done /validate.done
