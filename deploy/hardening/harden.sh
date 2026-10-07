#!/usr/bin/env bash
#
# harden.sh — Apply the official k3s CIS Hardening Guide to a k3s SERVER node.
# Source of truth: https://docs.k3s.io/security/hardening-guide (verified 2026-06).
#
# What it does (idempotent — safe to re-run):
#   1. Sets the CIS-recommended kernel parameters (sysctl).
#   2. Writes the API server audit policy + creates a 0700 log dir.
#   3. Writes the admission-control config (Pod Security Admission + EventRateLimit).
#   4. Drops a k3s server config fragment into /etc/rancher/k3s/config.yaml.d/
#      (a drop-in, so it never clobbers your existing config.yaml).
#   5. Installs NetworkPolicies via the auto-deploy manifests dir, including the
#      carve-outs k3s needs (DNS, metrics-server, Traefik) so the cluster keeps working.
#   6. Tightens PKI cert permissions to 600 (CIS 1.1.20).
#   7. Restarts k3s, waits for the API, then disables default SA token automount
#      in the built-in namespaces (CIS 5.1.5).
#
# Run as root ON THE SERVER NODE:  sudo ./harden.sh
# Preview without changing anything: sudo ./harden.sh --dry-run
#
set -euo pipefail

# ---- Tunables (override via flags or env) -----------------------------------
ENFORCE_LEVEL="${ENFORCE_LEVEL:-restricted}"   # restricted | baseline
PSA_EXEMPT_NS="${PSA_EXEMPT_NS:-kube-system}"  # comma-sep namespaces exempt from PSA
APP_NAMESPACES="${APP_NAMESPACES:-agennext}"    # Autonomyx workload namespace
DRY_RUN=0
DO_RESTART=1

K3S_DIR="/var/lib/rancher/k3s/server"
CFG_DROPIN_DIR="/etc/rancher/k3s/config.yaml.d"

usage() { grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0; }
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --no-restart) DO_RESTART=0 ;;
    --enforce=*) ENFORCE_LEVEL="${arg#*=}" ;;
    --exempt-ns=*) PSA_EXEMPT_NS="${arg#*=}" ;;
    --app-ns=*) APP_NAMESPACES="${arg#*=}" ;;
    -h|--help) usage ;;
    *) echo "Unknown arg: $arg" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[1;34m[harden]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

# write_file <path> <mode> <<<content via stdin>
write_file() {
  local path="$1" mode="$2" tmp
  if [[ "$DRY_RUN" == 1 ]]; then
    log "DRY-RUN would write $path (mode $mode):"; sed 's/^/    | /'; return
  fi
  mkdir -p "$(dirname "$path")"
  tmp="$(mktemp)"; cat > "$tmp"
  install -m "$mode" "$tmp" "$path"; rm -f "$tmp"
  log "wrote $path (mode $mode)"
}

run() { if [[ "$DRY_RUN" == 1 ]]; then log "DRY-RUN: $*"; else eval "$@"; fi; }

# ---- Preflight --------------------------------------------------------------
[[ "$DRY_RUN" == 1 || "$EUID" -eq 0 ]] || die "Run as root (sudo). Use --dry-run to preview."
command -v k3s >/dev/null 2>&1 || die "k3s not found in PATH — run this on a k3s server node."
[[ -d "/var/lib/rancher/k3s" ]] || warn "/var/lib/rancher/k3s missing — is this a fresh node? Continuing."

K3S_VER="$(k3s --version 2>/dev/null | sed -n 's/^k3s version v\([0-9.]*\).*/\1/p')"
MINOR="$(echo "${K3S_VER:-0.0.0}" | cut -d. -f2)"
log "Detected k3s v${K3S_VER:-unknown} (minor=${MINOR:-?}), enforce=${ENFORCE_LEVEL}"
if [[ -n "$MINOR" && "$MINOR" -lt 25 ]]; then
  warn "k3s <v1.25 uses PodSecurityPolicy, removed upstream. This script targets PSA (v1.25+)."
  warn "See references/legacy-psp.md for the PSP path before proceeding."
fi

TLS_CIPHERS="TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"

# Version-banded args (per official guide): v1.29+ vs v1.25–v1.28.
# service-account-extend-token-expiration doesn't exist pre-v1.29 and would
# prevent the apiserver from starting.
GC_THRESHOLD=100
SA_TOKEN_ARG=$'\n  - "service-account-extend-token-expiration=false"'
if [[ -n "$MINOR" && "$MINOR" -ge 25 && "$MINOR" -lt 29 ]]; then
  GC_THRESHOLD=10
  SA_TOKEN_ARG=""
fi

# ---- 1. Kernel parameters (CIS host-level) ----------------------------------
write_file /etc/sysctl.d/90-kubelet.conf 644 <<'EOF'
vm.panic_on_oom=0
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
EOF
run "sysctl -p /etc/sysctl.d/90-kubelet.conf >/dev/null"

# ---- 2. Audit policy + log dir (CIS 1.2.22–1.2.25) --------------------------
run "mkdir -p -m 700 ${K3S_DIR}/logs"
write_file "${K3S_DIR}/audit.yaml" 600 <<'EOF'
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  # Don't log read-only noise.
  - level: None
    verbs: ["get", "list", "watch"]
  # Secrets/configmaps: metadata only — never log secret values.
  - level: Metadata
    resources:
      - group: ""
        resources: ["secrets", "configmaps"]
  # RBAC changes and pod exec/attach: full request+response.
  - level: RequestResponse
    resources:
      - group: "rbac.authorization.k8s.io"
        resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
      - group: ""
        resources: ["pods/exec", "pods/attach", "pods/portforward"]
  # Everything else: request metadata.
  - level: Metadata
    omitStages: ["RequestReceived"]
EOF

# ---- 3. Admission control: Pod Security Admission + EventRateLimit -----------
EXEMPT_YAML=""
IFS=',' read -ra _ns <<< "$PSA_EXEMPT_NS"
for n in "${_ns[@]}"; do EXEMPT_YAML+="$(printf '\n        - %s' "$n")"; done
write_file "${K3S_DIR}/psa.yaml" 600 <<EOF
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
  - name: PodSecurity
    configuration:
      apiVersion: pod-security.admission.config.k8s.io/v1
      kind: PodSecurityConfiguration
      defaults:
        enforce: "${ENFORCE_LEVEL}"
        enforce-version: "latest"
        audit: "${ENFORCE_LEVEL}"
        audit-version: "latest"
        warn: "${ENFORCE_LEVEL}"
        warn-version: "latest"
      exemptions:
        usernames: []
        runtimeClasses: []
        namespaces:${EXEMPT_YAML}
  - name: EventRateLimit
    configuration:
      apiVersion: eventratelimit.admission.k8s.io/v1alpha1
      kind: Configuration
      limits:
        - type: Namespace
          qps: 50
          burst: 100
          cacheSize: 2000
        - type: User
          qps: 10
          burst: 50
EOF

# ---- 4. k3s server config drop-in (never clobbers existing config.yaml) ------
write_file "${CFG_DROPIN_DIR}/90-cis-harden.yaml" 600 <<EOF
# Managed by k3s-harden. CIS Hardening Guide remediations for k3s v1.29+.
# This is a drop-in: it merges with /etc/rancher/k3s/config.yaml.
# NOTE: if your base config.yaml already sets any *-arg list below, k3s replaces
# (not appends) on conflict. Append instead by suffixing the key with '+'.
protect-kernel-defaults: true
secrets-encryption: true
kube-apiserver-arg:
  - "enable-admission-plugins=NodeRestriction,EventRateLimit"
  - "admission-control-config-file=${K3S_DIR}/psa.yaml"
  - "audit-log-path=${K3S_DIR}/logs/audit.log"
  - "audit-policy-file=${K3S_DIR}/audit.yaml"
  - "audit-log-maxage=30"
  - "audit-log-maxbackup=10"
  - "audit-log-maxsize=100"${SA_TOKEN_ARG}
kube-controller-manager-arg:
  - "terminated-pod-gc-threshold=${GC_THRESHOLD}"
kubelet-arg:
  - "streaming-connection-idle-timeout=5m"
  - "tls-cipher-suites=${TLS_CIPHERS}"
EOF

# ---- 5. NetworkPolicies (auto-deployed from manifests dir) -------------------
DNS_RULES=""
IFS=',' read -ra _appns <<< "$APP_NAMESPACES"
for ns in "${_appns[@]}"; do
  DNS_RULES+=$'\n'"$(cat <<EOF
---
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: ${ns}
spec:
  podSelector: {}
  policyTypes: ["Ingress"]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ${ns}
EOF
)"
done

write_file "${K3S_DIR}/manifests/cis-network-policies.yaml" 644 <<EOF
# CIS 5.3.2 — default-deny-ish intra-namespace isolation + required carve-outs.
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: kube-system
spec:
  podSelector: {}
  policyTypes: ["Ingress"]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
---
# DNS must stay reachable or the whole cluster breaks.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      k8s-app: kube-dns
  policyTypes: ["Ingress"]
  ingress:
    - ports:
        - { port: 53, protocol: TCP }
        - { port: 53, protocol: UDP }
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-metrics-server
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      k8s-app: metrics-server
  policyTypes: ["Ingress"]
  ingress: [{}]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-traefik
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: traefik
  policyTypes: ["Ingress"]
  ingress: [{}]${DNS_RULES}
EOF

# ---- 6. PKI cert permissions (CIS 1.1.20) -----------------------------------
if [[ -d "${K3S_DIR}/tls" ]]; then
  run "chmod -R 600 ${K3S_DIR}/tls/*.crt"
fi

# ---- 7. Restart k3s and apply post-start remediations -----------------------
if [[ "$DO_RESTART" == 1 ]]; then
  run "systemctl daemon-reload"
  run "systemctl restart k3s.service"
  if [[ "$DRY_RUN" == 0 ]]; then
    log "Waiting for API server..."
    for i in $(seq 1 60); do
      if k3s kubectl get --raw='/readyz' >/dev/null 2>&1; then break; fi
      sleep 2
    done
    # CIS 5.1.5 — disable default SA token automount in built-in namespaces.
    for ns in default kube-node-lease kube-public; do
      k3s kubectl patch serviceaccount default -n "$ns" \
        -p '{"automountServiceAccountToken": false}' >/dev/null 2>&1 \
        && log "patched default SA in $ns" || warn "could not patch SA in $ns"
    done
  fi
else
  warn "Skipping restart (--no-restart). Run: sudo systemctl daemon-reload && sudo systemctl restart k3s.service"
fi

log "Done. Verify with: sudo ./verify.sh"
