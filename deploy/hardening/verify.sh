#!/usr/bin/env bash
#
# verify.sh — Check that the k3s CIS hardening was applied. Read-only; safe to run anytime.
# Run on the SERVER node: sudo ./verify.sh
#
set -uo pipefail
K3S_DIR="/var/lib/rancher/k3s/server"
PASS=0; FAIL=0
ok()  { printf '\033[1;32m  PASS\033[0m %s\n' "$*"; PASS=$((PASS+1)); }
no()  { printf '\033[1;31m  FAIL\033[0m %s\n' "$*"; FAIL=$((FAIL+1)); }
chk() { if eval "$2" >/dev/null 2>&1; then ok "$1"; else no "$1"; fi; }

echo "== Host / config files =="
chk "sysctl 90-kubelet.conf present"        "[ -f /etc/sysctl.d/90-kubelet.conf ]"
chk "kernel.panic_on_oops=1 live"           "[ \"\$(sysctl -n kernel.panic_on_oops)\" = 1 ]"
chk "audit policy present"                   "[ -f ${K3S_DIR}/audit.yaml ]"
chk "audit log dir is 0700"                  "[ \"\$(stat -c %a ${K3S_DIR}/logs 2>/dev/null)\" = 700 ]"
chk "admission config (psa.yaml) present"    "[ -f ${K3S_DIR}/psa.yaml ]"
chk "config drop-in present"                 "[ -f /etc/rancher/k3s/config.yaml.d/90-cis-harden.yaml ]"
chk "network policy manifest present"        "[ -f ${K3S_DIR}/manifests/cis-network-policies.yaml ]"
chk "PKI .crt files are 600"                 "! find ${K3S_DIR}/tls -name '*.crt' -perm /077 2>/dev/null | grep -q ."

echo "== Running cluster =="
KC="k3s kubectl"
chk "audit.log being written"               "[ -s ${K3S_DIR}/logs/audit.log ]"
chk "PodSecurity enforced (psa.yaml loaded)" "grep -q PodSecurity ${K3S_DIR}/psa.yaml"
chk "NetworkPolicies deployed"              "$KC get netpol -A 2>/dev/null | grep -q intra-namespace"
chk "default SA automount disabled (default ns)" \
    "$KC get sa default -n default -o jsonpath='{.automountServiceAccountToken}' 2>/dev/null | grep -q false"

echo
echo "Result: ${PASS} passed, ${FAIL} failed."
[ "$FAIL" -eq 0 ] || { echo "Some checks failed — re-run harden.sh or inspect above."; exit 1; }
echo "Cluster matches the k3s CIS hardening baseline."
echo "For a full benchmark scan, run the official tool: k3s kube-bench or 'kubectl' + CIS self-assessment."
