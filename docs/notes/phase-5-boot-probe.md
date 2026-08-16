# Phase 5 boot probe contract

Temporary branch-only diagnostic for one Linux-hosted boot-acceptance
run. The workflow is not a merge gate. This note records the probe
contract, evidence schema, success criteria, and reproduction command.
It does not record a run.

Reference: `docs/notes/spike-1e-boot.md` attempt C, and the Phase 5.2
decision there. The workflow file is
`.github/workflows/phase5-boot-probe.yml`. The harness is
`.github/scripts/phase5_boot_probe.sh`.

## Contract

- Trigger: `push` to `feat/phase-5-e2e-boot` only. No
  `workflow_dispatch`, pull-request, or default-branch trigger.
- Runner: one `ubuntu-latest` job. No secrets.
- Permissions: workflow default `{}`; job `contents: read`.
- Concurrency: `${{ github.workflow }}-${{ github.ref }}` with
  `cancel-in-progress: true`.
- Timeout: job `timeout-minutes: 60`. Online guest window defaults to
  480 s (`PHASE5_ONLINE_BOOT_SECONDS`). Conditional recovery guest
  window defaults to 300 s (`PHASE5_RECOVERY_BOOT_SECONDS`).
- Toolchain: pinned checkout, mise, and upload-artifact SHAs matching
  `.github/workflows/ci.yml` / `.github/workflows/release.yml`.
  `GOTOOLCHAIN=local`.
- Host packages: `qemu-system-x86`, `qemu-utils`, `ovmf`,
  `swtpm`, `swtpm-tools`, `tcpdump`, `tshark`.
- Firmware: first existing Secure Boot code and setup-mode varstore
  under `/usr/share/OVMF` or `/usr/share/qemu`. Current Ubuntu names
  include `OVMF_CODE_4M.secboot.fd`, `OVMF_CODE.secboot.4m.fd`,
  `OVMF_CODE_4M.secboot.qcow2`, and `edk2-x86_64-secure-code.fd`,
  paired with `OVMF_VARS_4M.fd`, `OVMF_VARS.4m.fd`, matching `.qcow2`
  varstores, or `edk2-i386-vars.fd`. The selected varstore is copied
  or cloned writable per run. Missing both families is a harness
  failure.
- Guest: direct port of attempt C. `q35,smm=on`,
  `cfi.pflash01.secure=on`, `swtpm` TPM 2.0 + `tpm-tis`, virtio source
  overlay, blank 8 GiB virtio target, deterministic MAC
  `52:54:00:12:34:56`, slirp `filter-dump` pcap, serial file, monitor
  file. Reboot is allowed so `secure-boot-enroll force` can enroll and
  continue inside the online window.
- Image: live x86_64 raw seeded media plus raw rescue media from
  `incusos-builder build` (`image.type: raw`, `architecture: x86_64`,
  `channel: stable`, `offline: true`, application `debug`).
- Online oracles: source and target disk size before and after,
  Secure Boot enrollment strings on serial. Guest-originated frames
  filtered by source MAC are recorded as diagnostics, not oracles.
- Recovery oracle: run only when online seed consumption is observed.
  Second time-boxed boot from the installed target with rescue media
  attached. Record `RESCUE_DATA` detection and
  `update.sjson` / `update.json` acceptance evidence. If seed
  consumption is not observed, record that recovery was not reachable.
- Evidence: always write compact `evidence.json` plus raw logs and
  pcaps. Empty log and pcap files are created before the first
  command so a setup failure still uploads a complete artifact.
  The workflow uploads them as artifact `phase5-boot-probe`
  (`if-no-files-found: error`) even when the job fails.
- Exit policy: harness or setup failure exits non-zero after writing
  whatever evidence exists. A completed diagnostic, including a clean
  negative finding, exits 0. The orchestrator classifies the finding
  from `evidence.json`; the job does not fail on a negative boot.

## Seed consumption

Online seed consumption is true only when the target qcow2 grows by at
least 1 MiB (`actual-size` from `qemu-img info --output=json`, or file
size if actual-size is unavailable). That growth is the only condition
that may set `online_boot.seed_consumption_observed`,
`classification: positive`, or `recovery_boot.reachable`.

Guest-originated pcap frames, including DHCP, ARP, and IPv6 traffic
whose Ethernet source MAC is the guest MAC, are recorded as
`guest_originated_frames` and `guest_originated_observed`. They are
independent diagnostic evidence. Network traffic does not prove seed
consumption.

Source-disk growth is recorded but is not seed consumption. Spike 1.E
attempt C observed ESP writes on the install-media overlay from key
enrollment.

## `evidence.json` fields

Top-level object. Absent optional strings are JSON `null`.

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | number | Evidence schema version. Current value `1`. |
| `probe` | string | Constant `phase5-boot-probe`. |
| `status` | string | `completed` or `failed`. |
| `classification` | string | `positive` when online seed consumption was observed, `negative` when the diagnostic completed without it, `harness_failed` when setup or qemu harness failed. |
| `harness_error` | string or null | Setup or qemu harness error. Null on a completed diagnostic. |
| `image.type` | string | Built image type. Always `raw`. |
| `image.architecture` | string | Always `x86_64`. |
| `image.channel` | string | Always `stable`. |
| `image.application` | string | Offline application used for rescue media. Always `debug`. |
| `image.version` | string or null | Resolved IncusOS version from the build envelope. |
| `image.seeded_image` | string | Path of the seeded raw image. |
| `image.rescue_image` | string | Path of the raw rescue media. |
| `guest.machine` | string | QEMU machine string. `q35,smm=on`. |
| `guest.accel` | string | `kvm` when `/dev/kvm` is readable and writable, otherwise `tcg`. |
| `guest.mac` | string | Guest NIC MAC. `52:54:00:12:34:56`. |
| `guest.firmware_code` | string | OVMF secure-boot code path. |
| `guest.firmware_vars_template` | string | Setup-mode varstore template copied per run. |
| `online_boot.timeout_seconds` | number | Online guest time box. |
| `online_boot.elapsed_seconds` | number | Observed online guest wall time. |
| `online_boot.qemu_exit_code` | string or null | QEMU wait status as a decimal string. |
| `online_boot.secure_boot_enrollment.enroll_message_seen` | boolean | Serial contained `Enrolling secure boot keys`. |
| `online_boot.secure_boot_enrollment.enroll_success_seen` | boolean | Serial contained `Custom Secure Boot keys successfully enrolled`. |
| `online_boot.secure_boot_enrollment.secure_boot_refusal_seen` | boolean | Serial contained `Unable to determine SecureBoot state`. |
| `online_boot.source_disk.path` | string | Source overlay path. |
| `online_boot.source_disk.before_bytes` | number | Source file size before the online boot. |
| `online_boot.source_disk.after_bytes` | number | Source file size after the online boot. |
| `online_boot.source_disk.before_actual_bytes` | number | Source `qemu-img` actual-size before the online boot. |
| `online_boot.source_disk.after_actual_bytes` | number | Source `qemu-img` actual-size after the online boot. |
| `online_boot.source_disk.grew` | boolean | Source size increased. Not a seed-consumption oracle. |
| `online_boot.target_disk.path` | string | Blank target disk path. |
| `online_boot.target_disk.before_bytes` | number | Target file size before the online boot. |
| `online_boot.target_disk.after_bytes` | number | Target file size after the online boot. |
| `online_boot.target_disk.before_actual_bytes` | number | Target `qemu-img` actual-size before the online boot. |
| `online_boot.target_disk.after_actual_bytes` | number | Target `qemu-img` actual-size after the online boot. |
| `online_boot.target_disk.grew` | boolean | Target growth met the 1 MiB seed-consumption threshold. |
| `online_boot.network.guest_mac` | string | MAC used to filter guest-originated frames. |
| `online_boot.network.pcap` | string | Online pcap path. |
| `online_boot.network.pcap_bytes` | number | Online pcap file size. |
| `online_boot.network.total_frames` | number | All frames in the online pcap, including slirp. |
| `online_boot.network.guest_originated_frames` | number | Frames whose source MAC is the guest MAC. |
| `online_boot.network.guest_originated_observed` | boolean | `guest_originated_frames > 0`. Diagnostic only; not seed consumption. |
| `online_boot.serial.path` | string | Online serial log path. |
| `online_boot.serial.bytes` | number | Online serial log size. |
| `online_boot.monitor.path` | string | Online monitor log path. |
| `online_boot.monitor.bytes` | number | Online monitor log size. |
| `online_boot.seed_consumption_observed` | boolean | Target-disk growth met the 1 MiB threshold. Guest-network frames do not set this. |
| `recovery_boot.reachable` | boolean | Second boot ran because seed consumption was observed. |
| `recovery_boot.skipped_reason` | string or null | Why recovery did not run. Set when `reachable` is false. |
| `recovery_boot.timeout_seconds` | number | Recovery guest time box. |
| `recovery_boot.elapsed_seconds` | number | Observed recovery guest wall time. Zero when skipped. |
| `recovery_boot.qemu_exit_code` | string or null | Recovery QEMU wait status. Null when skipped. |
| `recovery_boot.rescue_data_detected` | boolean | Recovery serial contained `RESCUE_DATA`. |
| `recovery_boot.update_sjson_acceptance` | boolean | Recovery serial contained `update.sjson`. |
| `recovery_boot.update_json_acceptance` | boolean | Recovery serial contained `update.json`. |
| `recovery_boot.network.pcap` | string | Recovery pcap path. Empty file when skipped. |
| `recovery_boot.network.pcap_bytes` | number | Recovery pcap size. |
| `recovery_boot.network.total_frames` | number | All frames in the recovery pcap. |
| `recovery_boot.network.guest_originated_frames` | number | Recovery frames from the guest MAC. |
| `recovery_boot.serial.path` | string | Recovery serial log path. |
| `recovery_boot.serial.bytes` | number | Recovery serial log size. |
| `recovery_boot.monitor.path` | string | Recovery monitor log path. |
| `recovery_boot.monitor.bytes` | number | Recovery monitor log size. |

## Artifact layout

Uploaded as `phase5-boot-probe` from
`${{ github.workspace }}/.phase5-boot-probe/`:

```
evidence.json
logs/online.serial.log
logs/online.monitor.log
logs/recovery.serial.log
logs/recovery.monitor.log
pcaps/online.pcap
pcaps/recovery.pcap
```

swtpm logs may also appear under `logs/`. Multi-GB images stay on the
runner workspace and are not uploaded.

## Success criteria

The probe job succeeds when the harness finishes and writes
`evidence.json`. That includes a negative finding
(`classification: negative`,
`online_boot.seed_consumption_observed: false`,
`recovery_boot.reachable: false`).

The probe job fails only when a required package, firmware file, CLI
build, image build, or qemu launch fails before a diagnostic result
exists, or qemu exits in under five seconds with a non-zero status.

Orchestrator classification after the run:

- `classification: harness_failed` — treat as a workflow failure, not
  a boot finding.
- `classification: negative` — target-disk growth of at least 1 MiB
  was not observed. Guest-originated frames alone do not change this.
  Recovery acceptance was not reachable. Keep the release checklist as
  the v1 gate.
- `classification: positive` — target-disk growth of at least 1 MiB
  was observed. Read the recovery fields to decide whether
  `RESCUE_DATA` and `update.sjson` / `update.json` acceptance can
  become a CI gate. Network traffic does not produce this
  classification.

## Reproduction

On a Linux host with the same packages and firmware files as the
workflow, from the repository root:

```bash
moon run :build --summary minimal
PHASE5_BOOT_PROBE_DIR="$PWD/.phase5-boot-probe" \
  bash .github/scripts/phase5_boot_probe.sh
```

The script writes `evidence.json`, logs, and pcaps under
`PHASE5_BOOT_PROBE_DIR`. Override `PHASE5_ONLINE_BOOT_SECONDS` or
`PHASE5_RECOVERY_BOOT_SECONDS` to change the guest time boxes. The
GitHub Actions path is a push to `feat/phase-5-e2e-boot`.
