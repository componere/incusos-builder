---
title: Track C execution options — economical routes to boot acceptance
plan: .journal/005/FUNCTIONAL_TEST_PLAN.md
date: 2026-08-16
researchers: ProviderScan (deep-researcher), EphemeralCI (deep-researcher), TechFloor (researcher)
status: for decision
---

# Track C — how to actually run boot acceptance without owning an x86 box

## The headline is not a price

**Stop before renting anything: BOOT-06 as written cannot pass.** It inspects
the wrong disk. Fix the checklist first, then buy an hour of hardware.

At the pinned upstream commit `0f5b8057f2fc`, the installer does **not** wipe the
seed from the installer media. It copies partition 2 to the **target**, then
deletes `install.*` from the target:

```go
// Remove the install seed from the target device, and copy any external user-provided seeds.
err = seed.CleanupPostInstall(ctx, fmt.Sprintf("%s%s2", targetDevice, targetPartitionPrefix))
```
`reference/incus-os/incus-osd/internal/install/install.go:893-894`; the helper at
`seed.go:31-36` does the `tar --delete`. The source is opened `O_RDONLY`
(`install.go:1023`), the target `O_WRONLY` (`install.go:1061`).

BOOT-06 hashes `SEED_PART` on the **source** block device before and after, and
asserts the digest changed and `install.*` disappeared. The source is expected to
be byte-identical afterwards, so that assertion can never hold. **The step is
unpassable as written, on any host, at any price.**

### Corrected oracle

- BOOT-05's source capture is still valid, but only as an *input baseline* —
  rename it `source-seed.before.*`.
- After the installer reports completion and the VM is stopped, detach the target
  volume from the gate VM and attach it alone to a throwaway Linux VM, then:
  ```bash
  udevadm settle
  TARGET_SEED=$(lsblk -nrpo NAME,PARTLABEL | awk '$2=="seed-data"{print $1;exit}')
  dd if="$TARGET_SEED" bs=4M | sha256sum
  dd if="$TARGET_SEED" bs=4M | tar -tf -
  ```
- Assert: the source baseline **contains** `install.*`; the target listing
  **lacks** it; source and target digests differ; and optionally that the source
  post-install digest still equals its pre-install value.

### What this says about the Phase 5 probe

`docs/notes/phase-5-boot-probe.md` used target-disk growth as its oracle and saw
none. That oracle was right in principle — a successful install copies partitions
to the target and then cleans it — so **zero growth means the installer never
reached its write path**, not that the seed was ignored. To distinguish "install
never started" from "install ran but did not consume the seed", check the target
for a GPT and a `seed-data` partition: no GPT means writes never began.

## Second correction: BOOT-07 does not test the ISO risk

The Track C config is `image.type: raw`, so the rescue artifact is a GPT disk with
`PARTLABEL=RESCUE_DATA` and recovery matches the partlabel first. The NUL-padded
ISO volume identifier (`N-MEDIA-3`) is therefore **never exercised** by BOOT-07 as
configured. `[INFERENCE]` current util-linux likely copes anyway — `iso9660.c`
hands the 32-byte PVD field to `blkid_probe_set_label`, which right-trims via
`strlen`, so the first NUL terminates the label at exactly `RESCUE_DATA`. Closing
that risk empirically needs an **additional ISO-media run**, not the current one.

## Cost floor

A physical TPM 2.0 chip is **not** required. Incus's `tpm` device is a software
TPM ("Incus uses a software TPM that supports TPM 2.0"), and QEMU's emulator
backend talks to `swtpm`. Any host with working `/dev/kvm` qualifies — which is
what makes the cheap routes viable.

The real requirement is `/dev/kvm` on x86_64. That means **bare metal** (native)
or a cloud VM whose provider **documents nested virtualization**.

### Paid infrastructure, priced for a 1–3 hour attended run

| Option | Route | Cost for 1–3 h | Notes |
|---|---|---|---|
| **Scaleway EM-A610R-NVMe** | bare metal | **€0.11–€0.33** | official `/dev/kvm` tutorial; highest-confidence immediate start |
| phoenixNAP s0.d1.small | bare metal | $0.087–$0.261 | cheapest *if* stock is live; 3–15 min deploy; stock unverifiable without auth |
| AWS m7i-flex.large | nested-virt VM | $0.103–$0.310 | best route if an AWS account is already warm |
| Hetzner Server Auction | bare metal | €0.0989/h incl. mandatory IPv4 | genuinely hourly now, no setup fee, but provisioning usually within one business day |
| Vultr E-2286G | bare metal | $0.275–$0.825 | reliable fallback |
| Oracle VM.Standard3.Flex | nested-virt VM | $0.055–$0.165 | cheapest nominal, but conditional on image setup and an `x86_64_v3` preflight |

Documented **negative** results — do not retry these: several mainstream cloud
VM products do not permit nested virtualization. GCP/Azure exact totals were left
**unconfirmed** rather than guessed.

### Near-free ephemeral CI

| Option | Cost | Why it is interesting |
|---|---|---|
| **Semaphore Cloud f1-standard-4** | $0 under the $15 recurring monthly credit | officially documented nested virt/libvirt, 4 CPU / 16 GB / 65 GB, and — crucially — first-party **`sem debug job --duration 3h`** SSH plus `sem port-forward` |
| Blacksmith 2-vCPU x64 | $0 recurring quota | official KVM, 80 GB, easiest GitHub workflow migration; bring your own tunnel |
| BuildJet 4-vCPU | first run free under $5 credit | official nested virt, 64 GB |
| Namespace 4-vCPU | pay-as-you-go | official `/dev/kvm`; 3 h maximum |

**GitHub-hosted standard runners do expose `/dev/kvm`** (since April 2024, via an
official udev recipe) — but generic nested VMs remain explicitly *experimental and
unsupported*, and the standard runner offers only ~14 GB of disk, which does not
fit a 50 GiB target volume plus a 3.4 GB installer. Worth one free preflight;
not worth planning the gate around.

### The console problem

Track C requires a human watching a VGA/SPICE console — the checklist forbids
inferring success from side effects. That is what rules most CI out and makes
Semaphore's attended 3-hour debug session the standout free option: run
`remote-viewer` under Xvfb, expose noVNC, forward the port, and watch from the
Mac. On rented metal, the equivalent is an SSH-tunnelled SPICE session with the
Incus API kept loopback-only.

## Pre-stage to shrink the paid window

Build the artifacts on the M4 **before** the clock starts — the plan already
permits a macOS build host. Transfer only `seeded-x86_64.raw`,
`rescue-data.raw`, the exact `release-gate.yaml`, and the JSON envelope with both
digests. That is about **3.76 GB decimal**, not the 15 GB the plan implies — the
15 GB figure is a local workspace budget, not the transfer pair. Compress with
`zstd`, transfer with `rsync --partial --append-verify`, and verify the
**uncompressed** digests against `result.sha256` / `result.resources_sha256`.

Disk caveat: the ~22 GB estimate holds only with a sparse/reflink source copy and
a thin or COW Incus pool. On a thick-provisioned pool, budget roughly **120 GiB**
free or volume creation will fail despite little data being written.

## Recommendation

1. **Fix BOOT-06 and BOOT-05's naming first.** It is free, it is a documentation
   change, and without it the run cannot pass.
2. **Try Semaphore Cloud first** — $0 under the recurring credit, documented
   nested virt, and the only first-party attended 3-hour session with port
   forwarding. Fail fast on a preflight (`uname -m`, CPU flags, `/dev/kvm`, 25+ GB
   free, passwordless sudo, a tiny KVM guest) before spending the window.
3. **If the preflight fails, rent Scaleway EM-A610R-NVMe** for about €0.11–€0.33.
   Native KVM, no nested-virt question, official documentation.
4. Add a **separate ISO-media run** if you want `N-MEDIA-3` closed empirically;
   the current raw configuration will never exercise it.

Total realistic cost: **€0 to €0.35** plus one to three hours of attended
operator time.


---

# Appendix — verbatim researcher reports

## TechFloor (technical floor)

# Technical floor for Track C

## Direct answer

The **guest-behavior floor** does not require Incus. A plain `qemu-system-x86_64` VM can produce the four substantive observations if it uses KVM, an `x86_64-v3` CPU model, OVMF/UEFI with a writable persistent variable store, a persistent TPM 2.0 `swtpm` state directory, VirtIO-SCSI disks, a 50 GiB target, and a graphical console. QEMU documents that its emulator TPM backend talks to the external `swtpm` process; no host TPM passthrough is necessary ([QEMU TPM docs, lines 280–348](https://github.com/qemu/qemu/blob/master/docs/specs/tpm.rst#L280-L348)).

The **current documented release gate**, however, does require Incus: it names an `x86_64` Linux Incus host, commands and evidence files specific to Incus, and says to run that checklist before every release tag (`docs/docs/how-to/verify-boot-acceptance.md:8-28,248-277`). A QEMU/libvirt run could prove the IncusOS behaviors, but it would not be a conforming execution of the gate without first amending the gate and defining replacement evidence.

A physical TPM 2.0 chip is **not required**. Incus says its `tpm` device “enable[s] access to a TPM emulator” and that “Incus uses a software TPM that supports TPM 2.0” ([Incus `tpm` device reference](https://linuxcontainers.org/incus/docs/main/reference/devices_tpm/)). QEMU identifies that emulator as `swtpm` and shows an x86 `swtpm socket --tpm2` plus `-tpmdev emulator` configuration ([QEMU TPM docs, lines 304–348](https://github.com/qemu/qemu/blob/master/docs/specs/tpm.rst#L304-L348)). Thus a rented nested-virtualization VM qualifies without a physical TPM, provided `/dev/kvm` works and `swtpm` can run.

## Release-blocking checklist defect: BOOT-06 reads the wrong disk

This should be corrected **before paying for a host**. At pinned upstream commit [`0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12`](https://github.com/lxc/incus-os/tree/0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12), the installer does not delete `install.*` from the source installer media. It copies partition 2 to the target and deletes `install.*` from the **target partition**.

The load-bearing upstream call is verbatim:

```go
// Remove the install seed from the target device, and copy any external user-provided seeds.
err = seed.CleanupPostInstall(ctx, fmt.Sprintf("%s%s2", targetDevice, targetPartitionPrefix))
```

`reference/incus-os/incus-osd/internal/install/install.go:893-894`; permanent source: [install.go#L893-L894](https://github.com/lxc/incus-os/blob/0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12/incus-osd/internal/install/install.go#L893-L894).

The helper is equally explicit:

```go
// CleanupPostInstall will remove the seed install from the target partition and copy any
// external user-provided seeds.
func CleanupPostInstall(ctx context.Context, targetSeedPartition string) error {
    // Remove the install configuration file, if present, from the target seed partition.
    for _, filename := range []string{"install.json", "install.yaml", "install.yml"} {
        _, err := subprocess.RunCommandContext(ctx, "tar", "-f", targetSeedPartition, "--delete", filename)
```

`reference/incus-os/incus-osd/internal/seed/seed.go:31-36`; permanent source: [seed.go#L31-L36](https://github.com/lxc/incus-os/blob/0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12/incus-osd/internal/seed/seed.go#L31-L36).

The preceding loop copies source partitions 2 through 5 or 8 to the target (`install.go:886-894`), and `doCopy` opens the source partition `O_RDONLY` and the target partition `O_WRONLY` (`install.go:1022-1063`; [permanent source](https://github.com/lxc/incus-os/blob/0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12/incus-osd/internal/install/install.go#L1022-L1063)). Upstream documentation also says the installer wipes the seed archive “from the final install,” not from the installer media (`reference/incus-os/doc/reference/seed.md:17-23`).

By contrast, the local checklist derives `SEED_PART` from `SOURCE_BLOCK` before boot, then hashes that same source partition after boot and requires the digest to differ (`docs/docs/how-to/verify-boot-acceptance.md:149-156,180-189`). During a normal successful install, that **source seed-data partition is expected to remain byte-identical**. The whole source overlay may grow because firmware/key enrollment writes elsewhere—the prior probe observed that—but partition 2 is opened read-only by the installer. Therefore BOOT-06's source digest inequality is unpassable as written; a difference would indicate an unrelated writer or corruption, not the intended seed cleanup.

### Corrected oracle

BOOT-05's source capture is useful, but it is an **input baseline**, not the “before” state of a partition expected to mutate. Rename it conceptually to `source-seed.before.{sha256,list}`. The correct post-install observation is the 50 GiB target's partition with `PARTLABEL=seed-data`:

1. Before boot, hash/list `$SOURCE_SEED_PART` and require `install.*` to be present, as BOOT-05 already does (`verify-boot-acceptance.md:149-156`).
2. After the installer reports completion, stop the VM and detach `$TARGET_VOL` from the gate VM.
3. Attach that custom **block** volume to a temporary Linux VM. Incus documents that block custom volumes attach only to VMs and shows `incus config device add <instance> <device> disk pool=<pool> source=<volume>` ([Incus storage-volume docs, “Attach the volume to an instance”](https://linuxcontainers.org/incus/docs/main/howto/storage_volumes/#attach-the-volume-to-an-instance)). Do not attach it to two VMs simultaneously; Incus explicitly warns against that on the same page.
4. In the inspector VM, locate the unique copied target partition and stream it back to the host:

```bash
incus exec "$INSPECTOR" -- udevadm settle
incus exec "$INSPECTOR" -- lsblk -nrpo NAME,SIZE,TYPE,PARTLABEL \
  | tee "$EVIDENCE/target-block-layout.txt"
TARGET_SEED=$(incus exec "$INSPECTOR" -- lsblk -nrpo NAME,PARTLABEL \
  | awk '$2 == "seed-data" {print $1; exit}')
test -n "$TARGET_SEED"

incus exec "$INSPECTOR" -- dd if="$TARGET_SEED" bs=4M status=none \
  | sha256sum | tee "$EVIDENCE/target-seed.after.sha256"
incus exec "$INSPECTOR" -- dd if="$TARGET_SEED" bs=4M status=none \
  | tar -tf - >"$EVIDENCE/target-seed.after.list"

SOURCE_BEFORE=$(cut -d' ' -f1 "$EVIDENCE/source-seed.before.sha256")
TARGET_AFTER=$(cut -d' ' -f1 "$EVIDENCE/target-seed.after.sha256")
test "$SOURCE_BEFORE" != "$TARGET_AFTER"
! grep -Eq '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/target-seed.after.list"
```

5. As a negative control, re-hash the source partition and require `source-before == source-after`. This proves the target changed while the preserved installer did not.
6. Detach the volume from the inspector, copy it as the checklist requires, and reattach the installed copy to the original gate VM for recovery. Preserve the original gate VM's OVMF NVRAM and vTPM state (`verify-boot-acceptance.md:199-217`).

A blank target has no seed partition before installation, so there is no same-device “before” hash to take. Comparing the source input partition with the post-install target partition is the correct invariant: the installer first copies source partition 2 and then deletes `install.*` from that target copy (`install.go:886-895`; `seed.go:31-40`).

### Reconciliation with the Phase 5 probe

The probe's **target-disk growth** oracle was correct in principle. A successful install wipes the target (`install.go:729-733`), creates/copies partitions, and then edits target partition 2 (`install.go:886-895`). `phase-5-boot-probe.md:41-50` records that the target qcow2 did not grow at all, while only the source overlay and network showed activity. That means the run did not reach any target-writing stage; it does not mean the source seed should have changed.

The probe does not distinguish “the installer never began” from “the installer began but failed before the first target write.” The removed harness and committed JSON do not record the target's virtual capacity, bus, or post-run GPT (`docs/notes/phase-5-boot-evidence.json`). The decisive experiment is post-stop target inspection:

- no target GPT/partitions: installation did not reach target provisioning;
- target GPT plus `seed-data` still containing `install.*`: it reached the copy but not successful cleanup;
- target GPT plus `seed-data` without `install.*`, and a changed digest relative to the source baseline: seed consumption succeeded;
- VGA installer-completion output distinguishes a completed install from all partial states (`verify-boot-acceptance.md:160-171`).

BOOT-05 does **not** need a different device for its pre-install capture. It correctly proves the source contains the trigger seed. The confusion begins when BOOT-06 assumes that same source partition is the object cleaned by installation.

## Acceptance observations versus procedural choices

### Hard observations

The release gate's substantive result is exactly four observations: installer completion on VGA/serial; a copied target seed partition whose digest differs and whose `install.*` member is gone; `RESCUE_DATA` detection; and the signed recovery update taking effect (`verify-boot-acceptance.md:13-15,192-195,220-246`). A file-size change, source-overlay growth, or network traffic is not a substitute (`verify-boot-acceptance.md:8-15`).

### Hard guest/platform constraints

- **KVM-capable x86_64 host and x86_64-v3 guest CPU.** The checklist requires an x86_64 Linux `/dev/kvm` host (`verify-boot-acceptance.md:22-24`); pinned upstream additionally requires `x86_64_v3` (`reference/incus-os/doc/getting-started/requirements.md:8-14`). A nested-virt cloud VM must expose both `/dev/kvm` and sufficient CPUID features; “x86_64” alone is not enough.
- **UEFI Secure Boot state, not legacy BIOS.** `security.secureboot=false` means Incus does not preload Microsoft keys; IncusOS enrolls its own keys into UEFI setup-mode NVRAM (`verify-boot-acceptance.md:135-141`; `reference/incus-os/doc/getting-started/installation/virtual-incus.md:50-69`). Plain QEMU must retain the writable OVMF VARS file across enrollment, install, and recovery.
- **TPM 2.0 visible before first boot.** Incus cannot hot-plug a TPM into a VM (`verify-boot-acceptance.md:139-141`), but the backend can be `swtpm`; no host TPM chip is needed ([Incus TPM docs](https://linuxcontainers.org/incus/docs/main/reference/devices_tpm/)). Plain QEMU must preserve the same `swtpm` state directory across both phases.
- **VirtIO-SCSI, not VirtIO-BLK.** Upstream explicitly says VirtIO-BLK disks do not appear to IncusOS the same way physical drives do (`reference/incus-os/doc/getting-started/installation.md:12-14`). The installer enumerates NVMe, SCSI, MMC and virtual devices and drops entries without a usable ID (`install.go:452-568`), so do not substitute a different bus based only on QEMU's ability to boot it.
- **50 GiB target.** Upstream rejects a target smaller than 50 GiB (`install.go:270-299`); the checklist uses a 50 GiB volume and a 4 GiB dummy root so only the target matches `min_size: 50GiB` (`verify-boot-acceptance.md:114-130`).
- **At least 4 GiB guest RAM.** Pinned upstream says at least 4 GiB “for system use only” (`requirements.md:12`); the checklist assigns 4 GiB and 2 vCPU (`verify-boot-acceptance.md:118-121`). Upstream's Incus VM example works with one vCPU (`virtual-incus.md:35-40`), so 2 vCPU is the gate's chosen floor, not an upstream architectural minimum.
- **A graphical console is practically hard.** The shipped x86_64 UKI has no `console=` and suppresses kernel/systemd status, so post-handoff serial silence carries little information (`docs/notes/spike-1e-boot.md:85-118,461-469`). Incus officially supports VGA via `incus console <vm> --type vga` with `remote-viewer` or `spicy` ([Incus console docs](https://linuxcontainers.org/incus/docs/main/howto/instances_console/#access-the-graphical-console-for-virtual-machines)). Plain QEMU can equivalently bind VNC/SPICE to loopback and tunnel it, for example `ssh -L 5900:127.0.0.1:5900 host` with QEMU VNC display `127.0.0.1:0`.
- **A virtual NIC is required, but outbound Internet is not required for the install/recovery payload.** Upstream lists one wired network port (`requirements.md:14`). For an offline build, the customizer sets update checks to `never` (`reference/incus-os/incus-osd/cmd/image-customizer/main.go:436-443`), the install path returns through `DoInstall` before normal startup (`cmd/incus-osd/main.go:270-281`), and recovery consumes the attached local update tree early in startup (`cmd/incus-osd/main.go:408-420`). Keep the NIC for supported topology and post-boot inspection, but a provider egress path is not a prerequisite for copying the OS or applying the attached signed payload.

### Procedural conveniences

`default`, `incusbr0`, `losetup`, and SPICE are not properties of IncusOS. They are concrete host glue chosen by the reproducible checklist (`verify-boot-acceptance.md:22-28,65-103`). A different pool/network name, direct host block file, libvirt, VNC, or raw QEMU can present the same guest hardware and observations. Incus itself is similarly not required by the guest.

What is lost by deviating is evidence comparability: no `incus-version.txt`; no expanded Incus install/recovery configs; no proof that Incus mapped `io.bus=virtio-scsi`, boot priorities, loop devices, and the vTPM as documented; and no test of `incus storage volume copy --volume-only` while retaining the same VM's NVRAM/TPM identity (`verify-boot-acceptance.md:199-217,248-259`). Record QEMU version, complete command line, OVMF code hash, VARS hash before/after, `swtpm` version/state identity, disk maps, and framebuffer capture if the gate is formally changed to accept QEMU.

**Recommendation:** correct BOOT-06, then follow the Incus checklist for the release record. Use plain QEMU/KVM only as a diagnostic or after an explicit gate amendment. The prior QEMU failure was a TCG/incorrect-topology negative run, not proof that KVM-backed QEMU is technically incapable.

## `RESCUE_DATA` lookup and the NUL-padding finding

Pinned IncusOS first checks `/dev/disk/by-partlabel/RESCUE_DATA`, then falls back to `/dev/disk/by-label/RESCUE_DATA`; it mounts only `vfat` or `iso9660` (`reference/incus-os/incus-osd/internal/recovery/recovery.go:31-65`; [permanent source](https://github.com/lxc/incus-os/blob/0f5b8057f2fcb2bed6408f7be3e0ac1602d23f12/incus-osd/internal/recovery/recovery.go#L31-L65)).

The current Track C config is `image.type: raw`, so its rescue file is GPT+FAT32. The builder sets both the GPT partition name and FAT label to `RESCUE_DATA` (`internal/media/fat.go:31-49`). Recovery therefore succeeds through the preferred **partition-label** path; the ISO PVD padding is not involved. Consequently, BOOT-07 as written does **not** close N-MEDIA-3. An empirical ISO check requires attaching an ISO rescue artifact in an additional run.

For ISO media, the NUL padding is standards-nonconforming but current Linux `libblkid` appears tolerant. The ISO probe passes all 32 PVD bytes to `blkid_probe_set_label` ([util-linux `iso9660.c` lines 421–427](https://github.com/util-linux/util-linux/blob/master/libblkid/src/superblocks/iso9660.c#L421-L427)); the label setter copies them and right-trims through a helper whose first operation is `strlen`, so the first NUL terminates the value at `RESCUE_DATA` ([`superblocks.c` lines 718–740](https://github.com/util-linux/util-linux/blob/master/libblkid/src/superblocks/superblocks.c#L718-L740), [`strutils.h` lines 346–363](https://github.com/util-linux/util-linux/blob/master/include/strutils.h#L346-L363)). systemd's persistent-storage rule imports `blkid` data and creates `/dev/disk/by-label/$ID_FS_LABEL_ENC` ([systemd rule](https://github.com/systemd/systemd/blob/main/rules.d/60-persistent-storage.rules.in)). This source reading **refutes the specific fear for current util-linux**, but does not prove the exact util-linux/systemd build shipped in pinned IncusOS. The settling experiment is `blkid -s LABEL -o value rescue.iso` plus checking `/dev/disk/by-label/RESCUE_DATA` on Linux, followed by an ISO recovery boot.

## Resource envelope

### Guest and host

- Gate VM: **2 vCPU, 4 GiB RAM, one 50 GiB target**, plus the 4 GiB dummy root and attached installer/rescue (`verify-boot-acceptance.md:114-130`). A provider host should have at least **2 x86_64-v3 vCPU and 8 GiB RAM** so the Linux/Incus host has headroom beyond the non-negotiable 4 GiB guest allocation. The 8 GiB host figure is an operational minimum, not an upstream IncusOS requirement.
- Logical disk provisioned by the literal workflow is over **110 GiB**: two simultaneously existing 50 GiB block volumes (target and installed copy), 4 GiB dummy root, about 3.43 GB installer, its working copy, rescue media, evidence, and host OS (`verify-boot-acceptance.md:95-103,114-130,199-217`). The plan's **22 GB physical-working-space** estimate is credible only with a verified sparse/reflink source copy and thin/COW Incus pool. On a thick-provisioned pool, request roughly **120 GiB free**; otherwise volume creation/copy can fail despite low written data.
- For release `202608102114`, the x86 raw installer is exactly **3,432,026,112 bytes** (`docs/notes/spike-1e-boot.md:344-352`). The rescue writer stages only the selected `x86_64/incus.raw.gz` plus `update.json` and `update.sjson` (`internal/build/build.go:206-239`); the published asset is 319,785,504 bytes ([release metadata](https://images.linuxcontainers.org/os/202608102114/update.json)). Applying the builder's sizing formula (`internal/media/fat.go:63-72`) gives a raw rescue image of about **329,374,720 bytes**. Thus the two transfer artifacts are about **3.76 GB decimal (3.50 GiB)**, not 15 GB. The 15 GB figure is a conservative local workspace budget, not pair size.

### Time

The campaign plan budgets BOOT-02 at 25–60 minutes, install at 20–60 minutes, and recovery at 15–40 minutes (`.wt/journal-jmgilman/.journal/005/FUNCTIONAL_TEST_PLAN.md:456-458,2813-2897`). Those are ceilings, not measurements: the M4 campaign measured a warm offline debug build in 5.3 seconds and ordinary raw builds in 17 seconds (`.wt/journal-jmgilman/.journal/005/WAVE2_TRACKA_RESULTS.md:117-120,197-203,839-852`), while the earlier x86 raw image download/decompress took under 30 seconds (`spike-1e-boot.md:344-352`). The `incus` offline build downloads roughly 928 MB of compressed OS+application inputs from the cited release metadata, so network speed and a cold cache dominate local build time.

A **1–3 hour operator reservation remains prudent**: host/Incus setup and transfer; 20–60 minutes watching install; minutes for corrected target inspection/copy; 15–40 minutes for recovery; then evidence archive and cleanup. KVM should remove TCG's emulation penalty, but no successful end-to-end timing exists yet, so narrowing the reservation further would be unsupported.

## Pre-stage off-host

Build on the M4 before starting the paid clock. The builder selects `architecture: x86_64` as data and the test plan explicitly permits a macOS build host (`FUNCTIONAL_TEST_PLAN.md:2813-2839`). Transfer only:

- `seeded-x86_64.raw`;
- `rescue-data.raw`;
- the exact `release-gate.yaml`;
- the JSON result envelope containing both published SHA-256 values.

Do not transfer the content-addressed cache or build scratch. Uncompressed transfer is about 3.76 GB for the cited release. To reduce bytes, compress each output locally with `zstd`, transfer with a resumable method, decompress on the rented host, and verify the **uncompressed** published digests:

```bash
# macOS
shasum -a 256 seeded-x86_64.raw rescue-data.raw
zstd -T0 --long=31 seeded-x86_64.raw -o seeded-x86_64.raw.zst
zstd -T0 --long=31 rescue-data.raw -o rescue-data.raw.zst
rsync -avP --partial --append-verify \
  seeded-x86_64.raw.zst rescue-data.raw.zst release-gate.yaml result.json host:/work/

# rented Linux host
unzstd seeded-x86_64.raw.zst
unzstd rescue-data.raw.zst
sha256sum seeded-x86_64.raw rescue-data.raw
# Compare exactly with result.sha256 and result.resources_sha256.
```

Compression and decompression preserve the byte stream, so the builder hashes remain the acceptance authority. `[INFERENCE]` The compressed transfer should be near the already-compressed upstream OS plus application payload (roughly 0.93 GB), but measure the actual `.zst` files rather than budgeting from that estimate.

## Remaining unknowns

1. The exact target capacity and disk bus used by the removed Phase 5.2 CI harness are not present in `phase-5-boot-evidence.json`; the committed result proves only zero target writes, not why the write path was never reached.
2. No successful Track C run provides measured KVM install/recovery duration.
3. Current util-linux source handles the NUL-padded ISO label, but the emitted ISO has not been tested against the exact `blkid`/udev versions inside pinned IncusOS. The current raw Track C does not exercise that path.
4. The corrected target-inspection workflow needs one disposable Linux VM or a storage-driver-specific host mapping; the helper-VM method is portable across Incus storage drivers but adds a few minutes and a small temporary root volume.

## ProviderScan (paid infrastructure)

## Scope and common technical facts

- Track C requires an x86_64 Linux **L1 host with working `/dev/kvm`**, not QEMU TCG; the repository checklist also requires the operator to observe the L2 VM through `incus console --type=vga` (`docs/docs/how-to/verify-boot-acceptance.md:13-18,86-112,136-148`). IncusOS itself requires an `x86_64_v3` CPU, UEFI, TPM 2.0, 4 GiB RAM, and 50 GiB storage: https://linuxcontainers.org/incus-os/docs/main/getting-started/requirements/ .
- A physical TPM is **not** required. Incus documents that its `tpm` device is an emulator and “uses a software TPM that supports TPM 2.0”: https://linuxcontainers.org/incus/docs/main/reference/devices_tpm/ . This materially broadens the host pool.
- Provider web/IPMI/serial consoles show the **outer host**, not the IncusOS guest. For every usable option below, the exact L2-console route is: bind Incus HTTPS to loopback (`incus config set core.https_address=127.0.0.1:8443`), create a trust token, SSH-forward local port 8443 to the L1 host, add that endpoint as an Incus remote on the Mac, then run local `incus console <remote>:<vm> --type=vga` so local `remote-viewer` carries SPICE through the tunnel. Incus documents the HTTPS listener/trust-token flow and the VGA/SPICE client command: https://linuxcontainers.org/incus/docs/main/howto/server_expose/ and https://linuxcontainers.org/incus/docs/main/howto/instances_console/ . An XQuartz `ssh -Y` session running `remote-viewer` on the L1 host is a fallback.
- All prices below are public list prices observed **2026-08-16**, before tax. Spot/preemptible capacity is a poor trade for an attended, stateful installer/recovery run: reclamation can destroy the observation window, while on-demand costs are pennies to tens of cents.

## Option: phoenixNAP Bare Metal Cloud `s0.d1.small` — cheapest high-confidence pick
- Route: bare metal
- KVM on x86_64: **yes** — phoenixNAP calls BMC single-tenant physical hardware with no hypervisor; `s0.d1.small` is an Intel Xeon E3-1240v3 (Haswell), 16 GB RAM, 2×240 GB SSD at $0.08/h. Haswell is the first Intel generation with the AVX2-era feature set needed by `x86_64_v3`; the provider also offers Proxmox/ESXi images, strong operational evidence that VT-x is enabled. Sources: https://phoenixnap.com/bare-metal-cloud and Intel CPU record https://www.intel.com/content/www/us/en/products/sku/75055/intel-xeon-processor-e31240-v3-8m-cache-3-40-ghz/specifications.html .
- Cost for a 1–3 h run: server $0.08/h plus the smallest public `/31` block, 2 addresses × $0.0035/h = **$0.087 for 1 h; $0.261 for 3 h, USD**. Every partial hour rounds up and the minimum is one hour; no contract or prepayment. Delete both server and IP allocation to stop billing. Source: https://phoenixnap.com/kb/phoenixnap-bare-metal-cloud-billing-models .
- Signup friction: account plus payment method; card verification is a temporary **$1 charge refunded within seven days**. No published government-ID requirement or account-approval SLA was found. Source: https://phoenixnap.com/kb/bare-metal-cloud-payment-management .
- Provisioning / image: provider advertises as little as 60 seconds; the deployment guide says OS-dependent deployment is normally **3–15 minutes**, Ubuntu Jammy is ready in about 3 minutes, and Ubuntu/Debian are catalog options. Capacity is stock-dependent by location. Sources: https://phoenixnap.com/bare-metal-cloud and https://phoenixnap.com/kb/how-to-deploy-bare-metal-cloud-server . Ubuntu 22.04 can use Zabbly packages.
- Guest console: the common Incus-API-over-SSH/SPICE tunnel above. phoenixNAP’s BMC remote console is useful only for recovering the L1 host.
- Spot/preemptible: no spot product is documented; use hourly on-demand. The billing page documents only hourly and reservations.
- Disqualifiers / risks: the cheapest SKU is old and inventory-dependent; verify `lscpu`, `/dev/kvm`, and an L2 `x86_64_v3` preflight before starting the acceptance clock. Hourly capacity is not reserved. Public IPv4 is not included in the $0.08 compute price.
- Confidence: **high** — direct physical hardware, known Haswell CPU, sufficient RAM/disk, exact hourly/minimum/IP pricing, and current provider provisioning docs.

## Option: Scaleway Elastic Metal `EM-A610R-NVMe`
- Route: bare metal
- KVM on x86_64: **yes** — dedicated AMD Ryzen PRO 3600, 32 GB RAM, 2×1.02 TB NVMe; the CPU is modern enough for `x86_64_v3`. Official configuration/rate: https://www.scaleway.com/en/pricing/elastic-metal/ .
- Cost for a 1–3 h run: **€0.11 / €0.33 EUR** at €0.11/h, before tax. Hourly billing has no commitment fee; the published unit is one hour. The default public IPv4 is included: https://www.scaleway.com/en/docs/elastic-metal/reference-content/elastic-metal-networking/ .
- Signup friction: self-service Scaleway account and payment method; automatic/manual fraud verification is possible, but I found no public approval-time SLA or fixed deposit. Create the account before the acceptance window.
- Provisioning / image: the one-call API path promises a fully operational server in **under 15 minutes**; the traditional OS installation can take up to an hour. Ubuntu and Debian images are offered. Sources: https://www.scaleway.com/en/docs/elastic-metal/api-cli/elastic-metal-with-api/ and https://www.scaleway.com/en/docs/elastic-metal/faq/ .
- Guest console: common Incus-API-over-SSH/SPICE tunnel.
- Spot/preemptible: none documented for Elastic Metal; hourly is already on-demand.
- Disqualifiers / risks: do **not** choose the cheaper €0.077/h `EM-A116X-SSD`: Scaleway promises only “Xeon E3-1220 or equivalent,” not the CPU revision, so `x86_64_v3` is not guaranteed. `EM-A610R-NVMe` avoids that ambiguity.
- Confidence: **high** for the selected Ryzen SKU; the only meaningful risk is stock/OS-install latency.

## Option: Vultr Bare Metal Intel E-2286G
- Route: bare metal
- KVM on x86_64: **yes** — Vultr describes single-tenant dedicated hardware with no virtualization layer. The qualifying modern SKU is Intel E-2286G, 32 GB RAM, 2×960 GB SSD. Official page: https://www.vultr.com/products/bare-metal/ .
- Cost for a 1–3 h run: **$0.275 / $0.825 USD**, published at $0.275/h with a $185 monthly cap. Storage, one public address, and listed bandwidth are included. The older $0.179/h “E3-1270” does not identify a CPU revision and therefore does not establish `x86_64_v3`; do not use it.
- Signup friction: self-service account/payment verification; Vultr may require account funding or fraud review depending on payment method. No public guaranteed approval time was found.
- Provisioning / image: on-demand console/API deployment; the product page does not publish a hard provisioning SLA. Recent Ubuntu/Debian are standard Vultr OS choices.
- Guest console: common Incus-API-over-SSH/SPICE tunnel; Vultr web console is L1 only.
- Spot/preemptible: no spot offering for this CPU bare-metal route is documented.
- Disqualifiers / risks: availability is region-specific and deployment timing is not guaranteed publicly.
- Confidence: **high** on hardware/rate; **medium** on same-hour stock and account timing.

## Option: Latitude.sh `m4.metal.small`
- Route: bare metal
- KVM on x86_64: **yes** — dedicated Intel Xeon E-2386G hardware; the current pricing catalog names `m4.metal.small` at $0.37/h: https://www.latitude.sh/pricing .
- Cost for a 1–3 h run: **$0.37 / $1.11 USD**, hourly on-demand. The public catalog did not disclose a smaller-than-one-hour increment, so budget whole hours.
- Signup friction: self-service account plus payment/fraud checks; no fixed deposit or verification SLA was confirmed from primary documentation.
- Provisioning / image: API-driven bare metal intended to provision in minutes; exact stock-dependent lead time was not contractually published. Ubuntu is a standard deployment image; confirm the current Debian/Ubuntu version at checkout.
- Guest console: common Incus-API-over-SSH/SPICE tunnel.
- Spot/preemptible: no applicable spot offering documented.
- Disqualifiers / risks: more expensive than phoenixNAP/Scaleway; exact billing granularity and image revision should be confirmed before purchase.
- Confidence: **medium** — qualifying modern hardware and public hourly rate, but less complete public billing/provisioning detail.

## Option: Hivelocity Instant Bare Metal
- Route: bare metal
- KVM on x86_64: **yes in principle** — the smallest modern configuration located in the current catalog uses a Xeon E-2336, which is `x86_64_v3`-capable; product family: https://www.hivelocity.net/bare-metal/ .
- Cost for a 1–3 h run: **unconfirmed from a stable public primary pricing page**; checkout/configurator pricing varies by location. Do not schedule this over phoenixNAP without obtaining the exact hourly rate, minimum, and setup charge first.
- Signup friction: account/payment verification; manual review is possible. No published approval SLA was confirmed.
- Provisioning / image: marketed as instant bare metal; exact SKU stock controls lead time. Ubuntu/Debian are available on dedicated servers.
- Guest console: common Incus-API-over-SSH/SPICE tunnel.
- Spot/preemptible: none documented.
- Disqualifiers / risks: public price/minimum was not independently reproducible, so it is not a priced credible winner for this decision.
- Confidence: **medium technically, low commercially**.

## Option: Equinix Metal
- Route: bare metal
- KVM on x86_64: formerly yes, but **service unavailable**.
- Cost for a 1–3 h run: **not purchasable**. Equinix Metal shut down on **2026-06-30 23:59 PST**, before this survey date: https://docs.equinix.com/metal/eos-faq/ .
- Signup friction: not applicable.
- Guest console: not applicable.
- Spot/preemptible: former spot market no longer matters.
- Disqualifiers / risks: hard disqualification — product is EOL and shut down.
- Confidence: **high**.

## Option: OVHcloud / So you Start / Kimsufi (OVH Eco)
- Route: bare metal
- KVM on x86_64: **yes** on a sufficiently modern dedicated-server CPU.
- Cost for a 1–3 h run: **not an hourly route**; the Eco/dedicated catalog uses a monthly commitment and may add setup fees, so a 1 h and 3 h run both cost the full first month plus any setup fee. Current location-specific checkout price was not stable enough to quote without guessing: https://eco.ovhcloud.com/ and https://www.ovhcloud.com/en/bare-metal/prices/ .
- Signup friction: OVH account, payment validation, and possible identity/fraud verification; availability varies.
- Guest console: common Incus tunnel; OVH KVM/IPMI only recovers L1.
- Spot/preemptible: none applicable.
- Disqualifiers / risks: monthly minimum and setup fees defeat a 1–3 h job.
- Confidence: **high that it is economically disqualified; price intentionally left unquoted rather than inferred**.

## Option: Hetzner dedicated/root server
- Route: bare metal
- KVM on x86_64: **yes** on modern AX/RX/EX hardware; Hetzner explicitly documents virtualization on root servers: https://docs.hetzner.com/robot/dedicated-server/virtualization/general/ .
- Cost for a 1–3 h run: **full monthly price plus the listed setup fee**; not hourly. Exact amount depends on model/location and changes in the live configurator: https://www.hetzner.com/dedicated-rootserver/ . Server Auction can remove setup fees but remains monthly.
- Signup friction: account/payment and potentially government-ID verification; this can delay first orders.
- Guest console: common Incus tunnel; request the provider KVM console only for L1 recovery.
- Spot/preemptible: none.
- Disqualifiers / risks: monthly minimum/setup fee make it irrational for a two-hour gate.
- Confidence: **high on technical capability and economic disqualification**.

## Option: phoenixNAP, Scaleway, and Vultr versus Servers.com
- Route: Servers.com bare metal
- KVM on x86_64: **yes** on modern dedicated CPUs.
- Cost for a 1–3 h run: **unconfirmed / sales-led monthly dedicated contract**, not a clearly self-service one-hour product: https://www.servers.com/pricing .
- Signup friction: sales/account onboarding rather than instant consumer checkout.
- Guest console: common Incus tunnel.
- Spot/preemptible: none documented.
- Disqualifiers / risks: absent transparent hourly/minimum pricing and slower sales provisioning; it cannot beat the three self-service bare-metal options above.
- Confidence: **medium**.

## Option: GCP Compute Engine nested virtualization
- Route: nested-virt VM
- KVM on x86_64: **yes** — GCP explicitly passes VT-x/AMD-V and supports Linux KVM. Current restrictions explicitly exclude **E2**, memory-optimized, Arm, and AMD except N4D: https://cloud.google.com/compute/docs/instances/nested-virtualization/overview . Enable with `--enable-nested-virtualization`; the current recommended method **does not require a custom image or special license key**: https://cloud.google.com/compute/docs/instances/nested-virtualization/enabling .
- Cost for a 1–3 h run: select a modern Intel C4/N2 shape with at least 2 vCPU and ~8 GB host RAM plus ≥50 GB persistent disk. **Exact current regional SKU total was not captured reproducibly from the dynamic primary price table, so no dollar figure is asserted here.** Use the official calculator immediately before launch: https://cloud.google.com/products/compute/pricing/general-purpose . Billing is per second with a one-minute minimum.
- Signup friction: new users provide a payment method; Google places a temporary $0–$1 authorization and may require bank verification. The free trial gives **$300 for 90 days**, and its listed Compute Engine restrictions do not exclude nested virtualization, so an eligible new account should have this run covered by trial credit. Source: https://cloud.google.com/free/docs/free-cloud-features .
- Provisioning / image: ordinary self-service VM, normally seconds/minutes subject to zone capacity; current Debian/Ubuntu public images exist.
- Guest console: common Incus tunnel; GCP serial console is L1 only.
- Spot/preemptible: available, but GCP says Spot can be reclaimed **at any time**, has no SLA, and is intended for fault-tolerant work: https://cloud.google.com/compute/docs/instances/spot . Do not use it for this operator-attended gate.
- Disqualifiers / risks: E2 is a hard no; choose a CPU platform that preserves `x86_64_v3` into L2 and verify it empirically. Nested I/O can be >10% slower per GCP.
- Confidence: **high technically, incomplete price capture**.

## Option: AWS EC2 `m7i-flex.large` — cheapest fully documented hyperscaler VM
- Route: nested-virt VM
- KVM on x86_64: **yes** — AWS added supported nested virtualization to non-metal instances in 2026. Current allowlist includes C/M/R 7i and 8i families/flex, X8i and I7i, and explicitly supports KVM. Enable `NestedVirtualization=enabled`: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/amazon-ec2-nested-virtualization.html and announcement https://aws.amazon.com/about-aws/whats-new/2026/02/amazon-ec2-nested-virtualization-on-virtual/ . `m7i-flex.large` is Sapphire Rapids, 2 vCPU/8 GiB; the cheaper `c7i-flex.large` has only 4 GiB and lacks host headroom for a 4 GiB guest.
- Cost for a 1–3 h run: us-east-1 Linux on-demand compute **$0.09576/h** + 22 GiB gp3 at $0.08/GB-month (**$0.002444/h**) + public IPv4 **$0.005/h** = **$0.1032 / $0.3096 USD**. EC2 Linux billing is per second with a 60-second minimum; EBS/IP prorate while allocated. Primary pricing references: https://aws.amazon.com/ec2/pricing/on-demand/ , https://aws.amazon.com/ebs/pricing/ , https://aws.amazon.com/vpc/pricing/ .
- Signup friction: AWS account, payment card, phone/identity checks, and default regional vCPU quota. This 2-vCPU VM fits normal new-account quota. Current new-account credit program can offset the run; do not assume old “750 free hours” rules.
- Provisioning / image: ordinary EC2 launch, generally under a minute subject to capacity; current Ubuntu 24.04/Debian AMIs are available.
- Guest console: common Incus tunnel; EC2 serial console is L1 only.
- Spot/preemptible: a live 2026-08-16 us-east-1 Linux Spot quote was about **$0.0375/h**, producing about **$0.0449 / $0.1348** with disk/IP. Spot interruption is not worth saving roughly six cents per hour.
- Disqualifiers / risks: only the documented family allowlist qualifies. Old non-metal C5/M5 instances are **not** on it. Nested performance is lower than metal.
- Confidence: **high** — current explicit AWS support, modern CPU, exact rate components, low quota burden.

## Option: AWS `c5n.metal` (metal fallback)
- Route: bare metal
- KVM on x86_64: **yes**, native VT-x.
- Cost for a 1–3 h run: cheapest current x86 metal found was `c5n.metal` at **$3.888/h**, plus EBS/public IPv4: about **$3.895 / $11.686 USD**. Pricing reference: https://aws.amazon.com/ec2/pricing/on-demand/ .
- Signup friction: new accounts commonly have a 5-vCPU standard quota, while `c5n.metal` exposes 72 vCPU, so a quota increase may block same-day use.
- Provisioning / image: Ubuntu/Debian AMIs available, but metal startup can take 20+ minutes and capacity is less predictable.
- Guest console: common Incus tunnel.
- Spot/preemptible: metal Spot exists but is capacity/price volatile and unsuitable for attended acceptance.
- Disqualifiers / risks: far more expensive and more quota/capacity friction than AWS’s now-supported `m7i-flex.large`.
- Confidence: **high but not recommended**.

## Option: Azure `D2s_v5`-class nested virtualization
- Route: nested-virt VM
- KVM on x86_64: **yes on documented nested-virtualization families**; select a current Dsv5/related supported Intel SKU rather than assuming every VM exposes VMX. Azure guidance: https://learn.microsoft.com/en-us/azure/virtual-machines/windows/nested-virtualization .
- Cost for a 1–3 h run: **exact current regional compute+managed-disk+public-IP total was not reproducibly captured from Microsoft’s dynamic price table and is intentionally unquoted**. Use the official calculator with a 2-vCPU/8-GiB D2s_v5-class VM and ≥50 GiB disk: https://azure.microsoft.com/en-us/pricing/calculator/ . VM billing is per second/minute-prorated depending resource; managed disk/IP continue until deleted.
- Signup friction: Microsoft account, phone/card verification; eligible new customers receive $200/30 days, subject to current offer terms: https://azure.microsoft.com/en-us/free/ .
- Provisioning / image: self-service minutes; Ubuntu 24.04 and Debian images are available.
- Guest console: common Incus tunnel; Azure serial console is L1 only.
- Spot/preemptible: Azure Spot has eviction/no SLA and variable regional price; unsuitable for this gate.
- Disqualifiers / risks: family/region support and L2 `x86_64_v3` must be checked; exact current price missing from this pass.
- Confidence: **high for the documented family, incomplete price capture**.

## Option: Oracle OCI `VM.Standard3.Flex` — lowest nominal VM cost, conditional runner-up
- Route: nested-virt VM
- KVM on x86_64: **yes on Intel OCI VM shapes** — Oracle’s KVM documentation says release 1.5 added nested virtualization for Intel-based OCI VMs and explicitly says AMD VM shapes do not support it. Current UEK 8 notices still discuss OCI nested-virtualization configuration, although the old prebuilt Oracle Linux 7 KVM Marketplace image is EOL: https://docs.oracle.com/en-us/iaas/oracle-linux/oci/index.htm and https://docs.oracle.com/en-us/iaas/oracle-linux/oci/general-notices.htm .
- Cost for a 1–3 h run: 1 OCPU (2 vCPU) **$0.04/h** + 8 GB × $0.0015 = $0.012/h + 50 GB balanced boot volume about $0.002856/h = **$0.0549 / $0.1646 USD**. Compute and storage are per second after a one-minute minimum. Official price list: https://www.oracle.com/cloud/price-list/ ; shape/billing reference: https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm .
- Signup friction: mobile number and card; Oracle documents a temporary **$1 authorization**, rejects virtual/single-use/prepaid cards, and account/tenancy validation can be troublesome. Source: https://www.oracle.com/cloud/free/faq/ .
- Provisioning / image: ordinary VM launch, inferred minutes but no SLA; capacity errors occur by region. Current Ubuntu 24.04 x86_64 images are documented: https://docs.oracle.com/en-us/iaas/images/ubuntu-2404/index.htm .
- Guest console: common Incus tunnel; OCI serial console is L1 only.
- Spot/preemptible: OCI preemptible capacity is 50% off and can be reclaimed at any time: https://docs.oracle.com/en-us/iaas/Content/Compute/Concepts/preemptible.htm . About $0.0289/$0.0866 including disk if uninterrupted; not worth the risk.
- Disqualifiers / risks: purpose-built KVM image is EOL; current docs do not promise the full CPUID seen by L2. Require host-passthrough and an L2 `x86_64_v3` preflight. Current UEK configuration delays nested-virt activation to preserve live migration.
- Confidence: **medium-high** — nominally cheapest, but less clean/current than phoenixNAP bare metal or AWS’s 2026 explicit feature. Treat as conditional, not the top no-surprises choice.

## Option: Contabo VDS S
- Route: nested-virt VM
- KVM on x86_64: **yes** — Contabo formally says nested virtualization is allowed only on VDS and dedicated servers, not ordinary VPS: https://help.contabo.com/en/support/solutions/articles/103000271595-can-i-setup-nested-virtualization-on-my-server- .
- Cost for a 1–3 h run: **€39 / €39 EUR** because VDS S is €39/month with a one-month minimum; no setup fee. A 24-month promotional effective rate is irrelevant: https://contabo.com/en/vds/ .
- Signup friction: mandatory customer-data verification can request passport/license/national ID plus utility bill, delaying provisioning: https://help.contabo.com/en/support/solutions/articles/103000348466-why-do-i-need-to-verify-my-purchase- .
- Provisioning / image: marketed as minutes after verification; 24 GB RAM, 180 GB NVMe, Ubuntu/Debian/Proxmox/custom images.
- Guest console: common Incus tunnel; Contabo also supplies outer-host VNC.
- Spot/preemptible: none.
- Disqualifiers / risks: monthly minimum and potentially slow KYC; L2 CPUID still needs `x86_64_v3` preflight.
- Confidence: **medium; technically allowed but economically poor**.

# Negative and unconfirmed cloud results

## Option: Infomaniak Cloud VPS / VPS Lite
- Route: nested-virt VM
- KVM on x86_64: **no** — formal FAQ says the infrastructure does not transmit virtualization instructions, so nested virtualization is impossible: https://www.infomaniak.com/en/support/faq/2811/use-nested-virtualization-cloud-vps-lite-vps .
- Cost for a 1–3 h run: not applicable; cannot satisfy `/dev/kvm`.
- Signup friction: irrelevant.
- Guest console: cannot reach the required L2 KVM guest.
- Disqualifiers / risks: hard no.
- Confidence: **high**.

## Option: Kamatera Cloud Servers
- Route: nested-virt VM
- KVM on x86_64: **no** — formal FAQ: “We currently do not support nested virtualization on our cloud servers”: https://www.kamatera.com/faq/answer/do-you-offer-nested-virtualization/ .
- Cost for a 1–3 h run: not applicable.
- Signup friction: irrelevant.
- Guest console: impossible for required L2 KVM.
- Disqualifiers / risks: hard no.
- Confidence: **high**.

## Option: Contabo ordinary VPS
- Route: nested-virt VM
- KVM on x86_64: **no** — the same formal Contabo policy explicitly excludes VPS and says the hardware features are absent: https://help.contabo.com/en/support/solutions/articles/103000271595-can-i-setup-nested-virtualization-on-my-server- .
- Cost for a 1–3 h run: not applicable; buy VDS only if accepting a monthly minimum.
- Signup friction: irrelevant.
- Guest console: impossible for required L2 KVM.
- Disqualifiers / risks: hard no.
- Confidence: **high**.

## Option: Hetzner Cloud
- Route: nested-virt VM
- KVM on x86_64: **unconfirmed; treat as no for planning**. No current formal Hetzner Cloud document promises `/dev/kvm`. A provider-hosted third-party tutorial says KVM cannot be used because nesting is not enabled: https://community.hetzner.com/tutorials/proxmox-on-cloud/ . The dedicated-server `vKVM` documentation is a different product.
- Cost for a 1–3 h run: irrelevant until Hetzner supplies a written current entitlement.
- Signup friction: not evaluated because technical prerequisite is unconfirmed.
- Guest console: provider console cannot fix absent VMX.
- Disqualifiers / risks: no primary supported-capability statement.
- Confidence: **medium that it is unsuitable; low as a formal-policy “no”**.

## Option: Akamai/Linode standard instance
- Route: nested-virt VM
- KVM on x86_64: **no supported route found**. A provider-hosted 2024 support answer says running KVM for nested virtualization is “not something we currently support”: https://www.linode.com/community/questions/25027/kvm-not-available . No formal product doc now promises `/dev/kvm`.
- Cost for a 1–3 h run: irrelevant.
- Signup friction: irrelevant.
- Guest console: provider Lish console is L1 only.
- Disqualifiers / risks: community/support answer rather than contract docs, but no positive entitlement.
- Confidence: **medium-high unsuitable; medium source authority**.

## Option: Vultr Cloud Compute (not Bare Metal)
- Route: nested-virt VM
- KVM on x86_64: **unconfirmed/unsupported**. No current formal Cloud Compute policy promises it; an older provider-hosted discussion says VPS nested virtualization is unsupported and recommends bare metal: https://discuss.vultr.com/discussion/1957/ask . Vultr saying its outer fleet uses KVM is not passthrough evidence.
- Cost for a 1–3 h run: irrelevant; use Vultr Bare Metal if choosing Vultr.
- Signup friction: irrelevant.
- Guest console: web console is L1 only.
- Disqualifiers / risks: no current supported `/dev/kvm` contract.
- Confidence: **medium**.

## Option: DigitalOcean Droplets
- Route: nested-virt VM
- KVM on x86_64: **ambiguous, not credible without written confirmation**. Current formal docs acknowledge “Droplets running workloads leveraging nested virtualization” and say nested VMs must restart after live migration, but do not guarantee `/dev/kvm` for every size/region: https://docs.digitalocean.com/products/droplets/details/live-migration/ . Older provider-hosted community answers conflict.
- Cost for a 1–3 h run: intentionally not ranked; a cheap Droplet is wasted money if VMX is absent/migrated.
- Signup friction: normal self-service, but technical entitlement is the blocker.
- Guest console: Droplet console is L1 only.
- Disqualifiers / risks: no stable `/dev/kvm` guarantee; live migration explicitly disrupts nested VMs.
- Confidence: **low** — obtain a current support answer and runtime preflight before considering it.

## Option: Oracle OCI AMD VM shapes
- Route: nested-virt VM
- KVM on x86_64: **no in Oracle’s KVM documentation** — AMD processor-based virtual machines do not support nested virtualization: https://docs.oracle.com/en-us/iaas/oracle-linux/oci/index.htm . Use Intel `VM.Standard3.Flex` instead.
- Cost for a 1–3 h run: not applicable.
- Signup friction: irrelevant.
- Guest console: cannot fix missing nested virtualization.
- Disqualifiers / risks: hard shape-family negative, though scoped to Oracle’s KVM documentation.
- Confidence: **medium-high**.

# Contradictions and remaining unknowns

- **AWS assumption changed:** “AWS needs `*.metal`” is obsolete as of February 2026. AWS now formally supports nested KVM on multiple non-metal 7i/8i families. `m7i-flex.large` is the practical AWS choice; metal is no longer cost-rational.
- **Oracle assumption is partly wrong:** Intel OCI VM shapes have official nested-virt documentation; AMD OCI VMs are the documented no. The old ready-made KVM image is EOL, so there is still more setup/risk than AWS/GCP.
- **DigitalOcean evidence conflicts:** the July 2026 migration doc acknowledges nested workloads, while old support answers are inconsistent. An acknowledgment is not an entitlement to `/dev/kvm` on a chosen SKU.
- **Equinix is gone:** any older comparison recommending Equinix Metal is stale after 2026-06-30.
- Current primary documentation did not provide stable exact prices for Hivelocity, GCP C4/N2, or Azure Dsv5 in a form captured here. They are explicitly marked unpriced rather than populated from third-party calculators.

# Ranked recommendation

1. **phoenixNAP `s0.d1.small`** — cheapest **high-confidence** option: native KVM, known `x86_64_v3`-era CPU, enough RAM/disk, 3–15 minute deployment, and only **$0.087–$0.261 all-in** including the needed public IP.
2. **Scaleway `EM-A610R-NVMe`** — equally clean native-KVM route at **€0.11–€0.33**; excellent fallback if phoenixNAP stock/account onboarding fails.
3. **AWS `m7i-flex.large`** — best documented nested-VM route at **$0.103–$0.310** and likely the lowest-friction choice if the developer already has AWS credentials.
4. **Oracle `VM.Standard3.Flex`** — lower nominal price (**$0.055–$0.165**) but conditional on current-image setup and an L2 `x86_64_v3` preflight; it is not the no-surprises winner.
5. **Vultr E-2286G bare metal** — reliable but costs more (**$0.275–$0.825**).

**What I would do:** create/verify a phoenixNAP account before the run window, deploy `s0.d1.small` with Ubuntu Jammy and a public `/31`, immediately prove `test -c /dev/kvm`, inspect CPU flags, install Incus plus `swtpm`/SPICE prerequisites, and boot a tiny L2 preflight that confirms `x86_64_v3` before copying the real artifacts. Keep the Incus API loopback-only and use the SSH-forwarded local `remote-viewer` path. If the SKU is out of stock or onboarding is delayed, switch directly to Scaleway `EM-A610R-NVMe`; if an existing AWS account is already warm, `m7i-flex.large` may win on total operator time despite nested virtualization.

## EphemeralCI (CI and dev environments)

# Ephemeral CI and dev environments for Track C

**Research date:** 2026-08-16. **Currency:** USD unless noted. Prices below are current published prices on that date.

## Bottom line

The earlier conclusion that a GitHub-hosted `ubuntu-latest` job necessarily had to use TCG is now stale. GitHub officially enabled `/dev/kvm` on its 2-vCPU hosted Linux fleet on 2024-04-02 and published the exact udev rule needed to make the device usable. Current GitHub Docs say nested virtualization is technically possible, but generic nested VMs are explicitly experimental and unsupported. Standard runners still have only 14 GB documented SSD, below Track C's ~22 GB floor; they are therefore a valuable **free KVM reconnaissance probe**, not the cleanest acceptance host. [GitHub KVM announcement](https://github.blog/changelog/2024-04-02-github-actions-hardware-accelerated-android-virtualization-now-available/); [current unsupported-nested-VM warning](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners#runner-images); [current runner sizes](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#supported-runners-and-hardware-resources).

The best near-free, human-attended route I found is **Semaphore Cloud on `f1-standard-4`**: Semaphore documents nested virtualization, 4 vCPU/16 GB/65 GB, passwordless sudo, a $15 monthly free credit, an interactive SSH session that can be extended to three hours, and built-in port forwarding. This combination addresses both the KVM and console problems without inventing a tunnel service. [Nested virtualization](https://docs.semaphore.io/reference/os-ubuntu#nested-virtualization); [machine specifications](https://docs.semaphore.io/reference/machine-types#f1); [billing](https://docs.semaphore.io/using-semaphore/billing#rates); [interactive SSH and port forwarding](https://docs.semaphore.io/using-semaphore/jobs#ssh-into-agent).

The repository's gate requires real observation, not inferred side effects: it requires `/dev/kvm`, a SPICE client, observed installer completion, and live evidence during recovery; the previous TCG run did not observe seed consumption. `docs/docs/how-to/verify-boot-acceptance.md:8-25,143-180,218-245`; `docs/notes/phase-5-boot-probe.md:1-60`.

Incus's TPM device is a **software TPM 2.0**, so none of these hosts needs a physical TPM chip. The runner only needs the `swtpm` package and Incus/QEMU support. [Incus TPM device reference](https://linuxcontainers.org/incus/docs/main/reference/devices_tpm/).

## Option: Semaphore Cloud `f1-standard-4` — best overall
- Route: CI runner with documented nested virtualization and native interactive debugging.
- KVM on x86_64: **Yes** — Semaphore Cloud's Ubuntu documentation explicitly supports nested virtualization and walks through creating a KVM guest with `uvt-kvm`; `f1-standard-4` is Intel x86_64, 4 vCPU, 16 GB RAM, 65 GB NVMe. [Nested virtualization](https://docs.semaphore.io/reference/os-ubuntu#nested-virtualization); [F1 specifications](https://docs.semaphore.io/reference/machine-types#f1).
- Cost for a 1–3 h run: **$0 while within the recurring $15 monthly credit**. Otherwise 4-vCPU F1 is $0.015/min, or **$0.90–$2.70**, billed by running minute with sub-minute jobs rounded to one minute. The $15 quota covers about 16.7 hours at this size. [Semaphore pricing/free credit](https://semaphore.io/pricing); [billing calculation and rates](https://docs.semaphore.io/using-semaphore/billing#how-machine-usage-is-billed).
- Signup friction: Free signup, no card required for the included credit; warm agents are allocated when the job starts. Install the `sem` CLI locally for interactive access. [Semaphore pricing](https://semaphore.io/pricing); [job lifecycle](https://docs.semaphore.io/using-semaphore/jobs#job-lifecycle).
- Guest console: Create or re-run the setup job as `sem debug job <job-id> --duration 3h`. Semaphore gives a full SSH shell and `sem port-forward <job-id> <local-port> <remote-port>`. The most Mac-friendly concrete display path is: on the runner launch Incus's SPICE `remote-viewer` in Xvfb, export that display through `x11vnc` + noVNC on loopback, then forward the noVNC port with `sem port-forward` and watch/interact in the Mac browser. This is a live rendering of `incus console "$VM" --type=vga`, not a post-run recording. [Three-hour debug session and attach](https://docs.semaphore.io/using-semaphore/jobs#ssh-into-agent); [port forwarding](https://docs.semaphore.io/using-semaphore/jobs#port-forwarding); [Incus VGA/SPICE console](https://linuxcontainers.org/incus/docs/main/howto/instances_console/); [noVNC requirements](https://github.com/novnc/noVNC#server-requirements).
- Disqualifiers / risks: Incus-in-Incus on this exact image is not provider-tested; the official example uses libvirt, so first run a strict preflight (`uname -m`, `/dev/kvm`, CPU flags, free disk, a throwaway KVM guest, then Incus initialization). Default jobs have a one-hour limit, so set the pipeline max duration or use the documented 3-hour debug session. Interactive sessions can be restricted by project/secret policy. [Job duration](https://docs.semaphore.io/using-semaphore/jobs#job-duration); [debug restrictions](https://docs.semaphore.io/using-semaphore/jobs#restrict-ssh-access).
- Confidence: **High for x86_64 KVM, disk, price, and interactive transport; medium-high end to end** until Incus+swtpm+OVMF is smoke-tested once.

## Option: Blacksmith x64 Linux runner — cheapest drop-in GitHub runner
- Route: Managed GitHub Actions runner.
- KVM on x86_64: **Yes** — Blacksmith states that all x64 Linux runners support KVM/nested virtualization. The smallest is 2 vCPU/8 GB/80 GB; 4 vCPU/16 GB/80 GB is also available. [Blacksmith runner overview and FAQ](https://docs.blacksmith.sh/blacksmith-runners/overview).
- Cost for a 1–3 h run: **$0 under the recurring 3,000 x64 2-vCPU minutes/month**. After that, the 2-vCPU x64 runner is $0.004/min, or **$0.24–$0.72**. Higher sizes consume free minutes proportionally. [Blacksmith pricing](https://www.blacksmith.sh/pricing); [free-tier accounting](https://docs.blacksmith.sh/blacksmith-runners/overview#faq).
- Signup friction: GitHub App installation; advertised as no credit card required and runners provision in under three seconds. [Blacksmith pricing](https://www.blacksmith.sh/pricing); [runner provisioning](https://docs.blacksmith.sh/blacksmith-runners/overview).
- Guest console: No native graphical console is documented. Add an official Tailscale ephemeral node to the workflow, start hardened SSH or expose only Incus TCP/8443 on the tailnet, then use either native Incus remote/SPICE or the Xvfb→x11vnc→noVNC browser bridge. Tailscale's Personal plan is $0 and includes 1,000 ephemeral-resource minutes/month. [Tailscale GitHub Action](https://tailscale.com/docs/integrations/github/github-action); [Tailscale pricing](https://tailscale.com/pricing); [Incus remote exposure](https://linuxcontainers.org/incus/docs/main/howto/server_expose/).
- Disqualifiers / risks: KVM is documented, but Incus, software TPM, UEFI, and a three-hour interactive tunnel are not an advertised Blacksmith scenario. Use `workflow_dispatch` from a trusted branch and do not expose secrets to fork PRs.
- Confidence: **High for KVM/resources/price; medium for the attended console integration**.

## Option: BuildJet AMD runner
- Route: Managed GitHub Actions runner.
- KVM on x86_64: **Yes** — BuildJet says all AMD runners support nested virtualization. Runners have 64 GB SSD; 2 vCPU/8 GB and 4 vCPU/16 GB shapes are available. [BuildJet hardware](https://buildjet.com/for-github-actions/docs/runners/hardware).
- Cost for a 1–3 h run: The one-time **$5 trial credit makes one run free**. Otherwise 2 vCPU is $0.004/min (**$0.24–$0.72**) and 4 vCPU is $0.008/min (**$0.48–$1.44**). [BuildJet pricing](https://buildjet.com/for-github-actions/docs/about/pricing).
- Signup friction: GitHub integration and one-time credit; no recurring public-repository free tier is documented.
- Guest console: Same Tailscale + Incus remote/SPICE or Tailscale + noVNC bridge as Blacksmith.
- Disqualifiers / risks: No native interactive terminal/graphical console was found; the tunnel and Incus setup are operator-owned. The 2-vCPU/8-GB size is economical but tight; 4/16 is safer.
- Confidence: **High for KVM/disk/price; medium for end-to-end console**.

## Option: Namespace nested-virtualization runner
- Route: Managed GitHub runner or ephemeral compute instance.
- KVM on x86_64: **Yes when `nested_virtualization: true`** — Namespace documents that this exposes `/dev/kvm` and automatically mounts it into containers with non-root permissions. [Namespace nested virtualization](https://namespace.so/docs/architecture/compute/nestedvirt); [feature announcement](https://namespace.so/blog/changelog-018).
- Cost for a 1–3 h run: 30-day free trial is advertised, but the public page does not state its credit amount. On Developer pay-as-you-go, 4 vCPU/8 GB is $0.006/min overage, or **$0.36–$1.08**; 4/16 is $0.012/min, or **$0.72–$2.16**. Minimum billing is one minute with 15-second downward rounding thereafter. A 50-GB attached cache volume, if needed to guarantee working space, adds $0.002/GB-hour, or **$0.10–$0.30**. [Namespace pricing and billing granularity](https://namespace.so/pricing); [cache volumes](https://namespace.so/docs/architecture/storage/cache-volumes).
- Signup friction: Account/GitHub integration; 30-day trial; root/privileged access is included. Developer instances have a three-hour maximum, exactly the upper Track C window. [Namespace pricing](https://namespace.so/pricing).
- Guest console: No first-party GUI was found. Use Tailscale plus a narrowly ACLed Incus/noVNC service.
- Disqualifiers / risks: Verify the ephemeral disk capacity or attach a sufficiently large cache volume before beginning. A three-hour hard maximum leaves no margin if setup is included, so pre-stage the image and packages or use a higher plan with a five-hour limit.
- Confidence: **High for `/dev/kvm`; medium-high overall after disk/time preflight**.

## Option: Cirrus CI Linux task
- Route: CI container/VM task.
- KVM on x86_64: **Yes for Linux containers configured with `kvm: true`**. Cirrus explicitly documents the option for hardware-accelerated virtualization. [Cirrus Linux containers](https://cirrus-ci.org/guide/linux/).
- Cost for a 1–3 h run: Public OSS projects receive up to 50 compute credits/month. Linux costs 3 credits per 1,000 vCPU-minutes; a 4-vCPU run is **$0.72–$2.16 equivalent**, but is **$0 within the OSS grant**. Cirrus also describes a $10/month private-personal plan. [Cirrus pricing](https://cirrus-ci.org/pricing/); [OSS free cap](https://cirrus-ci.org/features/); [free-usage FAQ](https://cirrus-ci.org/faq/).
- Signup friction: GitHub App/account; public OSS quota is automatic subject to fair-use limits. Cirrus Terminal offers SSH-like task access. [Cirrus Terminal](https://cirrus-ci.org/blog/2021/08/06/introducing-cirrus-terminal-a-simple-way-to-get-ssh-like-access-to-your-tasks/).
- Guest console: Cirrus Terminal is shell access, not a documented graphical stream. Use Tailscale/noVNC for the pixels.
- Disqualifiers / risks: The published KVM path is a container. Running the Incus daemon, managed bridge, loop devices, and nested mounts inside that container may require privileged/nesting settings not guaranteed by the `kvm: true` statement. Published default disk capacity was not established. This makes it a good free probe but a weaker acceptance host than Semaphore/Blacksmith.
- Confidence: **High for KVM device availability; medium-low for Incus and disk**.

## Option: Depot CI (not Depot GitHub Actions runners)
- Route: Depot's native CI sandbox.
- KVM on x86_64: **Yes only in Depot CI sandboxes** — Depot states `/dev/kvm` is available in every x86_64 Depot CI sandbox. Depot's separate GitHub Actions runner product does not document nested KVM; do not conflate them. [Depot CI overview](https://depot.dev/docs/ci/overview); [Depot runner troubleshooting](https://depot.dev/docs/github-actions/troubleshooting).
- Cost for a 1–3 h run: **$0 during the seven-day no-card trial**. Thereafter the minimum is the $20/month Developer plan, including 2,000 2-CPU sandbox minutes. Overage for 2 CPU/8 GB is $0.0001/sec = $0.006/min (**$0.36–$1.08**); 4/16 doubles that. Billing is per second with no one-minute minimum. [Depot CI pricing and sandbox table](https://depot.dev/docs/ci/overview#pricing).
- Signup friction: Seven-day trial, GitHub App, `depot ci migrate`; jobs start in 2–3 seconds. [Depot CI overview](https://depot.dev/docs/ci/overview).
- Guest console: Depot has built-in SSH debugging (`--ssh-after-step`). Add a Tailscale/noVNC stream for the graphical console. [Depot CI debugging](https://depot.dev/docs/ci/how-to-guides/debug-with-ssh).
- Disqualifiers / risks: The public sandbox-size table gives CPU/RAM but not disk size, so the 22-GB floor is unproven. After the trial this is a $20 minimum-term purchase rather than true one-off hourly compute.
- Confidence: **High for KVM and interactive shell; medium-low until disk is confirmed**.

## Option: GitHub-hosted standard Ubuntu runner — free reconnaissance, not supported acceptance
- Route: Standard GitHub-hosted Actions runner.
- KVM on x86_64: **Yes today**. GitHub's 2024 announcement explicitly targets the `kvm` device and says 2-vCPU hosted Linux runners now have hardware acceleration; current public Ubuntu runners are 4 vCPU/16 GB and private runners are 2/8. Public and private visibility no longer changes KVM availability. [2024 announcement and udev recipe](https://github.blog/changelog/2024-04-02-github-actions-hardware-accelerated-android-virtualization-now-available/); [public/private specs](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#supported-runners-and-hardware-resources).
- Cost for a 1–3 h run: **$0 and unlimited for public repositories**. Private repositories use included minutes, then Linux x64 is $0.006/min (**$0.36–$1.08**), rounded up per job to full minutes. [GitHub runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing).
- Signup friction: None beyond the existing repository; jobs are warm-provisioned in seconds. Use pinned `ubuntu-24.04`, not moving `ubuntu-latest`.
- Guest console: No GitHub-native SSH/GUI. Use the official Tailscale action, expose only the Incus API/noVNC endpoint on a narrow tailnet ACL, and keep the job alive with `timeout-minutes` under the six-hour hosted limit. [Actions limits](https://docs.github.com/en/actions/reference/limits); [Tailscale action](https://tailscale.com/docs/integrations/github/github-action).
- Disqualifiers / risks: **14 GB documented SSD is below Track C's ~22 GB floor.** Deleting preinstalled SDK/tool caches may reclaim space, but that is not a documented capacity guarantee. Generic nested VMs are explicitly experimental/unsupported; GitHub only commits to Android hardware acceleration. `ubuntu-slim` is an unprivileged 15-minute container and is categorically unsuitable. [Sizes and `ubuntu-slim`](https://docs.github.com/en/actions/reference/runners/github-hosted-runners); [unsupported warning](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners#runner-images).
- Confidence: **High that `/dev/kvm` exists; low that standard runners are a credible final acceptance host**.

## Option: GitHub larger 2-core Ubuntu runner
- Route: GitHub-hosted larger runner.
- KVM on x86_64: **Yes**, with the same experimental generic-nesting caveat; the 2-core size has 8 GB RAM and 75 GB SSD. [Larger-runner sizes](https://docs.github.com/en/actions/reference/runners/larger-runners#machine-sizes-for-larger-runners); [larger-runner KVM announcement](https://github.blog/changelog/2023-02-23-hardware-accelerated-android-virtualization-on-actions-windows-and-linux-larger-hosted-runners/).
- Cost for a 1–3 h run: $0.006/min = **$0.36–$1.08**, no included/public-repository minutes. For a new one-off setup, GitHub Team is $4/user/month, so practical first-month total is **$4.36–$5.08**. Idle pools are not billed. [Runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing); [GitHub plans](https://github.com/pricing).
- Signup friction: Organization owner, Team/Enterprise plan, card, nonzero Actions budget, runner-group configuration; card enablement may take ten minutes. [Larger-runner troubleshooting](https://docs.github.com/en/actions/reference/runners/larger-runners#troubleshooting-larger-runners).
- Guest console: Tailscale/Incus/noVNC as above; no native console.
- Disqualifiers / risks: More account work and higher real minimum than Semaphore/Blacksmith/BuildJet; still officially unsupported for generic nested VMs.
- Confidence: **High KVM/disk; medium-low value and support fit**.

## Option: Travis CI hosted Linux
- Route: Hosted CI VM.
- KVM on x86_64: **Likely yes, but documented through an Android example rather than a general guarantee** — Travis's official Android guide uses an x86_64 emulator and `sudo chmod -R 777 /dev/kvm`. [Travis Android guide](https://docs.travis-ci.com/user/languages/android/#creating-and-starting-an-emulator).
- Cost for a 1–3 h run: New accounts get a **14-day trial with 10,000 credits/1,000 Linux minutes**, enough for one run at $0. The lowest paid usage plan is **$15/month** for 35,000 Linux credits. OSS allotments are case-by-case, not automatic unlimited free use. [Billing overview](https://docs.travis-ci.com/user/billing-overview/); [pricing](https://www.travis-ci.com/pricing/); [OSS allocation policy](https://www.travis-ci.com/blog/2020-11-02-travis-ci-new-billing/).
- Signup friction: Account/trial; OSS continuation requires a support request.
- Guest console: Travis debug mode provides a live tmate shell but keeps the debug build alive only 30 minutes, so it cannot cover Track C. A normal 1–3 h job would still need Tailscale/noVNC. [Travis debug mode](https://docs.travis-ci.com/user/running-build-in-debug-mode/).
- Disqualifiers / risks: No general Incus/nested-VM support statement or qualifying disk guarantee was found; native interactive window is too short.
- Confidence: **Medium-low**.

## Option: Buildkite hosted Linux
- Route: Hosted CI agent.
- KVM on x86_64: **Undocumented / cannot qualify**. Current hosted-agent docs describe isolated virtualized environments but make no `/dev/kvm` or nested-virtualization promise. [Buildkite hosted agents](https://buildkite.com/docs/agent/buildkite-hosted); [Linux hosted agents](https://buildkite.com/docs/agent/buildkite-hosted/linux).
- Cost for a 1–3 h run: 30-day trial is free. Small x86 Linux is 2 vCPU/4 GB/47 GB at $0.008/min, or **$0.48–$1.44** afterward, billed to the second with no minimum. [Buildkite Linux hosted pricing](https://buildkite.com/platform/pipelines/hosting-options/linux-hosted-agents/); [billing model](https://buildkite.com/docs/agent/buildkite-hosted).
- Signup friction: 30-day trial; hosted queue setup.
- Guest console: Buildkite provides native terminal access, an unusually good starting point for noVNC/port-forward debugging. [Hosted-agent terminal access](https://buildkite.com/docs/agent/buildkite-hosted/terminal-access).
- Disqualifiers / risks: Missing KVM contract is decisive; 4 GB host RAM is also exactly the guest floor with no headroom.
- Confidence: **High that it must be rejected absent provider confirmation**.

## Option: GitHub Codespaces
- Route: Managed cloud dev environment.
- KVM on x86_64: **No supported route**. GitHub grants full root inside the dev container but only limited access to the outer VM; current containerlab documentation explicitly says Codespaces VMs do not support nested virtualization. [GitHub Codespaces architecture](https://docs.github.com/en/codespaces/about-codespaces/what-are-codespaces); [deep dive](https://docs.github.com/en/codespaces/about-codespaces/deep-dive); [containerlab Codespaces host requirements](https://containerlab.dev/manual/codespaces/#host-requirements).
- Cost for a 1–3 h run: Personal Free includes 120 core-hours and 15 GB-month, so one run is **$0** within quota. Overage is $0.18/hour for 2 cores or $0.36/hour for 4 cores; storage is $0.07/GB-month. [Codespaces billing](https://docs.github.com/en/billing/concepts/product-billing/github-codespaces).
- Signup friction: Existing GitHub account; browser/VS Code/CLI startup; no payment method needed until quota is exhausted.
- Guest console: Browser port forwarding would make noVNC easy, but the required KVM guest cannot be created. [Codespaces port forwarding](https://docs.github.com/en/codespaces/developing-in-a-codespace/forwarding-ports-in-your-codespace).
- Disqualifiers / risks: Container root is not outer-host root. Older anecdotes of accidental `/dev/kvm` availability conflict with current product/project documentation and are not a support contract.
- Confidence: **High reject**.

## Option: Gitpod / Ona Cloud
- Route: Managed cloud development environment (current successor to Gitpod Classic).
- KVM on x86_64: **Undocumented**; the current Ona docs corpus publishes managed VM environments but no `/dev/kvm`, VMX/SVM, or nested-virtualization support. Gitpod Classic PAYG was sunset in 2025. [Gitpod Classic sunset](https://ona.com/stories/gitpod-classic-payg-sunset); [Ona Cloud runner](https://ona.com/docs/ona/runners/ona-cloud).
- Cost for a 1–3 h run: No current free tier. Core starts at **$20/month** and includes 80 OCUs; a 4-vCPU/16-GB Standard environment consumes one OCU/hour, an effective $0.25/hour inside the minimum subscription. [Ona pricing](https://ona.com/pricing); [usage rates](https://ona.com/docs/ona/billing/usage).
- Signup friction: Core subscription/payment details; managed runner then provisions automatically.
- Guest console: Authenticated browser ports could carry noVNC, but KVM is missing.
- Disqualifiers / risks: $20 minimum and no nested-virt contract.
- Confidence: **High reject for this procurement**.

## Option: Coder
- Route: Self-hosted workspace orchestrator, not a compute provider.
- KVM on x86_64: **Only if the separately selected underlying host exposes it**. Coder explicitly says it is not a SaaS/fully managed offering; templates provision the user's EC2 VMs, Kubernetes pods, Docker containers, or other infrastructure. [Coder about/architecture](https://coder.com/docs/about).
- Cost for a 1–3 h run: Community software is **$0**, but underlying compute/storage/network cost is additional and unknown until a host is chosen. [Coder pricing](https://coder.com/pricing).
- Signup friction: Deploy Coder server/control plane, credentials, and Terraform template before provisioning a workspace.
- Guest console: Coder documents VNC/KasmVNC and port forwarding, but only after a qualifying host exists. [Coder remote desktops](https://coder.com/docs/user-guides/workspace-access/remote-desktops#vnc).
- Disqualifiers / risks: It merely moves the host-selection problem and adds setup; it supplies no free compute or KVM guarantee.
- Confidence: **High reject as an independent route**.

## Option: Daytona managed sandbox
- Route: Managed sandbox/dev environment.
- KVM on x86_64: **Undocumented** for both container and Linux VM sandboxes. A VM isolation boundary around the sandbox is not evidence that nested KVM is exposed inside it. [Daytona sandboxes](https://www.daytona.io/docs/en/sandboxes/); [runtime isolation](https://www.daytona.io/docs/en/isolation/#runtime-isolation).
- Cost for a 1–3 h run: Promotional $200 compute credit makes it **$0** while credit remains. Published maximum normal Linux 4-vCPU/8-GB cost is about **$0.3312/hour** from the CPU+RAM rates, or $0.33–$0.99, but managed root disk is capped at 10 GB. Daytona does not publish the $200 credit reset/expiry cadence. [Daytona pricing](https://www.daytona.io/pricing); [resource limits](https://www.daytona.io/docs/en/sandboxes/#resources).
- Signup friction: No-card trial, API key, millisecond sandbox creation.
- Guest console: Daytona has first-class browser VNC with Xfce/x11vnc/noVNC—excellent transport if KVM existed. [Daytona VNC](https://www.daytona.io/docs/en/vnc-access/).
- Disqualifiers / risks: No KVM contract and 10-GB root disk. BYOC requires the customer to supply the qualifying compute, so it moves rather than solves the problem. [Daytona BYOC](https://www.daytona.io/docs/en/bring-your-own-compute/).
- Confidence: **High reject**.

## Console judgement

A CI route **can** satisfy Track C, but only when a human watches a **live** SPICE/VNC/noVNC stream during both installer and recovery boots. The clean mechanisms are:

1. **Semaphore:** native 3-hour SSH debug session + native port forwarding + Xvfb/`remote-viewer`/x11vnc/noVNC.
2. **Other qualifying runners:** official Tailscale action creates an ephemeral ACL-controlled node; expose only Incus HTTPS on that tailnet and use a local Incus remote/SPICE client, or publish a loopback noVNC bridge over the tailnet. [Tailscale GitHub Action](https://tailscale.com/docs/integrations/github/github-action); [Incus server exposure](https://linuxcontainers.org/incus/docs/main/howto/server_expose/); [Incus VGA console](https://linuxcontainers.org/incus/docs/main/howto/instances_console/).
3. **Fallback:** key-only OpenSSH through ngrok TCP, then SSH-forward loopback Incus/noVNC. ngrok explicitly supports SSH/VNC TCP endpoints, but free TCP requires card verification and has a 1-GB outbound cap. [ngrok TCP endpoints](https://ngrok.com/docs/endpoints/tcp); [ngrok pricing](https://ngrok.com/pricing).

`tmate` alone is insufficient: it provides SSH/web shell, and its maintainer says port forwarding is unsupported. It can administer the job, but a second network/display mechanism must carry the console. [action-tmate](https://github.com/mxschmitt/action-tmate); [tmate port-forwarding issue](https://github.com/tmate-io/tmate/issues/37).

A framebuffer recording, screenshots, serial logs, packet counts, or disk-growth measurements are **secondary artifacts only**. They do not meet the repository's instruction to observe the console, and the prior probe proves why inference is unsafe. `docs/docs/how-to/verify-boot-acceptance.md:8-15,158-180,221-245`; `docs/notes/phase-5-boot-probe.md:24-60`.

GitHub's terms allow Actions for development/testing of the repository's software and limit VM access to one licensed user at a time; an attended boot-acceptance test appears aligned with that purpose. Leaving tunnel jobs idle as general remote desktops, sharing access, cryptomining, or unrelated workloads would create terms risk. [GitHub Additional Product Terms](https://docs.github.com/en/site-policy/github-terms/github-terms-for-additional-products-and-features#actions).

## Contradictions and remaining unknowns

- **Prior TCG result vs current KVM:** not a contradiction in evidence. The previous job deliberately used QEMU TCG; it did not test today's `/dev/kvm` path. Its negative result remains valid for TCG, while the official 2024 GitHub change justifies a new KVM probe. `docs/notes/phase-5-boot-probe.md:24-60`; [GitHub KVM announcement](https://github.blog/changelog/2024-04-02-github-actions-hardware-accelerated-android-virtualization-now-available/).
- **GitHub “hardware acceleration supported” vs “nested VMs unsupported”:** `/dev/kvm` exposure is established fact; compatibility of arbitrary Incus/QEMU guests is explicitly best-effort. Both statements are current GitHub policy. [Hosted-runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners); [unsupported warning](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners#runner-images).
- **Depot product split:** Depot CI explicitly has KVM; Depot GitHub Actions runners do not make that promise.
- **Codespaces anecdotes:** older reports of KVM in some default images conflict with current Codespaces architecture/current containerlab guidance. Treat accidental device exposure as unsupported, not procurement evidence.
- No provider except Semaphore documents the exact combination of KVM plus long interactive shell plus port forwarding. Incus+swtpm+OVMF must still be smoke-tested once on every candidate.
- For every candidate, preflight the actual CPU feature set required by the built IncusOS binary, not just `uname -m`; provider labels promise x86_64, not a stable microarchitecture baseline.

## Ranked recommendation

1. **Semaphore Cloud `f1-standard-4`** — $0 under recurring credit, officially documented nested virtualization, adequate disk/RAM, and the only first-party three-hour SSH + port-forward path.
2. **Blacksmith 2-vCPU x64** — $0 recurring quota, official KVM, 80 GB, easiest GitHub workflow migration; add Tailscale/noVNC.
3. **BuildJet 4-vCPU x64** — first run free under $5 credit, official nested virtualization, 64 GB; add Tailscale/noVNC.
4. **Namespace 4-vCPU** — official `/dev/kvm`, good pay-as-you-go price; watch the three-hour maximum and confirm/attach disk.
5. **GitHub larger 2-core** — technically sound KVM/disk, but Team-plan minimum and unsupported generic nesting make it worse value.
6. **GitHub standard public runner** — run one free preflight because it may work after reclaiming disk, but do not plan the acceptance gate around an undocumented disk workaround.
7. **Cirrus / Depot CI trial / Travis** — plausible probes with material Incus, disk, minimum-term, or console caveats.
8. **Buildkite, Codespaces, Ona, Coder, Daytona** — reject absent an explicit KVM-capable underlying host.

## What I would do

I would create a free Semaphore project with one manually triggered `f1-standard-4` job, make its first commands fail fast on `uname -m`, CPU feature baseline, `/dev/kvm`, 25+ GB free space, passwordless sudo, and a tiny KVM guest, then open it with `sem debug job <id> --duration 3h`. Inside that attended session I would install Incus/OVMF/swtpm and noVNC tooling, initialize `default` and `incusbr0`, launch `remote-viewer` under Xvfb, forward the noVNC port with `sem port-forward`, and run BOOT-02 through BOOT-10 while watching from the Mac browser. I would upload the evidence archive before exiting because the agent is destroyed. If the Incus smoke test fails, I would stop immediately rather than burn the free quota and rerun the same workflow on Blacksmith, where KVM and 80-GB disk are explicit.