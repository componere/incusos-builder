---
title: How to verify boot acceptance
description: Run the manual x86_64 Linux Incus boot-acceptance gate before a release candidate tag
---

# How to verify boot acceptance

Run this manual gate on an `x86_64` Linux Incus host before each release
candidate tag is first pushed. The tag-triggered `meigma/release` unit does not
run this procedure, and a successful release workflow cannot replace the four
observations below.

The gate has succeeded once. Track C completed the procedure on
2026-08-17 on a Semaphore Cloud `f1-standard-4` machine with nested KVM
(pipeline `4c5cc805`, job `a8b16331`). The archived evidence met all four
required observations:

1. Serial output reported `IncusOS was successfully installed` and then
   `Please remove the install media to complete the installation` 45
   seconds after start (`internal/install/install.go:388-390`).
2. Source seed `/dev/loop0p2` contained `applications.yaml`,
   `install.yaml`, and `update.yaml`. Target seed `/dev/sdb2` retained
   `applications.yaml` and `update.yaml` but not `install.yaml`; their
   digests differed, and the source digest remained unchanged.
3. Serial output reported `Recovery partition detected`
   (`internal/recovery/recovery.go:50`).
4. Serial output progressed from `Update metadata detected, verifying
   signature` to `Processing validated update metadata`, which is
   reachable only after `util.VerifySMIME` succeeds
   (`internal/recovery/recovery.go:180-212`). It then reported `Recovery
   actions completed` and `System is starting up`, while `Recovery
   failed:` was absent. A human adjudicated this observation as met.

That run establishes that this procedure is executable and its assertions
are satisfiable. It was one run on one venue, for IncusOS release
`202608102114` and upstream API pin
`v0.0.0-20260815030500-0f5b8057f2fc`; it does not guarantee later pins.
Continue to run the gate for every release candidate.

The earlier Phase 5.2 probe did not observe seed consumption or recovery
acceptance. Do not treat that probe, network traffic, a changed file size,
or source-overlay growth as a pass.

Pass only when you observe install completion, removal of the install
seed from the target seed-data partition, `RESCUE_DATA` detection, and
the signed recovery payload taking effect.

Use the same gate VM for install and recovery. Use a separate
throwaway Linux VM only to inspect the detached target volume. Copy
the detached installed volume; do not clone the gate VM.

## Prerequisites

- An `x86_64` Linux Incus host with `/dev/kvm`, a managed storage
  pool, and a managed network. This checklist uses `default` and
  `incusbr0`.
- An `x86_64` Linux VM image available to Incus for target inspection.
  The commands below use `images:ubuntu/24.04/cloud`.
- VirtIO-SCSI for every IncusOS disk. VirtIO-BLK does not expose the
  drives IncusOS needs.
- Keep the built installer raw file unchanged. Run the gate on a
  working copy.

Under Incus, the installer TUI mirrors to `/dev/ttyS0`: it adds that
device when `/dev/virtio-ports/org.linuxcontainers.incus` exists
(`internal/tui/tui.go:67-71`). Start the VM with
`incus start <vm> --console`; no SPICE or VGA client is required. Plain
QEMU does not provide that virtio port by default, so do not assume the
same serial behavior outside Incus.

## 1. Build the release media

Write a config with `image.type: raw`, `image.architecture: x86_64`,
`image.offline: true`, at least one application, and an install target
that can match only the 50 GiB volume:

```yaml
version: 1
image:
  type: raw
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications:
      - name: incus
  install:
    target:
      min_size: 50GiB
```

Build both artifacts and keep the published hashes:

```bash
incusos-builder build --json \
  -f release-gate.yaml \
  -o /absolute/path/seeded-x86_64.raw \
  --resources-output /absolute/path/rescue-data.raw
```

Record `result.sha256` (installer) and `result.resources_sha256`
(rescue media) from the JSON envelope. Offline builds cannot use
`-o -`.

## 2. Map the raw files to block devices

Incus `storage volume import` accepts only `backup` or `iso`. A disk
device can attach a host block device or an ISO file, not a raw disk
file. The `losetup` mapping is host glue; Incus documents
`source=/dev/...`.

```bash
#!/usr/bin/env bash
set -u

POOL=default
NETWORK=incusbr0
PROFILE=phase5-release-gate
VM=phase5-release-gate
TARGET_VOL=phase5-install-target
INSTALLED_VOL=phase5-installed-copy

SOURCE_RAW=/absolute/path/seeded-x86_64.raw
SOURCE_WORK=/absolute/path/seeded-x86_64.gate.raw
RESCUE_RAW=/absolute/path/rescue-data.raw
EVIDENCE=/absolute/path/phase5-manual-evidence
mkdir -p "$EVIDENCE"

test "$(uname -m)" = x86_64
test -b /dev/kvm
incus version | tee "$EVIDENCE/incus-version.txt"
incus storage show "$POOL"
incus network show "$NETWORK"

cp --reflink=auto --sparse=always -- "$SOURCE_RAW" "$SOURCE_WORK"
SOURCE_BLOCK=$(sudo losetup --find --show --partscan "$SOURCE_WORK")
RESCUE_BLOCK=$(sudo losetup --find --show --read-only --partscan "$RESCUE_RAW")
printf 'SOURCE_BLOCK=%s\nRESCUE_BLOCK=%s\n' "$SOURCE_BLOCK" "$RESCUE_BLOCK" \
  | tee "$EVIDENCE/loop-devices.txt"

sha256sum -- "$SOURCE_RAW" "$SOURCE_WORK" "$RESCUE_RAW" \
  | tee "$EVIDENCE/artifact-sha256.txt"
```

## 3. Create the profile, target, and empty VM

Do not rely on the host default profile.

```bash
incus profile create "$PROFILE"
incus profile device add "$PROFILE" root disk pool="$POOL" path=/
incus profile device add "$PROFILE" eth0 nic network="$NETWORK"

incus storage volume create "$POOL" "$TARGET_VOL" --type=block size=50GiB

incus init --empty --vm "$VM" \
  --profile "$PROFILE" \
  --config security.secureboot=false \
  --config limits.cpu=2 \
  --config limits.memory=4GiB \
  --device root,size=4GiB \
  --device root,boot.priority=10 \
  --device root,io.bus=virtio-scsi

incus config device add "$VM" vtpm tpm

incus config device add "$VM" install-media disk \
  source="$SOURCE_BLOCK" io.bus=virtio-scsi readonly=false boot.priority=30
incus config device add "$VM" install-target disk \
  pool="$POOL" source="$TARGET_VOL" io.bus=virtio-scsi boot.priority=20

incus config show "$VM" --expanded | tee "$EVIDENCE/install-config.yaml"
```

`security.secureboot=false` tells Incus not to load the default
Microsoft keys, so IncusOS can enroll its own. The VM still uses UEFI.
This is not legacy BIOS.

Add the software TPM before the first start; Incus cannot hot-plug a
TPM into a VM. Higher `boot.priority` boots first, so the installer
(`30`) precedes the 50 GiB target (`20`) and the dummy root (`10`).

## 4. Record the source seed baseline, then install

The install seed is a tar archive at the start of the source installer
image's seed-data partition. Record this input baseline for the later
comparison with the target seed partition. It is not the first half of
a before-and-after comparison on the source device.

```bash
SEED_PART=$(lsblk -nrpo NAME,PARTLABEL "$SOURCE_BLOCK" | awk '$2 == "seed-data" {print $1; exit}')
test -n "$SEED_PART"

sudo dd if="$SEED_PART" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/source-seed.before.sha256"
sudo dd if="$SEED_PART" bs=4M status=none \
  | tar -tf - >"$EVIDENCE/source-seed.before.list"
grep -E '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/source-seed.before.list"

incus start "$VM" --console
# Detach from serial with Ctrl+A, then Q.
incus console "$VM" --show-log | tee "$EVIDENCE/install-serial.log"
```

Secure Boot enrollment can run before the installer UI. Wait until the
installer reports completion. Do not treat network traffic as success.

## 5. Prove the target seed was cleaned

After the installer reports completion, stop the gate VM and detach
both the source installer and the target. Attach the target to a
throwaway Linux VM as its only data disk. This inspection works with
any Incus storage driver; do not assume that the host can loop-map the
managed volume.

```bash
incus stop "$VM"
incus config device remove "$VM" install-media
incus config device remove "$VM" install-target

INSPECT_VM=phase5-target-inspector
incus init images:ubuntu/24.04/cloud "$INSPECT_VM" \
  --vm --profile "$PROFILE" \
  --device root,boot.priority=10
incus config device add "$INSPECT_VM" inspect-target disk \
  pool="$POOL" source="$TARGET_VOL" io.bus=virtio-scsi boot.priority=0
incus start "$INSPECT_VM"

incus exec "$INSPECT_VM" -- udevadm settle
incus exec "$INSPECT_VM" -- \
  lsblk -o NAME,SIZE,TYPE,PTTYPE,FSTYPE,LABEL,PARTLABEL \
  | tee "$EVIDENCE/target-block-layout.txt"
TARGET_SEED=$(incus exec "$INSPECT_VM" -- \
  lsblk -nrpo NAME,PARTLABEL \
  | awk '$2 == "seed-data" {print $1; exit}')
test -n "$TARGET_SEED"

incus exec "$INSPECT_VM" -- \
  dd if="$TARGET_SEED" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/target-seed.after.sha256"
incus exec "$INSPECT_VM" -- \
  dd if="$TARGET_SEED" bs=4M status=none \
  | tar -tf - >"$EVIDENCE/target-seed.after.list"

SOURCE_BEFORE=$(cut -d' ' -f1 "$EVIDENCE/source-seed.before.sha256")
TARGET_AFTER=$(cut -d' ' -f1 "$EVIDENCE/target-seed.after.sha256")
grep -Eq '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/source-seed.before.list"
! grep -Eq '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/target-seed.after.list"
test "$SOURCE_BEFORE" != "$TARGET_AFTER"

incus stop "$INSPECT_VM"
incus config device remove "$INSPECT_VM" inspect-target
incus delete "$INSPECT_VM"
```

All three assertions must pass: the source baseline contains an
`install.*` entry, the target listing does not, and the source and
target digests differ. Stop here if any assertion fails.

The source is expected to remain byte-identical. At pinned upstream
commit `0f5b8057f2fc`, the installer copies partition 2 to the target,
then calls `CleanupPostInstall` to delete `install.*` from the target
seed partition.

Optionally confirm that the source itself is unchanged:

```bash
sudo blockdev --flushbufs "$SOURCE_BLOCK"
sudo dd if="$SEED_PART" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/source-seed.after.sha256"
SOURCE_AFTER=$(cut -d' ' -f1 "$EVIDENCE/source-seed.after.sha256")
test "$SOURCE_BEFORE" = "$SOURCE_AFTER"
```

The earlier Phase 5.2 probe observed zero target-disk growth. That
means the installer never reached its write path, not that it ignored
the seed. In `target-block-layout.txt`, no GPT means writes never
began. A GPT with a `seed-data` partition means the target write path
ran; if that partition still contains `install.*`, the installer ran
but did not consume the install seed.

## 6. Copy the installed volume and attach rescue media

```bash
incus storage volume copy \
  "$POOL/$TARGET_VOL" "$POOL/$INSTALLED_VOL" --volume-only

incus config device add "$VM" installed-target disk \
  pool="$POOL" source="$INSTALLED_VOL" io.bus=virtio-scsi boot.priority=20

incus config device add "$VM" rescue-data disk \
  source="$RESCUE_BLOCK" io.bus=virtio-scsi readonly=true boot.priority=0

incus config show "$VM" --expanded | tee "$EVIDENCE/recovery-config.yaml"
lsblk -o NAME,FSTYPE,LABEL,PARTLABEL "$RESCUE_BLOCK" \
  | tee "$EVIDENCE/rescue-block-layout.txt"
```

Attach the copy to the same VM. Official docs do not promise that a
cloned VM keeps the enrolled UEFI NVRAM and vTPM identity. Boot order
is now installed target `20`, dummy root `10`, rescue data `0`.

## 7. Accept recovery

```bash
incus start "$VM" --console
# Detach from serial with Ctrl+A, then Q.
incus console "$VM" --show-log | tee "$EVIDENCE/recovery-serial.log"
```

A valid recovery disk has a FAT or ISO data partition labeled
`RESCUE_DATA`. Early in boot IncusOS looks for it, then tries
`hotfix.sh.sig` at the root and signed updates under `update/`.

With this guide's `image.type: raw` config, recovery finds the GPT
`PARTLABEL=RESCUE_DATA` before it considers a filesystem label. The run
does not exercise the NUL-padded ISO volume identifier, so `N-MEDIA-3`
remains open.

Require all of the following:

1. Serial evidence that the installed target booted.
2. Evidence that IncusOS detected `RESCUE_DATA`.
3. Evidence that the signed recovery payload was accepted and applied:
   the expected post-boot OS or application version or effect. Official
   docs do not publish a stable success string; do not match an
   invented log line.
4. The evidence archive listed below stored with the release record.

Fail the gate if any item is missing.

### Optional ISO-media variant

To close the ISO volume-identifier risk empirically, run this checklist
again as a separate gate run with distinct VM, volume, loop-device, and
evidence names. Change only `image.type` to `iso`, write the installer
and rescue artifacts to `.iso` files, and require the same recovery
evidence from the ISO rescue-media run.

[INFERENCE] Current util-linux likely resolves the ISO label correctly:
its iso9660 probe passes the fixed 32-byte PVD field to
`blkid_probe_set_label`, whose right-trimming uses `strlen`; the first
NUL therefore terminates the label at `RESCUE_DATA`. Only the separate
ISO-media run establishes that behavior on the release host.

## 8. Archive evidence, then clean up

Keep with the release record and inspect alongside the rehearsal draft and
workflow artifacts:

- `incus-version.txt`
- `loop-devices.txt`
- `install-config.yaml` and `recovery-config.yaml`
- `artifact-sha256.txt` plus the builder `result.sha256` and
  `result.resources_sha256` values
- `source-seed.before.sha256`, `source-seed.before.list`,
  `target-seed.after.sha256`, and `target-seed.after.list`
- `source-seed.after.sha256` when the optional source-immutability
  check is run
- `target-block-layout.txt`
- `install-serial.log` and `recovery-serial.log`
- `rescue-block-layout.txt`

Then:

```bash
incus stop "$VM"
incus config device remove "$VM" rescue-data
incus config device remove "$VM" installed-target
incus delete "$VM"
incus storage volume delete "$POOL" "$INSTALLED_VOL"
incus storage volume delete "$POOL" "$TARGET_VOL"
incus profile delete "$PROFILE"

sudo losetup --detach "$RESCUE_BLOCK"
sudo losetup --detach "$SOURCE_BLOCK"
```

Delete `$SOURCE_WORK` only after the required evidence is archived.
The instance must be stopped before `incus delete`. Custom volumes
remain until you delete them.
