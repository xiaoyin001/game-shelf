# 文件说明：本地与CI共用固定SDK、冻结Ubuntu库和锁文件headless shell；不改变宿主内核。
FROM node:22.23.3-bookworm-slim@sha256:3ed98443660ff48413b5af19ecefce3b9d50845a901a43420d47cc63a9fddbe9 AS node
FROM docker:29.1.3-cli@sha256:4fa0ee1f3a7e4354c4ea34558b6d4ee32859baf4973d4c8ccc8e7fe3dd730c04 AS dockercli
FROM golang:1.25.0-bookworm@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a AS sdk

# source保留真实GCC/CGO与race；不需要Docker socket或浏览器。
FROM sdk AS source
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm /opt/npm
RUN ln -s /opt/npm/bin/npm-cli.js /usr/local/bin/npm && ln -s /opt/npm/bin/npx-cli.js /usr/local/bin/npx
ENV CI=1 CGO_ENABLED=1 GOCACHE=/ci-cache/go-build GOMODCACHE=/ci-cache/go-mod NPM_CONFIG_CACHE=/ci-cache/npm
WORKDIR /workspace

# 只装实际运行的headless shell；签名和包哈希仍校验，历史快照有意不检查新鲜度期限。
FROM ubuntu:22.04@sha256:98510ac07bb4545e389be502a8918b385c018bfe4ceeb5ac7e0375c782268b04 AS playable
COPY --from=sdk /usr/local/go /usr/local/go
COPY --from=sdk /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm /opt/npm
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=dockercli /usr/local/libexec/docker/cli-plugins/docker-compose /usr/local/libexec/docker/cli-plugins/docker-compose
RUN ln -s /opt/npm/bin/npm-cli.js /usr/local/bin/npm && ln -s /opt/npm/bin/npx-cli.js /usr/local/bin/npx
ENV PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin CI=1 CGO_ENABLED=0 GOCACHE=/ci-cache/go-build GOMODCACHE=/ci-cache/go-mod NPM_CONFIG_CACHE=/ci-cache/npm PLAYWRIGHT_BROWSERS_PATH=/ms-playwright DEBIAN_FRONTEND=noninteractive
RUN printf 'deb [check-valid-until=no] https://snapshot.ubuntu.com/ubuntu/20261021T000000Z jammy main restricted universe multiverse\ndeb [check-valid-until=no] https://snapshot.ubuntu.com/ubuntu/20261021T000000Z jammy-updates main restricted universe multiverse\ndeb [check-valid-until=no] https://snapshot.ubuntu.com/ubuntu/20261021T000000Z jammy-security main restricted universe multiverse\n' > /etc/apt/sources.list
COPY package.json package-lock.json /opt/ci-browser/
RUN apt-get update && apt-get install -y --no-install-recommends git util-linux libstdc++6 && cd /opt/ci-browser && export NPM_CONFIG_CACHE=/tmp/npm-bootstrap && npm ci --no-audit --no-fund && npx playwright install --with-deps --only-shell chromium && rm -rf node_modules /tmp/npm-bootstrap /var/lib/apt/lists/*
WORKDIR /workspace
