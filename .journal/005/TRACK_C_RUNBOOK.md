---
title: Track C runbook — running boot acceptance on Semaphore Cloud
venue: Semaphore Cloud, project incusos-builder-eins, f1-standard-4 / ubuntu2404
date: 2026-08-16
status: ready to execute; not yet executed
---

# Track C on Semaphore — verified runbook

## Verdict: viable, free, and no graphics needed

Both preflight blocks passed on `f1-standard-4`. The venue question is settled:
Semaphore can host the attended gate, at $0 against the recurring monthly credit
(a full three-hour session costs about $2.70 of a $15 credit).

The surprise that makes this easy: **the installer's TUI is mirrored to the
serial console, but only inside an Incus VM.**

```go
// If we're running in an Incus VM, additionally use /dev/ttyS0.
_, err := os.Stat("/dev/virtio-ports/org.linuxcontainers.incus")
if err == nil {
    ttyDevs = append(ttyDevs, "/dev/ttyS0")
}
```
`reference/incus-os/incus-osd/internal/tui/tui.go:67-71` at the pinned commit.

So the operator watches the install in the ordinary SSH terminal with
`incus start "$VM" --console`. **No SPICE client, no noVNC, no port forwarding,
no X server.** That removes the single most awkward requirement in the gate.

Two consequences worth recording:

- The earlier session note claiming "the shipped UKIs configure no console" is
  **refuted for the Incus venue**. It was true of the plain QEMU/OVMF spike,
  which has no `org.linuxcontainers.incus` virtio port — which is very likely
  why the Phase 5.2 probe saw nothing on its console and could not tell a failed
  install from a silent one.
- It is an argument for running the gate under **Incus specifically**, not a
  plain QEMU equivalent. The documented checklist and the observable console
  agree only in the Incus case.

## Preflight evidence (both blocks passed)

Host, block 1 — pipeline `9c8bb158`, job `6c20e0e3`:

| Property | Observed |
|---|---|
| Arch / CPU | x86_64, 13th Gen Intel i5-13500, 4 vCPU, 16 GB |
| Virtualization | `vmx`; `kvm_intel` + `kvm` loaded |
| `/dev/kvm` | readable and writable by the unprivileged `semaphore` user |
| Disk | 52 GB free |
| Tooling | QEMU 8.2.2, swtpm 0.7.3, OVMF `OVMF_CODE_4M.fd`, Incus 7.3 |
| Real guest | booted under `-enable-kvm`: `Hypervisor detected: KVM` |

Topology, block 2 — pipeline `5e4dfbe6`, job `c82f95e5`:

| Property | Observed |
|---|---|
| Pool | `default`, driver `dir` (sparse) |
| Network | `incusbr0` bridge, `10.193.173.1/24` |
| VM | UEFI, `security.secureboot=false`, vTPM added before first start |
| Guest TPM | `/dev/tpm0` and `/dev/tpmrm0` present; `tpm_version_major` = **2** |
| Guest UEFI | `/sys/firmware/efi` present |
| Pool usage | 913 MB actual for an 8 GiB volume — sparse confirmed |
| Headroom | 50 GB free after the VM existed |

The sparse pool matters: a thick-provisioned 50 GiB target would not fit in
52 GB alongside a 3.4 GB installer. Use the `dir` driver, as the preflight did.

## Correction to apply before following any older runbook

Research drafted a runbook whose seed-consumption step reproduced the **old,
broken** oracle — hashing the seed partition on the **source** block device
before and after and asserting the digest changed. That is the defect fixed in
PR #21 (finding `F-DOC-11`): at pinned upstream the installer copies partition 2
to the **target** and deletes `install.*` there
(`install.go:893-894` → `seed.go:31-36`), opening the source `O_RDONLY`. The
source is expected to be byte-identical afterwards, so that assertion can never
pass.

**Use the corrected target-side oracle below.** If you are reading a copy of
`verify-boot-acceptance.md` from `master`, it is stale until PR #21 merges.

## Runbook

### Mac side

1. Build the artifacts locally first — the paid/attended clock should not cover a
   build. The gate config is `image.type: raw`, `architecture: x86_64`,
   `offline: true`, one application, `install.target.min_size: 50GiB`.
   Record `result.sha256` and `result.resources_sha256`.
2. Compress and stage for transfer (~3.76 GB uncompressed pair):
   ```bash
   zstd -T0 --long=31 seeded-x86_64.raw -o seeded-x86_64.raw.zst
   zstd -T0 --long=31 rescue-data.raw   -o rescue-data.raw.zst
   ```
3. Start the session and keep the Mac awake. `sem debug job` does **not** attach
   to the machine that ran a finished job — it starts a *new* job and agent, so
   nothing from the preflight persists:
   ```bash
   caffeinate -dims sem debug job "$ORIGINAL_JOB_ID" --duration 3h
   ```
   Do not close this terminal and do not type `exit` until evidence is archived.
   Disconnecting stops the debug job and destroys the agent. Three hours is the
   documented example; Semaphore publishes no absolute maximum.

### Job side

4. Install and initialize exactly as the preflight did — Zabbly Incus, then
   `incus admin init --preseed` with the `dir` pool and `incusbr0`.
5. Transfer the two `.zst` artifacts, decompress, and verify the **uncompressed**
   digests against the envelope values before using them.
6. Follow `docs/docs/how-to/verify-boot-acceptance.md` sections 1–4 to create the
   VM. The TPM must be added **before** the first start; Incus cannot hot-plug it.
7. Capture the **source** seed baseline — this is an input baseline, not half of
   a before/after pair on the same device:
   `source-seed.before.sha256`, `source-seed.before.list`. It must contain an
   `install.*` entry.
8. Start and watch the install on serial:
   ```bash
   incus start "$VM" --console
   ```
   Expect the TUI, then these exact strings from `install.go:388-389`:
   ```
   IncusOS was successfully installed
   Please remove the install media to complete the installation
   ```
   Detach with **Ctrl+A then Q** — never `exit`, which would end the Semaphore
   debug session and destroy the machine. Then persist the log:
   ```bash
   incus console "$VM" --show-log | tee "$EVIDENCE/install-serial.log"
   ```
9. **Corrected seed-consumption oracle.** Stop the VM, detach the install media
   and the target, then inspect the *target* in a throwaway VM:
   ```bash
   incus stop "$VM"
   incus config device remove "$VM" install-media
   incus config device remove "$VM" install-target
   # attach the target volume alone to a disposable Linux VM, then inside it:
   udevadm settle
   TARGET_SEED=$(lsblk -nrpo NAME,PARTLABEL | awk '$2=="seed-data"{print $1;exit}')
   dd if="$TARGET_SEED" bs=4M | sha256sum   # -> target-seed.after.sha256
   dd if="$TARGET_SEED" bs=4M | tar -tf -   # -> target-seed.after.list
   ```
   Assert: source baseline **contains** `install.*`; target listing **lacks** it;
   the two digests **differ**; and optionally the source digest is unchanged.
10. Continue with recovery (`RESCUE_DATA` detection and signed-payload
    acceptance), archive all evidence, and only then exit the session.

## Residual risks

1. **Session death loses everything.** The agent is ephemeral and dies with the
   SSH session; archive evidence off-box before exiting, not at the end.
2. **No published debug-duration maximum.** Three hours is documented by
   example only, and no Cloud idle-timeout is stated. Budget for the possibility
   that the session ends early, and checkpoint evidence as you go.
3. **Serial mirroring is Incus-specific** and could change with an upstream
   release. Re-check `tui.go` when the pin moves.
4. **Seed consumption remains unobserved** everywhere. This runbook makes the
   observation *possible*; it does not predict the outcome.
5. `N-MEDIA-3` (NUL-padded ISO label) is still untested — the raw configuration
   matches the GPT partlabel and never reads the ISO identifier. An optional
   second ISO run closes it.
