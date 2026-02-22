#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: setup-libvirt-matrix-vms.sh [options]

Provision libvirt VM lanes required by an offline replay matrix.

Options:
  --matrix <path>            Matrix config JSON path (default: offline/matrix.yaml)
  --controller <path>        jcs-offline-replay binary (auto-build if omitted)
  --image-dir <dir>          Base image directory (default: $HOME/vm-images)
  --pool-dir <dir>           VM disk/seed directory (default: /var/lib/libvirt/images/jcs-offline)
  --ssh-pubkey <path>        SSH public key for root login in guests (auto-detect if omitted)
  --memory-mb <int>          Memory per VM in MB (default: 4096)
  --vcpus <int>              vCPU count per VM (default: 2)
  --disk-size <size>         Optional qemu-img resize target (example: 30G)
  --network <name>           Libvirt network name (default: default)
  --install-ubuntu-deps      Install Ubuntu host deps via apt and enable libvirtd
  --recreate                 Recreate existing VM domains/disks for matrix nodes
  --no-hosts-update          Do not update /etc/hosts with VM hostname/IP
  -h, --help                 Show this help

Base image defaults (override via env vars):
  debian12-vm            -> $JCS_VM_IMAGE_DEBIAN12 or <image-dir>/debian12.qcow2
  ubuntu2204-vm-ga       -> $JCS_VM_IMAGE_UBUNTU2204_GA or <image-dir>/ubuntu2204-ga.qcow2 or <image-dir>/ubuntu2204.qcow2
  ubuntu2204-vm-hwe      -> $JCS_VM_IMAGE_UBUNTU2204_HWE or <image-dir>/ubuntu2204-hwe.qcow2 or <image-dir>/ubuntu2204.qcow2
  fedora40-vm            -> $JCS_VM_IMAGE_FEDORA40 or <image-dir>/fedora40.qcow2
  rocky9-vm              -> $JCS_VM_IMAGE_ROCKY9 or <image-dir>/rocky9.qcow2
  lts-legacy-kernel-vm   -> $JCS_VM_IMAGE_UBUNTU2204_LEGACY or <image-dir>/ubuntu2204-legacy.qcow2 or <image-dir>/ubuntu2204.qcow2
  debian12-vm-arm64      -> $JCS_VM_IMAGE_DEBIAN12_ARM64 or <image-dir>/debian12-arm64.qcow2 or <image-dir>/debian12.qcow2
  ubuntu2204-vm-ga-arm64 -> $JCS_VM_IMAGE_UBUNTU2204_GA_ARM64 or <image-dir>/ubuntu2204-ga-arm64.qcow2 or <image-dir>/ubuntu2204-arm64.qcow2 or <image-dir>/ubuntu2204.qcow2
  ubuntu2204-vm-hwe-arm64-> $JCS_VM_IMAGE_UBUNTU2204_HWE_ARM64 or <image-dir>/ubuntu2204-hwe-arm64.qcow2 or <image-dir>/ubuntu2204-arm64.qcow2 or <image-dir>/ubuntu2204.qcow2
  fedora40-vm-arm64      -> $JCS_VM_IMAGE_FEDORA40_ARM64 or <image-dir>/fedora40-arm64.qcow2 or <image-dir>/fedora40.qcow2
  rocky9-vm-arm64        -> $JCS_VM_IMAGE_ROCKY9_ARM64 or <image-dir>/rocky9-arm64.qcow2 or <image-dir>/rocky9.qcow2
  lts-legacy-kernel-vm-arm64 -> $JCS_VM_IMAGE_UBUNTU2204_LEGACY_ARM64 or <image-dir>/ubuntu2204-legacy-arm64.qcow2 or <image-dir>/ubuntu2204-arm64.qcow2 or <image-dir>/ubuntu2204.qcow2

Notes:
  - This script never downloads guest images; provide them locally first.
  - Requires sudo for system libvirt and /etc/hosts updates.
USAGE
}

MATRIX="offline/matrix.yaml"
CONTROLLER=""
IMAGE_DIR="${HOME}/vm-images"
POOL_DIR="/var/lib/libvirt/images/jcs-offline"
SSH_PUBKEY=""
MEMORY_MB="4096"
VCPUS="2"
DISK_SIZE=""
LIBVIRT_NETWORK="default"
INSTALL_UBUNTU_DEPS=0
RECREATE=0
UPDATE_HOSTS=1
LIBVIRT_URI="${LIBVIRT_URI:-qemu:///system}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --matrix)
      MATRIX="$2"
      shift 2
      ;;
    --controller)
      CONTROLLER="$2"
      shift 2
      ;;
    --image-dir)
      IMAGE_DIR="$2"
      shift 2
      ;;
    --pool-dir)
      POOL_DIR="$2"
      shift 2
      ;;
    --ssh-pubkey)
      SSH_PUBKEY="$2"
      shift 2
      ;;
    --memory-mb)
      MEMORY_MB="$2"
      shift 2
      ;;
    --vcpus)
      VCPUS="$2"
      shift 2
      ;;
    --disk-size)
      DISK_SIZE="$2"
      shift 2
      ;;
    --network)
      LIBVIRT_NETWORK="$2"
      shift 2
      ;;
    --install-ubuntu-deps)
      INSTALL_UBUNTU_DEPS=1
      shift
      ;;
    --recreate)
      RECREATE=1
      shift
      ;;
    --no-hosts-update)
      UPDATE_HOSTS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 2
  fi
}

run_virsh() {
  sudo virsh -c "$LIBVIRT_URI" "$@"
}

run_virt_install() {
  sudo virt-install --connect "$LIBVIRT_URI" "$@"
}

choose_ssh_key() {
  if [[ -n "$SSH_PUBKEY" ]]; then
    if [[ ! -f "$SSH_PUBKEY" ]]; then
      echo "ssh public key not found: $SSH_PUBKEY" >&2
      exit 2
    fi
    return
  fi

  for key in "$HOME/.ssh/id_ed25519.pub" "$HOME/.ssh/id_rsa.pub"; do
    if [[ -f "$key" ]]; then
      SSH_PUBKEY="$key"
      return
    fi
  done

  echo "no ssh public key found; pass --ssh-pubkey <path>" >&2
  exit 2
}

resolve_base_image() {
  local node_id="$1"
  local distro="$2"
  local image=""

  first_existing() {
    local candidate=""
    for candidate in "$@"; do
      if [[ -n "$candidate" && -f "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done
    return 1
  }

  case "$node_id" in
    debian12-vm)
      image="${JCS_VM_IMAGE_DEBIAN12:-${IMAGE_DIR}/debian12.qcow2}"
      ;;
    ubuntu2204-vm-ga)
      image="${JCS_VM_IMAGE_UBUNTU2204_GA:-}"
      if [[ -z "$image" ]]; then
        if [[ -f "${IMAGE_DIR}/ubuntu2204-ga.qcow2" ]]; then
          image="${IMAGE_DIR}/ubuntu2204-ga.qcow2"
        else
          image="${IMAGE_DIR}/ubuntu2204.qcow2"
        fi
      fi
      ;;
    ubuntu2204-vm-hwe)
      image="${JCS_VM_IMAGE_UBUNTU2204_HWE:-}"
      if [[ -z "$image" ]]; then
        if [[ -f "${IMAGE_DIR}/ubuntu2204-hwe.qcow2" ]]; then
          image="${IMAGE_DIR}/ubuntu2204-hwe.qcow2"
        else
          image="${IMAGE_DIR}/ubuntu2204.qcow2"
        fi
      fi
      ;;
    fedora40-vm)
      image="${JCS_VM_IMAGE_FEDORA40:-${IMAGE_DIR}/fedora40.qcow2}"
      ;;
    rocky9-vm)
      image="${JCS_VM_IMAGE_ROCKY9:-${IMAGE_DIR}/rocky9.qcow2}"
      ;;
    lts-legacy-kernel-vm)
      image="${JCS_VM_IMAGE_UBUNTU2204_LEGACY:-}"
      if [[ -z "$image" ]]; then
        if [[ -f "${IMAGE_DIR}/ubuntu2204-legacy.qcow2" ]]; then
          image="${IMAGE_DIR}/ubuntu2204-legacy.qcow2"
        else
          image="${IMAGE_DIR}/ubuntu2204.qcow2"
        fi
      fi
      ;;
    debian12-vm-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_DEBIAN12_ARM64:-}" \
        "${IMAGE_DIR}/debian12-arm64.qcow2" \
        "${IMAGE_DIR}/debian12.qcow2" || true)"
      ;;
    ubuntu2204-vm-ga-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_UBUNTU2204_GA_ARM64:-}" \
        "${IMAGE_DIR}/ubuntu2204-ga-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204.qcow2" || true)"
      ;;
    ubuntu2204-vm-hwe-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_UBUNTU2204_HWE_ARM64:-}" \
        "${IMAGE_DIR}/ubuntu2204-hwe-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204.qcow2" || true)"
      ;;
    fedora40-vm-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_FEDORA40_ARM64:-}" \
        "${IMAGE_DIR}/fedora40-arm64.qcow2" \
        "${IMAGE_DIR}/fedora40.qcow2" || true)"
      ;;
    rocky9-vm-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_ROCKY9_ARM64:-}" \
        "${IMAGE_DIR}/rocky9-arm64.qcow2" \
        "${IMAGE_DIR}/rocky9.qcow2" || true)"
      ;;
    lts-legacy-kernel-vm-arm64)
      image="$(first_existing \
        "${JCS_VM_IMAGE_UBUNTU2204_LEGACY_ARM64:-}" \
        "${IMAGE_DIR}/ubuntu2204-legacy-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204-arm64.qcow2" \
        "${IMAGE_DIR}/ubuntu2204.qcow2" || true)"
      ;;
    *)
      case "$distro" in
        debian-12) image="${IMAGE_DIR}/debian12.qcow2" ;;
        ubuntu-22.04) image="${IMAGE_DIR}/ubuntu2204.qcow2" ;;
        fedora-40) image="${IMAGE_DIR}/fedora40.qcow2" ;;
        rocky-9) image="${IMAGE_DIR}/rocky9.qcow2" ;;
      esac
      ;;
  esac

  if [[ -z "$image" || ! -f "$image" ]]; then
    echo "missing base image for node=${node_id} distro=${distro}; looked for: ${image}" >&2
    exit 2
  fi
  printf '%s\n' "$image"
}

make_seed_iso() {
  local domain="$1"
  local seed_iso="$2"
  local pubkey
  pubkey="$(cat "$SSH_PUBKEY")"
  local tmp
  tmp="$(mktemp -d)"

  cat > "${tmp}/user-data" <<EOF
#cloud-config
hostname: ${domain}
manage_etc_hosts: true
disable_root: false
ssh_pwauth: false
users:
  - name: root
    ssh_authorized_keys:
      - ${pubkey}
runcmd:
  - systemctl enable --now qemu-guest-agent || true
EOF

  cat > "${tmp}/meta-data" <<EOF
instance-id: ${domain}
local-hostname: ${domain}
EOF

  cloud-localds "${tmp}/seed.iso" "${tmp}/user-data" "${tmp}/meta-data"
  sudo mv "${tmp}/seed.iso" "$seed_iso"
  sudo chmod 0644 "$seed_iso"
  rm -rf "$tmp"
}

domain_exists() {
  local domain="$1"
  run_virsh dominfo "$domain" >/dev/null 2>&1
}

wait_for_domain_ip() {
  local domain="$1"
  local ip=""
  for _ in $(seq 1 90); do
    ip="$(run_virsh domifaddr "$domain" --source lease 2>/dev/null | awk '/ipv4/ {print $4}' | cut -d/ -f1 | head -n1)"
    if [[ -n "$ip" ]]; then
      printf '%s\n' "$ip"
      return 0
    fi
    sleep 2
  done
  return 1
}

update_hosts_entry() {
  local domain="$1"
  local ip="$2"
  sudo sed -i "/[[:space:]]${domain}\$/d" /etc/hosts
  echo "${ip} ${domain}" | sudo tee -a /etc/hosts >/dev/null
}

wait_for_ssh() {
  local target="$1"
  local opts=( -n -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new )
  for _ in $(seq 1 90); do
    if ssh "${opts[@]}" "$target" true >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

ensure_snapshot_cold() {
  local domain="$1"
  local snapshot="$2"
  local ssh_target="$3"

  ssh -n -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new "$ssh_target" \
    "nohup sh -c 'systemctl poweroff || shutdown -h now || poweroff' >/dev/null 2>&1 &" >/dev/null 2>&1 || true

  for _ in $(seq 1 60); do
    if [[ "$(run_virsh domstate "$domain" | tr -d '[:space:]')" == "shutoff" ]]; then
      break
    fi
    sleep 2
  done
  if [[ "$(run_virsh domstate "$domain" | tr -d '[:space:]')" != "shutoff" ]]; then
    run_virsh destroy "$domain" >/dev/null 2>&1 || true
  fi

  run_virsh snapshot-delete "$domain" "$snapshot" >/dev/null 2>&1 || true
  run_virsh snapshot-create-as "$domain" "$snapshot" "offline replay baseline" --atomic >/dev/null
}

create_or_update_domain() {
  local node_id="$1"
  local domain="$2"
  local distro="$3"
  local snapshot="$4"
  local ssh_target="$5"
  local base_image="$6"
  local disk_path="${POOL_DIR}/${domain}.qcow2"
  local seed_iso="${POOL_DIR}/${domain}-seed.iso"

  if domain_exists "$domain" && [[ "$RECREATE" -eq 1 ]]; then
    echo "[setup] recreate requested: ${domain}"
    run_virsh destroy "$domain" >/dev/null 2>&1 || true
    run_virsh undefine "$domain" --nvram >/dev/null 2>&1 || run_virsh undefine "$domain" >/dev/null 2>&1 || true
    sudo rm -f "$disk_path" "$seed_iso"
  fi

  if ! domain_exists "$domain"; then
    echo "[setup] creating domain: ${domain} (node=${node_id}, distro=${distro})"
    sudo qemu-img convert -O qcow2 "$base_image" "$disk_path"
    if [[ -n "$DISK_SIZE" ]]; then
      sudo qemu-img resize "$disk_path" "$DISK_SIZE" >/dev/null
    fi
    sudo chmod 0644 "$disk_path"

    make_seed_iso "$domain" "$seed_iso"

    run_virt_install \
      --name "$domain" \
      --memory "$MEMORY_MB" \
      --vcpus "$VCPUS" \
      --cpu host-passthrough \
      --import \
      --disk "path=${disk_path},format=qcow2,bus=virtio" \
      --disk "path=${seed_iso},device=cdrom" \
      --network "network=${LIBVIRT_NETWORK},model=virtio" \
      --graphics none \
      --video none \
      --os-variant detect=on,require=off \
      --noautoconsole >/dev/null
  else
    echo "[setup] domain already exists, reusing: ${domain}"
  fi

  run_virsh start "$domain" >/dev/null 2>&1 || true

  local ip=""
  if ip="$(wait_for_domain_ip "$domain")"; then
    echo "[setup] ${domain} ip=${ip}"
    if [[ "$UPDATE_HOSTS" -eq 1 ]]; then
      update_hosts_entry "$domain" "$ip"
    fi
  else
    echo "[setup] could not resolve DHCP IP for ${domain}; relying on existing DNS/hosts" >&2
  fi

  if ! wait_for_ssh "$ssh_target"; then
    echo "ssh not reachable for ${domain} via ${ssh_target}" >&2
    exit 2
  fi

  ensure_snapshot_cold "$domain" "$snapshot" "$ssh_target"
  run_virsh start "$domain" >/dev/null 2>&1 || true
  if ! wait_for_ssh "$ssh_target"; then
    echo "ssh not reachable after snapshot for ${domain} via ${ssh_target}" >&2
    exit 2
  fi
  echo "[setup] ready: ${domain} snapshot=${snapshot} ssh=${ssh_target}"
}

if [[ "$INSTALL_UBUNTU_DEPS" -eq 1 ]]; then
  echo "[setup] installing Ubuntu host dependencies"
  require_cmd sudo
  sudo apt-get update
  sudo apt-get install -y qemu-kvm libvirt-daemon-system libvirt-clients virtinst cloud-image-utils qemu-utils
  sudo systemctl enable --now libvirtd
  sudo virsh -c "$LIBVIRT_URI" net-start "$LIBVIRT_NETWORK" >/dev/null 2>&1 || true
  sudo virsh -c "$LIBVIRT_URI" net-autostart "$LIBVIRT_NETWORK" >/dev/null 2>&1 || true
fi

for cmd in sudo go python3 tar virsh virt-install qemu-img cloud-localds ssh awk cut grep; do
  require_cmd "$cmd"
done

choose_ssh_key
echo "[setup] repo: ${ROOT}"
echo "[setup] matrix: ${MATRIX}"
echo "[setup] image-dir: ${IMAGE_DIR}"
echo "[setup] pool-dir: ${POOL_DIR}"
echo "[setup] ssh-pubkey: ${SSH_PUBKEY}"
echo "[setup] libvirt-uri: ${LIBVIRT_URI}"
echo "[setup] network: ${LIBVIRT_NETWORK}"

if [[ -z "$CONTROLLER" ]]; then
  tmp_ctl="$(mktemp -d)"
  trap 'rm -rf "$tmp_ctl"' EXIT
  CONTROLLER="${tmp_ctl}/jcs-offline-replay"
  echo "[setup] building controller: ${CONTROLLER}"
  CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$CONTROLLER" ./cmd/jcs-offline-replay
fi
if [[ ! -x "$CONTROLLER" ]]; then
  echo "controller is not executable: ${CONTROLLER}" >&2
  exit 2
fi

sudo mkdir -p "$POOL_DIR"
sudo chmod 0755 "$POOL_DIR"

tmp_data="$(mktemp -d)"
trap 'rm -rf "$tmp_data"' EXIT
matrix_json="${tmp_data}/matrix.json"
vm_tsv="${tmp_data}/vms.tsv"

"$CONTROLLER" inspect-matrix --matrix "$MATRIX" > "$matrix_json"

python3 - "$matrix_json" "$vm_tsv" <<'PY'
import json,sys
matrix=json.load(open(sys.argv[1],encoding='utf-8'))
out=sys.argv[2]
rows=[]
for n in matrix.get("nodes",[]):
    if n.get("mode")!="vm":
        continue
    replay=(n.get("runner") or {}).get("replay") or []
    env=(n.get("runner") or {}).get("env") or {}
    domain=replay[1] if len(replay)>1 else ""
    snapshot=replay[2] if len(replay)>2 else "snapshot-cold"
    ssh_target=env.get("JCS_VM_SSH_TARGET", f"root@{domain}")
    rows.append((n.get("id",""), domain, n.get("distro",""), n.get("kernel_family",""), snapshot, ssh_target))
with open(out,"w",encoding="utf-8") as f:
    for r in rows:
        f.write("\t".join(r)+"\n")
print(f"vm_nodes={len(rows)}")
PY

if [[ ! -s "$vm_tsv" ]]; then
  echo "matrix has no VM nodes: ${MATRIX}" >&2
  exit 2
fi

while IFS=$'\t' read -r node_id domain distro kernel_family snapshot ssh_target; do
  [[ -z "$node_id" ]] && continue
  if [[ -z "$domain" ]]; then
    echo "vm node has empty domain: ${node_id}" >&2
    exit 2
  fi
  if [[ -z "$snapshot" ]]; then
    snapshot="snapshot-cold"
  fi
  if [[ -z "$ssh_target" ]]; then
    ssh_target="root@${domain}"
  fi
  base_image="$(resolve_base_image "$node_id" "$distro")"
  echo "[setup] node=${node_id} domain=${domain} base-image=${base_image}"
  create_or_update_domain "$node_id" "$domain" "$distro" "$snapshot" "$ssh_target" "$base_image"
done < "$vm_tsv"

echo "[setup] verifying vm lanes"
while IFS=$'\t' read -r node_id domain _kernel _family snapshot ssh_target; do
  [[ -z "$node_id" ]] && continue
  run_virsh dominfo "$domain" >/dev/null
  run_virsh snapshot-list --name "$domain" | grep -Fx "$snapshot" >/dev/null
  ssh -n -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new "$ssh_target" true >/dev/null
  echo "[setup] verified: ${node_id} -> ${domain}"
done < "$vm_tsv"

echo "[setup] VM provisioning complete."
echo "[setup] next: ./offline/scripts/cold-replay-preflight.sh --matrix ${MATRIX}"
