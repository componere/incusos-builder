# Phase 5.2 Linux boot experiment

> **Superseded on 2026-08-17.** This negative result stood until Track C
> passed on a Semaphore Cloud `f1-standard-4` machine with nested KVM
> (pipeline `4c5cc805`, job `a8b16331`). That run observed install
> completion, target-seed cleanup, `RESCUE_DATA` detection, and
> human-adjudicated recovery-payload acceptance. The findings below remain
> the dated record of the Phase 5.2 experiment.
>
> The durable explanation is that this probe's target-disk-growth oracle
> was correct in principle, but the installer never reached its target
> write path. Its console was also blind: plain QEMU did not provide
> `/dev/virtio-ports/org.linuxcontainers.incus`, and the IncusOS TUI adds
> `/dev/ttyS0` only when that port exists
> (`internal/tui/tui.go:67-71`). The probe therefore could not distinguish
> a failed install from an install that produced no serial output. Under
> Incus, the port exists and the installer TUI mirrors to the serial
> console.

Completed diagnostic. Classification is **negative**. The v1 boot
acceptance gate is a manual release checklist, not a CI job. The
manual checklist runs before every release tag until a CI boot gate
succeeds.

This is the Phase 5.2 revisit named by the Phase 1 boot spike, whose
findings live in the session 002 journal folder
(`.journal/002/spike-1e-boot.md` on the `journal/jmgilman` branch).
Machine evidence is committed as
[`docs/notes/phase-5-boot-evidence.json`](phase-5-boot-evidence.json).
The temporary branch-only workflow and harness that produced the run
are not kept.

## Run

- Workflow run: [31958439711](https://github.com/componere/incusos-builder/actions/runs/31958439711)
- Started: 2026-08-16T16:22:56Z
- Updated: 2026-08-16T16:33:11Z
- Head: `b19c2336aeab1e06b18194c95e6faf7ff3da1a59` on `feat/phase-5-e2e-boot`
- Job conclusion: success (`status: completed`, `harness_error: null`)

## Topology

GitHub-hosted `ubuntu-latest` QEMU guest, TCG, machine `q35,smm=on`.
Firmware `/usr/share/OVMF/OVMF_CODE_4M.secboot.fd` with a writable
setup-mode varstore from `/usr/share/OVMF/OVMF_VARS_4M.fd`. Guest MAC
`52:54:00:12:34:56`. Image: live x86_64 raw seeded media plus raw
rescue media, IncusOS `202608102114`, channel `stable`, application
`debug`. Online window 480 s (elapsed 482 s, QEMU exit `0`). Recovery
window would have been 300 s; it did not run.

## Measurements

From the committed evidence:

- Classification: `negative`.
- Secure Boot enrollment: `Enrolling secure boot keys` and
  `Custom Secure Boot keys successfully enrolled` both seen. Secure
  Boot refusal was not seen.
- Source overlay grew: actual size 200704 → 856064 bytes; file size
  196664 → 917504 bytes. Source growth is not seed consumption.
- Blank target did not grow: actual size stayed 200704 bytes; file
  size stayed 196736 bytes. `target_disk.grew` is false.
- Seed consumption: not observed (`seed_consumption_observed: false`).
- Guest-originated frames: 3 of 3 pcap frames (322 bytes). Network
  activity is diagnostic only. It is not seed consumption and does
  not change the classification.
- Recovery: not reachable. `skipped_reason` is
  `online seed consumption was not observed`. Elapsed 0 s.
- Signed rescue acceptance was not observed.
  `rescue_data_detected`, `update_sjson_acceptance`, and
  `update_json_acceptance` are all false because the recovery guest
  did not run.

## Decision

Phase 5.2 required one Linux-hosted diagnostic. Seed consumption was
not observed, so recovery acceptance could not be asserted. A CI boot
gate is not viable. The v1 gate is the manual release checklist on a
Linux host with Incus, as the interim default in spike 1.E. That
checklist runs before every release tag until a CI boot gate
succeeds.
