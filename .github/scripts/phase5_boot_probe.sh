#!/usr/bin/env bash
# Temporary Linux boot-acceptance probe for Phase 5.2.
#
# Ports spike 1.E attempt C onto ubuntu-latest: q35+smm, writable setup-mode
# OVMF varstore, swtpm, source media, blank target, deterministic guest MAC,
# slirp pcap, serial and monitor logs. Harness and setup failures exit
# non-zero. A completed negative diagnostic exits 0 after writing evidence.

set -euo pipefail

readonly GUEST_MAC='52:54:00:12:34:56'
readonly ONLINE_BOOT_SECONDS="${PHASE5_ONLINE_BOOT_SECONDS:-480}"
readonly RECOVERY_BOOT_SECONDS="${PHASE5_RECOVERY_BOOT_SECONDS:-300}"
readonly TARGET_DISK_GIB=8
readonly SEED_CONSUMPTION_BYTES=1048576
readonly APP_NAME='debug'
readonly SCHEMA_VERSION=1

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly REPO_ROOT="$repo_root"
readonly OUT_DIR="${PHASE5_BOOT_PROBE_DIR:-$REPO_ROOT/.phase5-boot-probe}"
readonly WORK_DIR="$OUT_DIR/work"
readonly LOG_DIR="$OUT_DIR/logs"
readonly PCAP_DIR="$OUT_DIR/pcaps"
readonly EVIDENCE_PATH="$OUT_DIR/evidence.json"
readonly CONFIG_PATH="$WORK_DIR/probe.yaml"
readonly SEEDED_IMG="$WORK_DIR/seeded.img"
readonly RESCUE_IMG="$WORK_DIR/rescue.img"
readonly SRC_QCOW="$WORK_DIR/src.qcow2"
readonly TARGET_QCOW="$WORK_DIR/target.qcow2"
readonly CLI="$REPO_ROOT/bin/incusos-builder"

OVMF_CODE=''
OVMF_VARS_TEMPLATE=''
OVMF_VARS=''
OVMF_FORMAT='raw'

classification='harness_failed'
image_version=''
accel='tcg'
online_seed_consumption='false'
harness_error=''
qemu_pid=''
swtpm_pidfile=''

mkdir -p "$WORK_DIR" "$LOG_DIR" "$PCAP_DIR"
: >"$LOG_DIR/online.serial.log"
: >"$LOG_DIR/online.monitor.log"
: >"$LOG_DIR/recovery.serial.log"
: >"$LOG_DIR/recovery.monitor.log"
: >"$PCAP_DIR/online.pcap"
: >"$PCAP_DIR/recovery.pcap"

file_size() {
  if [[ -f "$1" ]]; then
    stat -c '%s' "$1"
  else
    printf '0'
  fi
}

qcow_actual_size() {
  python3 - "$1" <<'PY'
import json, subprocess, sys
path = sys.argv[1]
try:
    raw = subprocess.check_output(["qemu-img", "info", "--output=json", path], text=True)
except (OSError, subprocess.CalledProcessError):
    print(0)
    raise SystemExit(0)
info = json.loads(raw)
print(int(info.get("actual-size") or 0))
PY
}

count_pcap_frames() {
  python3 - "$1" "$GUEST_MAC" <<'PY'
import pathlib, struct, sys

path = pathlib.Path(sys.argv[1])
mac_text = sys.argv[2]
want = bytes(int(part, 16) for part in mac_text.split(":"))
total = 0
guest = 0
if not path.is_file() or path.stat().st_size < 24:
    print(f"{total} {guest}")
    raise SystemExit(0)
data = path.read_bytes()
magic = data[:4]
if magic == b"\xd4\xc3\xb2\xa1":
    endian = "<"
elif magic == b"\xa1\xb2\xc3\xd4":
    endian = ">"
else:
    print(f"{total} {guest}")
    raise SystemExit(0)
offset = 24
header = struct.Struct(endian + "IIII")
while offset + header.size <= len(data):
    _ts_sec, _ts_usec, incl_len, _orig_len = header.unpack_from(data, offset)
    offset += header.size
    frame = data[offset:offset + incl_len]
    offset += incl_len
    if len(frame) < incl_len:
        break
    total += 1
    if len(frame) >= 12 and frame[6:12] == want:
        guest += 1
print(f"{total} {guest}")
PY
}

serial_contains() {
  local file="$1"
  local needle="$2"
  if [[ -f "$file" ]] && grep -F -q -- "$needle" "$file"; then
    printf 'true'
  else
    printf 'false'
  fi
}

disk_snapshot() {
  local path="$1"
  printf '%s %s' "$(file_size "$path")" "$(qcow_actual_size "$path")"
}

json_escape() {
  python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1" 2>/dev/null || printf '"%s"' "${1//\"/\\\"}"
}

write_stub_evidence() {
  local status="${1:-failed}"
  local error_json
  error_json="$(json_escape "${harness_error}")"
  cat >"$EVIDENCE_PATH" <<EOF
{"schema_version":${SCHEMA_VERSION},"probe":"phase5-boot-probe","status":"${status}","classification":"${classification}","harness_error":${error_json},"image":{"type":"raw","architecture":"x86_64","channel":"stable","application":"${APP_NAME}","version":null,"seeded_image":"${SEEDED_IMG}","rescue_image":"${RESCUE_IMG}"},"guest":{"machine":"q35,smm=on","accel":"${accel}","mac":"${GUEST_MAC}","firmware_code":"${OVMF_CODE}","firmware_vars_template":"${OVMF_VARS_TEMPLATE}"},"online_boot":{"timeout_seconds":${ONLINE_BOOT_SECONDS},"elapsed_seconds":0,"qemu_exit_code":null,"secure_boot_enrollment":{"enroll_message_seen":false,"enroll_success_seen":false,"secure_boot_refusal_seen":false},"source_disk":{"path":"${SRC_QCOW}","before_bytes":0,"after_bytes":0,"before_actual_bytes":0,"after_actual_bytes":0,"grew":false},"target_disk":{"path":"${TARGET_QCOW}","before_bytes":0,"after_bytes":0,"before_actual_bytes":0,"after_actual_bytes":0,"grew":false},"network":{"guest_mac":"${GUEST_MAC}","pcap":"${PCAP_DIR}/online.pcap","pcap_bytes":0,"total_frames":0,"guest_originated_frames":0,"guest_originated_observed":false},"serial":{"path":"${LOG_DIR}/online.serial.log","bytes":0},"monitor":{"path":"${LOG_DIR}/online.monitor.log","bytes":0},"seed_consumption_observed":false},"recovery_boot":{"reachable":false,"skipped_reason":${error_json},"timeout_seconds":${RECOVERY_BOOT_SECONDS},"elapsed_seconds":0,"qemu_exit_code":null,"rescue_data_detected":false,"update_sjson_acceptance":false,"update_json_acceptance":false,"network":{"pcap":"${PCAP_DIR}/recovery.pcap","pcap_bytes":0,"total_frames":0,"guest_originated_frames":0},"serial":{"path":"${LOG_DIR}/recovery.serial.log","bytes":0},"monitor":{"path":"${LOG_DIR}/recovery.monitor.log","bytes":0}}}
EOF
}

write_evidence() {
  local status="$1"
  local online_elapsed="${2:-0}"
  local online_qemu_exit="${3:-}"
  local recovery_reachable="${4:-false}"
  local recovery_skipped="${5:-}"
  local recovery_elapsed="${6:-0}"
  local recovery_qemu_exit="${7:-}"
  local src_before_bytes="${8:-0}"
  local src_before_actual="${9:-0}"
  local src_after_bytes="${10:-0}"
  local src_after_actual="${11:-0}"
  local tgt_before_bytes="${12:-0}"
  local tgt_before_actual="${13:-0}"
  local tgt_after_bytes="${14:-0}"
  local tgt_after_actual="${15:-0}"
  local online_pcap_bytes="${16:-0}"
  local online_total_frames="${17:-0}"
  local online_guest_frames="${18:-0}"
  local recovery_pcap_bytes="${19:-0}"
  local recovery_total_frames="${20:-0}"
  local recovery_guest_frames="${21:-0}"
  local enroll_msg="${22:-false}"
  local enroll_ok="${23:-false}"
  local sb_refusal="${24:-false}"
  local rescue_detected="${25:-false}"
  local update_sjson="${26:-false}"
  local update_json="${27:-false}"
  local src_grew='false'
  local tgt_grew='false'
  local guest_net='false'
  local seed_obs='false'

  if (( src_after_actual > src_before_actual || src_after_bytes > src_before_bytes )); then
    src_grew='true'
  fi
  if (( tgt_after_actual - tgt_before_actual >= SEED_CONSUMPTION_BYTES || tgt_after_bytes - tgt_before_bytes >= SEED_CONSUMPTION_BYTES )); then
    tgt_grew='true'
    seed_obs='true'
  fi
  if (( online_guest_frames > 0 )); then
    guest_net='true'
  fi
  online_seed_consumption="$seed_obs"

  if ! command -v python3 >/dev/null 2>&1; then
    write_stub_evidence "$status"
    return 0
  fi

  python3 - "$EVIDENCE_PATH" <<'PY'
import json, os, sys

path = sys.argv[1]
payload = {
    "schema_version": int(os.environ["EV_SCHEMA_VERSION"]),
    "probe": "phase5-boot-probe",
    "status": os.environ["EV_STATUS"],
    "classification": os.environ["EV_CLASSIFICATION"],
    "harness_error": os.environ.get("EV_HARNESS_ERROR") or None,
    "image": {
        "type": "raw",
        "architecture": "x86_64",
        "channel": "stable",
        "application": os.environ["EV_APP_NAME"],
        "version": os.environ.get("EV_IMAGE_VERSION") or None,
        "seeded_image": os.environ["EV_SEEDED"],
        "rescue_image": os.environ["EV_RESCUE"],
    },
    "guest": {
        "machine": "q35,smm=on",
        "accel": os.environ["EV_ACCEL"],
        "mac": os.environ["EV_GUEST_MAC"],
        "firmware_code": os.environ["EV_OVMF_CODE"],
        "firmware_vars_template": os.environ["EV_OVMF_VARS_TEMPLATE"],
    },
    "online_boot": {
        "timeout_seconds": int(os.environ["EV_ONLINE_TIMEOUT"]),
        "elapsed_seconds": int(os.environ["EV_ONLINE_ELAPSED"]),
        "qemu_exit_code": os.environ.get("EV_ONLINE_QEMU_EXIT") or None,
        "secure_boot_enrollment": {
            "enroll_message_seen": os.environ["EV_ENROLL_MSG"] == "true",
            "enroll_success_seen": os.environ["EV_ENROLL_OK"] == "true",
            "secure_boot_refusal_seen": os.environ["EV_SB_REFUSAL"] == "true",
        },
        "source_disk": {
            "path": os.environ["EV_SRC"],
            "before_bytes": int(os.environ["EV_SRC_BEFORE_BYTES"]),
            "after_bytes": int(os.environ["EV_SRC_AFTER_BYTES"]),
            "before_actual_bytes": int(os.environ["EV_SRC_BEFORE_ACTUAL"]),
            "after_actual_bytes": int(os.environ["EV_SRC_AFTER_ACTUAL"]),
            "grew": os.environ["EV_SRC_GREW"] == "true",
        },
        "target_disk": {
            "path": os.environ["EV_TGT"],
            "before_bytes": int(os.environ["EV_TGT_BEFORE_BYTES"]),
            "after_bytes": int(os.environ["EV_TGT_AFTER_BYTES"]),
            "before_actual_bytes": int(os.environ["EV_TGT_BEFORE_ACTUAL"]),
            "after_actual_bytes": int(os.environ["EV_TGT_AFTER_ACTUAL"]),
            "grew": os.environ["EV_TGT_GREW"] == "true",
        },
        "network": {
            "guest_mac": os.environ["EV_GUEST_MAC"],
            "pcap": os.environ["EV_ONLINE_PCAP"],
            "pcap_bytes": int(os.environ["EV_ONLINE_PCAP_BYTES"]),
            "total_frames": int(os.environ["EV_ONLINE_TOTAL_FRAMES"]),
            "guest_originated_frames": int(os.environ["EV_ONLINE_GUEST_FRAMES"]),
            "guest_originated_observed": os.environ["EV_GUEST_NET"] == "true",
        },
        "serial": {
            "path": os.environ["EV_ONLINE_SERIAL"],
            "bytes": int(os.environ["EV_ONLINE_SERIAL_BYTES"]),
        },
        "monitor": {
            "path": os.environ["EV_ONLINE_MONITOR"],
            "bytes": int(os.environ["EV_ONLINE_MONITOR_BYTES"]),
        },
        "seed_consumption_observed": os.environ["EV_SEED_OBS"] == "true",
    },
    "recovery_boot": {
        "reachable": os.environ["EV_RECOVERY_REACHABLE"] == "true",
        "skipped_reason": os.environ.get("EV_RECOVERY_SKIPPED") or None,
        "timeout_seconds": int(os.environ["EV_RECOVERY_TIMEOUT"]),
        "elapsed_seconds": int(os.environ["EV_RECOVERY_ELAPSED"]),
        "qemu_exit_code": os.environ.get("EV_RECOVERY_QEMU_EXIT") or None,
        "rescue_data_detected": os.environ["EV_RESCUE_DETECTED"] == "true",
        "update_sjson_acceptance": os.environ["EV_UPDATE_SJSON"] == "true",
        "update_json_acceptance": os.environ["EV_UPDATE_JSON"] == "true",
        "network": {
            "pcap": os.environ["EV_RECOVERY_PCAP"],
            "pcap_bytes": int(os.environ["EV_RECOVERY_PCAP_BYTES"]),
            "total_frames": int(os.environ["EV_RECOVERY_TOTAL_FRAMES"]),
            "guest_originated_frames": int(os.environ["EV_RECOVERY_GUEST_FRAMES"]),
        },
        "serial": {
            "path": os.environ["EV_RECOVERY_SERIAL"],
            "bytes": int(os.environ["EV_RECOVERY_SERIAL_BYTES"]),
        },
        "monitor": {
            "path": os.environ["EV_RECOVERY_MONITOR"],
            "bytes": int(os.environ["EV_RECOVERY_MONITOR_BYTES"]),
        },
    },
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(payload, handle, separators=(",", ":"), sort_keys=True)
    handle.write("\n")
PY
}

export_evidence_env() {
  export EV_SCHEMA_VERSION="$SCHEMA_VERSION"
  export EV_STATUS="$1"
  export EV_CLASSIFICATION="$classification"
  export EV_HARNESS_ERROR="${harness_error}"
  export EV_APP_NAME="$APP_NAME"
  export EV_IMAGE_VERSION="$image_version"
  export EV_SEEDED="$SEEDED_IMG"
  export EV_RESCUE="$RESCUE_IMG"
  export EV_ACCEL="$accel"
  export EV_GUEST_MAC="$GUEST_MAC"
  export EV_OVMF_CODE="$OVMF_CODE"
  export EV_OVMF_VARS_TEMPLATE="$OVMF_VARS_TEMPLATE"
  export EV_ONLINE_TIMEOUT="$ONLINE_BOOT_SECONDS"
  export EV_ONLINE_ELAPSED="${2:-0}"
  export EV_ONLINE_QEMU_EXIT="${3:-}"
  export EV_RECOVERY_REACHABLE="${4:-false}"
  export EV_RECOVERY_SKIPPED="${5:-}"
  export EV_RECOVERY_TIMEOUT="$RECOVERY_BOOT_SECONDS"
  export EV_RECOVERY_ELAPSED="${6:-0}"
  export EV_RECOVERY_QEMU_EXIT="${7:-}"
  export EV_SRC="$SRC_QCOW"
  export EV_SRC_BEFORE_BYTES="${8:-0}"
  export EV_SRC_BEFORE_ACTUAL="${9:-0}"
  export EV_SRC_AFTER_BYTES="${10:-0}"
  export EV_SRC_AFTER_ACTUAL="${11:-0}"
  export EV_TGT="$TARGET_QCOW"
  export EV_TGT_BEFORE_BYTES="${12:-0}"
  export EV_TGT_BEFORE_ACTUAL="${13:-0}"
  export EV_TGT_AFTER_BYTES="${14:-0}"
  export EV_TGT_AFTER_ACTUAL="${15:-0}"
  export EV_ONLINE_PCAP="$PCAP_DIR/online.pcap"
  export EV_ONLINE_PCAP_BYTES="${16:-0}"
  export EV_ONLINE_TOTAL_FRAMES="${17:-0}"
  export EV_ONLINE_GUEST_FRAMES="${18:-0}"
  export EV_RECOVERY_PCAP="$PCAP_DIR/recovery.pcap"
  export EV_RECOVERY_PCAP_BYTES="${19:-0}"
  export EV_RECOVERY_TOTAL_FRAMES="${20:-0}"
  export EV_RECOVERY_GUEST_FRAMES="${21:-0}"
  export EV_ENROLL_MSG="${22:-false}"
  export EV_ENROLL_OK="${23:-false}"
  export EV_SB_REFUSAL="${24:-false}"
  export EV_RESCUE_DETECTED="${25:-false}"
  export EV_UPDATE_SJSON="${26:-false}"
  export EV_UPDATE_JSON="${27:-false}"
  export EV_SRC_GREW='false'
  export EV_TGT_GREW='false'
  export EV_GUEST_NET='false'
  export EV_SEED_OBS='false'
  if (( ${EV_SRC_AFTER_ACTUAL:-0} > ${EV_SRC_BEFORE_ACTUAL:-0} || ${EV_SRC_AFTER_BYTES:-0} > ${EV_SRC_BEFORE_BYTES:-0} )); then
    EV_SRC_GREW='true'
  fi
  if (( ${EV_TGT_AFTER_ACTUAL:-0} - ${EV_TGT_BEFORE_ACTUAL:-0} >= SEED_CONSUMPTION_BYTES || ${EV_TGT_AFTER_BYTES:-0} - ${EV_TGT_BEFORE_BYTES:-0} >= SEED_CONSUMPTION_BYTES )); then
    EV_TGT_GREW='true'
    EV_SEED_OBS='true'
  fi
  if (( ${EV_ONLINE_GUEST_FRAMES:-0} > 0 )); then
    EV_GUEST_NET='true'
  fi
  export EV_ONLINE_SERIAL="$LOG_DIR/online.serial.log"
  export EV_ONLINE_SERIAL_BYTES
  EV_ONLINE_SERIAL_BYTES="$(file_size "$LOG_DIR/online.serial.log")"
  export EV_ONLINE_MONITOR="$LOG_DIR/online.monitor.log"
  export EV_ONLINE_MONITOR_BYTES
  EV_ONLINE_MONITOR_BYTES="$(file_size "$LOG_DIR/online.monitor.log")"
  export EV_RECOVERY_SERIAL="$LOG_DIR/recovery.serial.log"
  export EV_RECOVERY_SERIAL_BYTES
  EV_RECOVERY_SERIAL_BYTES="$(file_size "$LOG_DIR/recovery.serial.log")"
  export EV_RECOVERY_MONITOR="$LOG_DIR/recovery.monitor.log"
  export EV_RECOVERY_MONITOR_BYTES
  EV_RECOVERY_MONITOR_BYTES="$(file_size "$LOG_DIR/recovery.monitor.log")"
}

stop_pidfile() {
  local pidfile="$1"
  if [[ -z "$pidfile" || ! -f "$pidfile" ]]; then
    return 0
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$pidfile" || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    local waited=0
    while kill -0 "$pid" 2>/dev/null && (( waited < 10 )); do
      sleep 1
      waited=$((waited + 1))
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  fi
  rm -f "$pidfile"
}

stop_qemu() {
  if [[ -n "$qemu_pid" ]] && kill -0 "$qemu_pid" 2>/dev/null; then
    kill -TERM "$qemu_pid" 2>/dev/null || true
    local grace=0
    while kill -0 "$qemu_pid" 2>/dev/null && (( grace < 10 )); do
      sleep 1
      grace=$((grace + 1))
    done
    if kill -0 "$qemu_pid" 2>/dev/null; then
      kill -KILL "$qemu_pid" 2>/dev/null || true
    fi
    wait "$qemu_pid" 2>/dev/null || true
  fi
  qemu_pid=''
}

cleanup_guests() {
  stop_qemu
  stop_pidfile "$swtpm_pidfile"
  swtpm_pidfile=''
}

fail_setup() {
  harness_error="$1"
  classification='harness_failed'
  export_evidence_env failed
  write_evidence failed || write_stub_evidence failed
  exit 1
}

on_harness_fail() {
  local line="${1:-unknown}"
  if [[ -z "$harness_error" ]]; then
    harness_error="harness failure at line ${line}"
  fi
  classification='harness_failed'
  cleanup_guests
  export_evidence_env failed 0 '' false "$harness_error" 0 '' 0 0 0 0 0 0 0 0 0 0 0 0 0 0 false false false false false false
  write_evidence failed || write_stub_evidence failed
}

trap 'on_harness_fail $LINENO; exit 1' ERR
trap 'cleanup_guests' EXIT

require_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    fail_setup "required command not found: ${name}"
  fi
}

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    fail_setup "required file not found: ${path}"
  fi
}

first_existing() {
  local path
  for path in "$@"; do
    if [[ -f "$path" ]]; then
      printf '%s' "$path"
      return 0
    fi
  done
  return 1
}

firmware_format() {
  case "$1" in
    *.qcow2) printf 'qcow2' ;;
    *) printf 'raw' ;;
  esac
}

discover_ovmf() {
  local code vars
  code="$(first_existing \
    /usr/share/OVMF/OVMF_CODE_4M.secboot.fd \
    /usr/share/OVMF/OVMF_CODE.secboot.4m.fd \
    /usr/share/OVMF/OVMF_CODE_4M.secboot.qcow2 \
    /usr/share/OVMF/OVMF_CODE.secboot.fd \
    /usr/share/OVMF/OVMF_CODE.secboot.qcow2 \
    /usr/share/OVMF/OVMF_CODE_4M.ms.fd \
    /usr/share/qemu/edk2-x86_64-secure-code.fd \
    /usr/share/OVMF/OVMF_CODE_4M.fd \
    /usr/share/OVMF/OVMF_CODE.fd \
    || true)"
  if [[ -z "$code" ]]; then
    fail_setup "no OVMF secure-boot code found under /usr/share/OVMF or /usr/share/qemu"
  fi

  case "$code" in
    *4M*|*4m*|*edk2-x86_64-secure-code.fd)
      if [[ "$code" == *.qcow2 ]]; then
        vars="$(first_existing \
          /usr/share/OVMF/OVMF_VARS_4M.fd \
          /usr/share/OVMF/OVMF_VARS.4m.fd \
          /usr/share/OVMF/OVMF_VARS_4M.qcow2 \
          /usr/share/OVMF/OVMF_VARS.4m.qcow2 \
          /usr/share/OVMF/OVMF_VARS.qcow2 \
          /usr/share/OVMF/OVMF_VARS.fd \
          /usr/share/qemu/edk2-i386-vars.fd \
          || true)"
      else
        vars="$(first_existing \
          /usr/share/OVMF/OVMF_VARS_4M.fd \
          /usr/share/OVMF/OVMF_VARS.4m.fd \
          /usr/share/OVMF/OVMF_VARS_4M.snakeoil.fd \
          /usr/share/OVMF/OVMF_VARS.fd \
          /usr/share/qemu/edk2-i386-vars.fd \
          || true)"
      fi
      ;;
    *)
      vars="$(first_existing \
        /usr/share/OVMF/OVMF_VARS.fd \
        /usr/share/OVMF/OVMF_VARS_4M.fd \
        /usr/share/OVMF/OVMF_VARS.4m.fd \
        /usr/share/qemu/edk2-i386-vars.fd \
        || true)"
      ;;
  esac
  if [[ -z "$vars" ]]; then
    fail_setup "no setup-mode OVMF varstore template found to pair with ${code}"
  fi

  OVMF_CODE="$code"
  OVMF_VARS_TEMPLATE="$vars"
  OVMF_FORMAT="$(firmware_format "$code")"
  if [[ "$OVMF_FORMAT" == 'qcow2' ]]; then
    OVMF_VARS="$WORK_DIR/OVMF_VARS.qcow2"
  else
    OVMF_VARS="$WORK_DIR/OVMF_VARS.fd"
  fi
}

prepare_ovmf_vars() {
  rm -f "$OVMF_VARS"
  local vars_format
  vars_format="$(firmware_format "$OVMF_VARS_TEMPLATE")"
  if [[ "$OVMF_FORMAT" == 'qcow2' ]]; then
    if [[ "$vars_format" == 'qcow2' ]]; then
      qemu-img create -f qcow2 -F qcow2 -b "$OVMF_VARS_TEMPLATE" "$OVMF_VARS"
    else
      qemu-img convert -f raw -O qcow2 "$OVMF_VARS_TEMPLATE" "$OVMF_VARS"
    fi
  elif [[ "$vars_format" == 'qcow2' ]]; then
    qemu-img convert -f qcow2 -O raw "$OVMF_VARS_TEMPLATE" "$OVMF_VARS"
  else
    cp "$OVMF_VARS_TEMPLATE" "$OVMF_VARS"
  fi
}

select_accel() {
  if [[ -e /dev/kvm && -r /dev/kvm && -w /dev/kvm ]]; then
    accel='kvm'
  else
    accel='tcg'
  fi
}

start_swtpm() {
  local state_dir="$1"
  local sock="$2"
  local pidfile="$3"
  rm -rf "$state_dir"
  mkdir -p "$state_dir"
  rm -f "$sock" "$pidfile"
  swtpm socket \
    --tpm2 \
    --tpmstate "dir=${state_dir}" \
    --ctrl "type=unixio,path=${sock}" \
    --log "level=1,file=${LOG_DIR}/$(basename "$state_dir").swtpm.log" \
    --pid "file=${pidfile}" \
    --daemon
  swtpm_pidfile="$pidfile"
  local waited=0
  while [[ ! -e "$sock" ]] && (( waited < 50 )); do
    sleep 0.1
    waited=$((waited + 1))
  done
  if [[ ! -e "$sock" ]]; then
    harness_error="swtpm did not create control socket ${sock}"
    classification='harness_failed'
    return 1
  fi
}

run_guest() {
  local label="$1"
  local timeout_secs="$2"
  local boot_disk="$3"
  local second_disk="$4"
  local extra_disk="$5"
  local pcap="$6"
  local serial_log="$7"
  local monitor_log="$8"
  local tpm_state="$9"
  local tpm_sock="${10}"
  local tpm_pid="${11}"

  : >"$serial_log"
  : >"$monitor_log"
  rm -f "$pcap"

  start_swtpm "$tpm_state" "$tpm_sock" "$tpm_pid"

  local started_at ended_at elapsed exit_code
  started_at="$(date +%s)"

  local qemu_args=(
    qemu-system-x86_64
    -machine "q35,smm=on,accel=${accel}"
    -m 4096
    -smp 4
    -display none
    -global driver=cfi.pflash01,property=secure,value=on
    -drive "if=pflash,format=${OVMF_FORMAT},unit=0,readonly=on,file=${OVMF_CODE}"
    -drive "if=pflash,format=${OVMF_FORMAT},unit=1,file=${OVMF_VARS}"
    -chardev "socket,id=chrtpm,path=${tpm_sock}"
    -tpmdev emulator,id=tpm0,chardev=chrtpm
    -device tpm-tis,tpmdev=tpm0
    -drive "file=${boot_disk},if=virtio,format=qcow2"
  )

  if [[ -n "$second_disk" ]]; then
    qemu_args+=(-drive "file=${second_disk},if=virtio,format=qcow2")
  fi
  if [[ -n "$extra_disk" ]]; then
    qemu_args+=(-drive "file=${extra_disk},if=virtio,format=raw,readonly=on")
  fi

  qemu_args+=(
    -netdev user,id=n0
    -device "virtio-net-pci,netdev=n0,mac=${GUEST_MAC}"
    -object "filter-dump,id=d0,netdev=n0,file=${pcap}"
    -serial "file:${serial_log}"
    -monitor "file:${monitor_log}"
  )

  "${qemu_args[@]}" &
  qemu_pid="$!"

  local waited=0
  while kill -0 "$qemu_pid" 2>/dev/null && (( waited < timeout_secs )); do
    sleep 1
    waited=$((waited + 1))
  done

  if kill -0 "$qemu_pid" 2>/dev/null; then
    kill -TERM "$qemu_pid" 2>/dev/null || true
    local grace=0
    while kill -0 "$qemu_pid" 2>/dev/null && (( grace < 20 )); do
      sleep 1
      grace=$((grace + 1))
    done
    if kill -0 "$qemu_pid" 2>/dev/null; then
      kill -KILL "$qemu_pid" 2>/dev/null || true
    fi
  fi

  exit_code=0
  wait "$qemu_pid" || exit_code=$?
  qemu_pid=''
  ended_at="$(date +%s)"
  elapsed=$((ended_at - started_at))
  stop_pidfile "$tpm_pid"
  swtpm_pidfile=''

  if (( elapsed < 5 )) && (( exit_code != 0 )); then
    harness_error="${label} qemu exited after ${elapsed}s with status ${exit_code}"
    classification='harness_failed'
    return 1
  fi

  printf '%s %s' "$elapsed" "$exit_code"
}

require_cmd qemu-system-x86_64
require_cmd qemu-img
require_cmd swtpm
require_cmd python3
discover_ovmf
select_accel

if [[ ! -x "$CLI" ]]; then
  require_cmd moon
  (cd "$REPO_ROOT" && moon run :build --summary minimal)
fi
require_file "$CLI"
if [[ ! -x "$CLI" ]]; then
  fail_setup "built CLI is not executable: ${CLI}"
fi

cat >"$CONFIG_PATH" <<EOF
version: 1
image:
  type: raw
  architecture: x86_64
  channel: stable
  offline: true
seeds:
  applications:
    applications:
      - name: ${APP_NAME}
  install:
    force_install: true
    force_reboot: true
    target:
      sort_order: largest
  network:
    version: "1"
EOF

build_json="$WORK_DIR/build.json"
"$CLI" build --json --no-input --color never --progress never \
  -f "$CONFIG_PATH" \
  -o "$SEEDED_IMG" \
  --resources-output "$RESCUE_IMG" \
  --force \
  >"$build_json"

image_version="$(python3 - "$build_json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
if "error" in doc:
    err = doc.get("error") or {}
    message = err.get("message") or "build failed"
    raise SystemExit(message)
print((doc.get("result") or {}).get("version") or "")
PY
)"

require_file "$SEEDED_IMG"
require_file "$RESCUE_IMG"

rm -f "$SRC_QCOW" "$TARGET_QCOW"
qemu-img create -f qcow2 -F raw -b "$SEEDED_IMG" "$SRC_QCOW"
qemu-img create -f qcow2 "$TARGET_QCOW" "${TARGET_DISK_GIB}G"
prepare_ovmf_vars

read -r src_before_bytes src_before_actual <<<"$(disk_snapshot "$SRC_QCOW")"
read -r tgt_before_bytes tgt_before_actual <<<"$(disk_snapshot "$TARGET_QCOW")"

online_tpm_state="$WORK_DIR/tpm-online"
online_result="$(run_guest \
  online \
  "$ONLINE_BOOT_SECONDS" \
  "$SRC_QCOW" \
  "$TARGET_QCOW" \
  '' \
  "$PCAP_DIR/online.pcap" \
  "$LOG_DIR/online.serial.log" \
  "$LOG_DIR/online.monitor.log" \
  "$online_tpm_state" \
  "$WORK_DIR/swtpm-online.sock" \
  "$WORK_DIR/swtpm-online.pid")"
read -r online_elapsed online_qemu_exit <<<"$online_result"

read -r src_after_bytes src_after_actual <<<"$(disk_snapshot "$SRC_QCOW")"
read -r tgt_after_bytes tgt_after_actual <<<"$(disk_snapshot "$TARGET_QCOW")"
read -r online_total_frames online_guest_frames <<<"$(count_pcap_frames "$PCAP_DIR/online.pcap")"
online_pcap_bytes="$(file_size "$PCAP_DIR/online.pcap")"

enroll_msg="$(serial_contains "$LOG_DIR/online.serial.log" 'Enrolling secure boot keys')"
enroll_ok="$(serial_contains "$LOG_DIR/online.serial.log" 'Custom Secure Boot keys successfully enrolled')"
sb_refusal="$(serial_contains "$LOG_DIR/online.serial.log" 'Unable to determine SecureBoot state')"

if (( tgt_after_actual - tgt_before_actual >= SEED_CONSUMPTION_BYTES || tgt_after_bytes - tgt_before_bytes >= SEED_CONSUMPTION_BYTES )); then
  online_seed_consumption='true'
fi

recovery_reachable='false'
recovery_skipped='online seed consumption was not observed'
recovery_elapsed=0
recovery_qemu_exit=''
recovery_pcap_bytes=0
recovery_total_frames=0
recovery_guest_frames=0
rescue_detected='false'
update_sjson='false'
update_json='false'

if [[ "$online_seed_consumption" == 'true' ]]; then
  recovery_reachable='true'
  recovery_skipped=''
  recovery_result="$(run_guest \
    recovery \
    "$RECOVERY_BOOT_SECONDS" \
    "$TARGET_QCOW" \
    '' \
    "$RESCUE_IMG" \
    "$PCAP_DIR/recovery.pcap" \
    "$LOG_DIR/recovery.serial.log" \
    "$LOG_DIR/recovery.monitor.log" \
    "$WORK_DIR/tpm-recovery" \
    "$WORK_DIR/swtpm-recovery.sock" \
    "$WORK_DIR/swtpm-recovery.pid")"
  read -r recovery_elapsed recovery_qemu_exit <<<"$recovery_result"
  read -r recovery_total_frames recovery_guest_frames <<<"$(count_pcap_frames "$PCAP_DIR/recovery.pcap")"
  recovery_pcap_bytes="$(file_size "$PCAP_DIR/recovery.pcap")"
  rescue_detected="$(serial_contains "$LOG_DIR/recovery.serial.log" 'RESCUE_DATA')"
  update_sjson="$(serial_contains "$LOG_DIR/recovery.serial.log" 'update.sjson')"
  update_json="$(serial_contains "$LOG_DIR/recovery.serial.log" 'update.json')"
else
  : >"$LOG_DIR/recovery.serial.log"
  : >"$LOG_DIR/recovery.monitor.log"
  : >"$PCAP_DIR/recovery.pcap"
fi

if [[ "$online_seed_consumption" == 'true' ]]; then
  classification='positive'
else
  classification='negative'
fi

trap - ERR
export_evidence_env completed \
  "$online_elapsed" "$online_qemu_exit" \
  "$recovery_reachable" "$recovery_skipped" "$recovery_elapsed" "$recovery_qemu_exit" \
  "$src_before_bytes" "$src_before_actual" "$src_after_bytes" "$src_after_actual" \
  "$tgt_before_bytes" "$tgt_before_actual" "$tgt_after_bytes" "$tgt_after_actual" \
  "$online_pcap_bytes" "$online_total_frames" "$online_guest_frames" \
  "$recovery_pcap_bytes" "$recovery_total_frames" "$recovery_guest_frames" \
  "$enroll_msg" "$enroll_ok" "$sb_refusal" \
  "$rescue_detected" "$update_sjson" "$update_json"
write_evidence completed \
  "$online_elapsed" "$online_qemu_exit" \
  "$recovery_reachable" "$recovery_skipped" "$recovery_elapsed" "$recovery_qemu_exit" \
  "$src_before_bytes" "$src_before_actual" "$src_after_bytes" "$src_after_actual" \
  "$tgt_before_bytes" "$tgt_before_actual" "$tgt_after_bytes" "$tgt_after_actual" \
  "$online_pcap_bytes" "$online_total_frames" "$online_guest_frames" \
  "$recovery_pcap_bytes" "$recovery_total_frames" "$recovery_guest_frames" \
  "$enroll_msg" "$enroll_ok" "$sb_refusal" \
  "$rescue_detected" "$update_sjson" "$update_json"

printf 'wrote %s classification=%s seed_consumption=%s\n' \
  "$EVIDENCE_PATH" "$classification" "$online_seed_consumption"
