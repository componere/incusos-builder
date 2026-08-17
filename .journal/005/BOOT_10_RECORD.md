---
title: Track C boot acceptance release record
subject: componere/incusos-builder @ c08cef3097c657044a92afda181e89dd6c5da2f7
upstream: v0.0.0-20260815030500-0f5b8057f2fc
date: 2026-08-17
session: 005
status: adjudicated pass
---

# Track C boot acceptance release record

## Verdict

**Pass.** BOOT-05 through BOOT-08 met all four required observations. The
pipeline asserted O1, O2, and O3 against behavior and strings from the pinned
IncusOS source. It also asserted that the signed recovery metadata passed
signature verification. The developer reviewed the O4 evidence and adjudicated
the recovery payload as accepted.

This is a Track C boot-acceptance verdict, not a decision to ship the release.
The other release blockers in `FUNCTIONAL_TEST_PLAN.md` remain independent.

## Execution identity

| Field | Value |
|---|---|
| Date | 2026-08-17 |
| Venue | Semaphore Cloud `f1-standard-4`, nested KVM |
| Pipeline | `4c5cc805-c758-4292-bb50-da877dc5fb18` (`4c5cc805`) |
| Job | `a8b16331-fdc2-43f2-ab16-4f7d555b40d0` (`a8b16331`) |
| Builder commit | `c08cef3097c657044a92afda181e89dd6c5da2f7` |
| IncusOS upstream pin | `v0.0.0-20260815030500-0f5b8057f2fc` |
| Image release | `202608102114`, channel `stable` |
| Image type | `raw`, `x86_64`, offline |
| Full transcript | [`TRACK_C_GATE_LOG.txt`](TRACK_C_GATE_LOG.txt) |
| Job evidence | `artifacts/jobs/a8b16331-fdc2-43f2-ab16-4f7d555b40d0/evidence/` and `trackc-evidence.tar.gz` |

The pipeline ran the substance of the Track C gate unattended. It did not
follow the attended Incus-host procedure by hand or use a VGA observer. It
built and booted the media on the same Semaphore worker, polled the Incus
serial console, asserted the required evidence, and uploaded the evidence
bundle. The full transcript has 2,478 lines, including 361 `GATE` evidence
lines.

## Evidence state before this attempt

Before this run, IncusOS seed consumption had never been observed in any
environment. `RESCUE_DATA` detection and signed recovery acceptance were also
unobserved. The dated Phase 5.2 QEMU probe recorded a negative result; this
record does not revise that result.

The new result explains the earlier result rather than contradicting it. The
Phase 5.2 target-disk-growth oracle was correct in principle, but the installer
never reached its target write path in that run. Its plain-QEMU console was
also blind: that topology had no
`/dev/virtio-ports/org.linuxcontainers.incus`, and pinned
`reference/incus-os/incus-osd/internal/tui/tui.go:67-71` adds `/dev/ttyS0`
mirroring only when that port exists. In the accepted run, Incus supplied the
port and the serial console showed the installer and recovery paths. These two
tested venues support Incus, not plain QEMU, as the boot-acceptance venue.

## BOOT-05 through BOOT-08 observations

| Case | Observation | Result | Evidence and basis |
|---|---|---|---|
| BOOT-05 / O1 | Install completion | **Observed** | `install-serial.log` contains `IncusOS was successfully installed`, followed by `Please remove the install media to complete the installation`, 45 seconds after the install start. These are the pinned `install.go:388-390` completion strings. `source-seed.before.sha256` and `source-seed.before.list` establish the pre-install seed baseline. |
| BOOT-06 / O2 | Target seed consumption | **Observed** | `source-seed.before.sha256`, `source-seed.before.list`, `target-block-layout.txt`, `target-seed.after.sha256`, `target-seed.after.list`, and `source-seed.after.sha256` satisfy the target-side oracle. The result agrees with pinned `install.go:893-894` and `seed.go:31-36`, where post-install cleanup removes `install.json`, `install.yaml`, or `install.yml` from the target seed. |
| BOOT-07 / O3 | Raw `RESCUE_DATA` detection | **Observed** | `rescue-block-layout.txt` records the raw artifact's GPT partition and FAT label as `RESCUE_DATA`. `recovery-serial.log` contains `Recovery partition detected`, the pinned `recovery.go:50` detection string. |
| BOOT-08 / O4 | Signed recovery payload accepted | **Observed; developer-adjudicated** | `recovery-serial.log` contains `Update metadata detected, verifying signature` (`recovery.go:180`) and then `Processing validated update metadata` (`recovery.go:212`), which is reachable only after `util.VerifySMIME` succeeds. It continues through `Recovery actions completed` (`recovery.go:89`) and `System is starting up` (`main.go:657`); `Recovery failed:` is absent. `recovery-installed-applications.txt` and `recovery-startup-lines.txt` are the bounded adjudication extracts. The developer accepted this sequence as O4. |

The raw pipeline transcript deliberately ends with O4 marked
`ACCEPTED-NOT-ADJUDICATED` and the gate pending human review. That was the
pipeline's state at job completion. The developer's subsequent adjudication is
recorded here; the raw transcript remains unchanged.

### Target seed oracle

The source and target evidence is non-vacuous:

```text
source seed              /dev/loop0p2  8537e35609af63d8ecb6a65ff01d39b987ae3497055732705499be12c1397b3c
source entries                          applications.yaml install.yaml update.yaml
target seed              /dev/sdb2     84a75f5c85bdfccc7107758cc16f9204a0220ce03e7fa5c29f2560cf26569e92
target entries                          applications.yaml update.yaml
source seed after install               8537e35609af63d8ecb6a65ff01d39b987ae3497055732705499be12c1397b3c
```

`install.yaml` is present in the source and absent from the target, and the
partition digests differ. `applications.yaml` remains on the target as a
positive control: the target tar is readable and not empty. The source digest
is unchanged after installation. Together, these observations prove target
seed cleanup for this run without mistaking source mutation, an empty target,
or unrelated disk growth for consumption.

## What the campaign proves

A green documentation and build campaign proves that the documented commands
match the tested builder and that the builder emits media with the claimed
structure and content. It does not prove that IncusOS boots that media,
consumes the seed, detects `RESCUE_DATA`, or accepts the signed recovery
payload. BOOT-05 through BOOT-08 supply those runtime observations for the
execution identified above.

Conversely, this Track C pass does not make every documentation, build,
repository-settings, supply-chain, or post-tag case green. Release reviewers
must still apply the independent exit criteria in `FUNCTIONAL_TEST_PLAN.md`.

## Residual gaps and limits

- This is one observation on one venue at one upstream pin. It is not a
  guarantee for later IncusOS pins, other hypervisors, or future release media.
- **N-MEDIA-3 remains open.** The run used `image.type: raw`. Recovery matched
  the GPT `PARTLABEL=RESCUE_DATA` and never read the ISO primary volume
  descriptor. The NUL-padded ISO volume identifier therefore remains untested.
  A separate optional ISO run must show that Linux reports the identifier as
  exactly `RESCUE_DATA` and that IncusOS detects it before N-MEDIA-3 can close.
- The result covers an Incus VM with nested KVM, not bare metal.

## Release-review action

Record BOOT-05, BOOT-06, BOOT-07, BOOT-08, and BOOT-10 as passed. Record
BOOT-02 through BOOT-04 and BOOT-09 as passed by the unattended pipeline, with
the mechanism differences noted in the functional test plan. Treat the Track C
boot claim as observed for this run, retain N-MEDIA-3 as an open residual gap,
and decide the release only after the remaining blockers are resolved.
