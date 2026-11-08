#!/bin/sh
# Runs go vet and go test -race as a normal user inside a golang container, the
# way CI does. Tests that run as root pass permission checks that fail for
# everyone else.
#
#   git archive HEAD | tar -x -C /tmp/cc-src
#   docker run --rm --name cc-go-test --cpus 4 --memory 5g -v /tmp/cc-src:/src \
#     -v "$PWD/testing/go-test-nonroot.sh:/run.sh:ro" golang:1.27 sh /run.sh
#
# It makes /src writable for everyone, so hand it a copy, not the checkout.
# With the argument desktop, as in `sh /run.sh desktop`, it checks the desktop
# app's build of the server instead of the container's.
set -u
tags=${1:-}

id ci >/dev/null 2>&1 || adduser --disabled-password --gecos "" ci >/dev/null
chmod -R a+rwX /src
# Root fills the module cache, which the test user only reads.
(cd /src/backend && go mod download) || exit 1

echo "export GOFLAGS=-tags=$tags" > /tmp/as-ci.sh
cat >> /tmp/as-ci.sh <<'EOF'
export HOME=/tmp/ci-home GOCACHE=/tmp/ci-cache GOMODCACHE=/go/pkg/mod PATH=/usr/local/go/bin:$PATH
mkdir -p "$HOME"
cd /src/backend || exit 1
go vet ./... || exit 1
go test -race ./... > /tmp/test.out 2>&1
status=$?
grep -E '^(FAIL|--- FAIL|panic)' /tmp/test.out | head -40
echo "packages ok: $(grep -c '^ok' /tmp/test.out), failed: $(grep -c '^FAIL' /tmp/test.out)"
exit $status
EOF
su ci -s /bin/sh /tmp/as-ci.sh
