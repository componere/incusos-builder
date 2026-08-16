---
title: How to verify boot acceptance
description: Run the manual x86_64 Linux Incus boot-acceptance checklist before a release tag
---

# How to verify boot acceptance

Run this checklist on an `x86_64` Linux Incus host before every release
tag until a CI boot gate succeeds. Pass only when you observe install
completion, a wiped installer seed, `RESCUE_DATA` detection, and the
signed recovery payload taking effect. Network traffic, a changed file
size, and source-overlay growth are not seed consumption.

Use one Incus VM for install and recovery. Copy the detached installed
volume; do not clone the VM.

## Prerequisites

- An `x86_64` Linux Incus host with `/dev/kvm`, QEMU 8.2 or newer, a
  managed storage pool, and a managed network. This checklist uses
  `default` and `incusbr0`.
- A SPICE client (`remote-viewer` or `spicy`) for VGA.
- VirtIO-SCSI for every IncusOS disk. VirtIO-BLK does not expose the
  drives IncusOS needs.
- Keep the built installer raw file unchanged. Run the gate on a
  working copy.

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

## 4. Record the seed, then install

The install seed is a tar archive at the start of the installer image's
second partition.

```bash
SEED_PART=$(lsblk -nrpo NAME,PARTN "$SOURCE_BLOCK" | awk '$2 == "2" {print $1; exit}')
test -n "$SEED_PART"

sudo dd if="$SEED_PART" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/seed-partition.before.sha256"
sudo dd if="$SEED_PART" bs=4M status=none \
  | tar -tf - >"$EVIDENCE/seed.before.list"
grep -E '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/seed.before.list"

incus start "$VM"
incus console "$VM" --type=vga
```

In another terminal:

```bash
incus console "$VM"
# Detach from serial with Ctrl+A, then Q.
incus console "$VM" --show-log | tee "$EVIDENCE/install-serial.log"
```

Secure Boot enrollment can run before the installer UI. Wait until the
installer reports completion. Do not treat network traffic as success.

## 5. Prove the seed was wiped

```bash
incus stop "$VM"
incus config device remove "$VM" install-media
incus config device remove "$VM" install-target

sudo blockdev --flushbufs "$SOURCE_BLOCK"
sudo dd if="$SEED_PART" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/seed-partition.after.sha256"
sudo dd if="$SEED_PART" bs=4M status=none \
  | tar -tf - >"$EVIDENCE/seed.after.list" 2>"$EVIDENCE/seed.after.tar.stderr" || true

BEFORE=$(cut -d' ' -f1 "$EVIDENCE/seed-partition.before.sha256")
AFTER=$(cut -d' ' -f1 "$EVIDENCE/seed-partition.after.sha256")
test "$BEFORE" != "$AFTER"
! grep -Eq '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/seed.after.list"
```

Both checks must pass: the second partition changed, and the
`install.*` seed recorded before boot is no longer readable. Stop here
if either check fails. Incus has no seed-consumption command; this
readback is host-side evidence.

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
incus start "$VM"
incus console "$VM" --type=vga
```

In another terminal:

```bash
incus console "$VM"
incus console "$VM" --show-log | tee "$EVIDENCE/recovery-serial.log"
```

A valid recovery disk has a FAT or ISO data partition labeled
`RESCUE_DATA`. Early in boot IncusOS looks for it, then tries
`hotfix.sh.sig` at the root and signed updates under `update/`.

Require all of the following:

1. VGA or serial evidence that the installed target booted.
2. Evidence that IncusOS detected `RESCUE_DATA`.
3. Evidence that the signed recovery payload was accepted and applied:
   the expected post-boot OS or application version or effect. Official
   docs do not publish a stable success string; do not match an
   invented log line.
4. The evidence archive listed below stored with the release record.

Fail the gate if any item is missing.

## 8. Archive evidence, then clean up

Keep with the release record:

- `incus-version.txt`
- `install-config.yaml` and `recovery-config.yaml`
- `artifact-sha256.txt` plus the builder `result.sha256` and
  `result.resources_sha256` values
- `seed-partition.before.sha256`, `seed.before.list`,
  `seed-partition.after.sha256`, `seed.after.list`
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
