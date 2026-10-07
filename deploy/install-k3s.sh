#!/usr/bin/env bash
# Autonomyx / AGenNext Chat — one-pass installer for a single-node k3s VPS.
#
# CNCF-mature stack, no Docker:
#   Cloud Native Buildpacks (pack) builds an OCI image using Podman as the backend,
#   the image is imported straight into k3s' containerd (no registry),
#   then CRDs + Helm install the chart, and it smoke-tests itself.
#
# Run this ONCE on the VPS, from the repo root:
#   chmod +x deploy/install-k3s.sh && ./deploy/install-k3s.sh
#
# Override via env if needed: NS=, IMG=, TAG=, BUILDER=
set -euo pipefail

NS="${NS:-agennext}"
IMG="${IMG:-agennext-chat}"
TAG="${TAG:-0.1.0}"
# NOTE: confirm the current builder tag from buildpacks.io / paketo.io if this fails.
BUILDER="${BUILDER:-paketobuildpacks/builder-jammy-base}"

echo "==> 0. Preflight — required tools"
missing=0
for t in pack podman kubectl helm; do
  if ! command -v "$t" >/dev/null 2>&1; then echo "   MISSING: $t"; missing=1; fi
done
[ "$missing" = 0 ] || { echo "Install the missing tools, then re-run."; exit 1; }
[ -d deploy/helm/agennext-chat ] || { echo "Run from the repo root (deploy/helm not found)."; exit 1; }

echo "==> 1. Start Podman's Docker-compatible socket (pack uses it as its backend)"
systemctl --user enable --now podman.socket 2>/dev/null || true
export DOCKER_HOST="unix://${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/podman/podman.sock"

echo "==> 2. Build the image with Cloud Native Buildpacks (no Dockerfile needed)"
pack build "${IMG}:${TAG}" --builder "${BUILDER}"

echo "==> 3. Import the OCI image into k3s containerd (no registry)"
podman save "${IMG}:${TAG}" -o "/tmp/${IMG}.tar"
sudo k3s ctr images import "/tmp/${IMG}.tar"
rm -f "/tmp/${IMG}.tar"

echo "==> 4. Install the CRDs"
kubectl apply -k deploy/crds/

echo "==> 5. Install the chart (single-node overrides; locally-imported image)"
helm upgrade --install agennext-chat deploy/helm/agennext-chat \
  --namespace "${NS}" --create-namespace \
  --set image.repository="${IMG}" \
  --set image.tag="${TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set replicaCount=1 \
  --set podDisruptionBudget.enabled=false

echo "==> 6. Wait for rollout"
kubectl -n "${NS}" rollout status deploy/agennext-chat --timeout=180s
kubectl -n "${NS}" get pods -o wide

echo "==> 7. Smoke test (/healthz, /readyz, /v1/chat)"
kubectl -n "${NS}" port-forward svc/agennext-chat 8080:8080 >/tmp/agennext-pf.log 2>&1 &
pf=$!; sleep 4
curl -fsS http://127.0.0.1:8080/healthz && echo "   <- healthz OK"
curl -fsS http://127.0.0.1:8080/readyz  && echo "   <- readyz OK"
curl -fsS -X POST http://127.0.0.1:8080/v1/chat \
  -H 'content-type: application/json' \
  -d '{"id":"evt-1","source":"channel/web","type":"chat.message.v1","session":"s1","tenant":"acme","principal":"user-1","capability":"rag.retrieve","message":"What is your return policy?"}' || true
echo
kill "$pf" 2>/dev/null || true

echo
echo "==> DONE. agennext-chat is installed in namespace '${NS}'."
echo "    This is the loop core only — the reasoning model and real tools are stubs."
