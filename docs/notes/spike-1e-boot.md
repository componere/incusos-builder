# Spike 1.E — Boot gate feasibility: qemu boot of the seeded IncusOS image on macOS/HVF

Date: 2026-08-15 (wall clock below is local `date +%H:%M:%S`)
Host: macOS 25.4.0 arm64, Apple M4 Max. QEMU 10.2.0 (Homebrew), HVF accel.
Firmware: `/opt/homebrew/share/qemu/edk2-aarch64-code.fd` (edk2-stable202408-prebuilt.qemu.org).
No swtpm on this host (no TPM 2.0 emulation available). No local Incus server.

Artifact under test: `spikes/splice/out/seeded.img`
(IncusOS 202608102114 aarch64, seed tar spliced at byte 2,148,532,224; seed = install + network sections).

Integrity check — artifact untouched by this spike:

```
$ shasum -a 256 spikes/splice/out/seeded.img
ecd75e4723ead89bfd123186a7f1889d8255a011d7fffe80b2f059f0d4b2d545  spikes/splice/out/seeded.img
```

matches `sha256_seeded` in `spikes/splice/out/MANIFEST.txt`. All boots ran against a qcow2 overlay:

```
$ qemu-img create -f qcow2 -b .../spikes/splice/out/seeded.img -F raw /tmp/spike1e/scratch.qcow2
```

## Outcome classification

**Partial boot.** UEFI + systemd-boot + Linux kernel + systemd PID 1 all start. The guest then
goes permanently idle on a systemd start job with **no timeout**, and produces **zero disk writes,
zero network packets, and zero console output** after kernel handoff. **No seed consumption was
observable.** Rescue-media detection could not be evaluated (no console/log channel reachable).

## Attempt 1 — plain UEFI boot

Exact command (run in a detached tmux session, `tmux -L claude-agent`, session `bootgate`):

```
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 4096 -smp 4 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=/tmp/spike1e/scratch.qcow2,if=virtio,format=qcow2 \
  -serial mon:stdio -display none
```

Timing: started 15:08:09, killed 15:12:01 — **3m52s** total. UEFI → kernel handoff happened in
**under 10 s** (the post-handoff line was already present at the +10 s capture).

Serial console, verbatim, complete (this is *everything* the guest ever emitted):

```
UEFI firmware (version edk2-stable202408-prebuilt.qemu.org built at 16:28:50 on Sep 12 2024)
ArmTrngLib could not be correctly initialized.
Error: Image at 0013FDB6000 start failed: 00000001
Error: Image at 0013FD6D000 start failed: Not Found
Error: Image at 0013FCBA000 start failed: Unsupported
Error: Image at 0013FC3F000 start failed: Not Found
Error: Image at 0013FB65000 start failed: Aborted
Tpm2SubmitCommand - Tcg2 - Not Found
Tpm2GetCapabilityPcrs fail!
Tpm2SubmitCommand - Tcg2 - Not Found
Image type X64 can't be loaded on AARCH64 UEFI system.
BdsDxe: loading Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x2,0x0)
ConvertPages: failed to find range 1C22E0000 - 1C2301FFF
BdsDxe: starting Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x2,0x0)
                              IncusOS 202608102114
                         Reboot Into Firmware Interface
                   ------------------------------------------
                                  Boot in 1 s.
ConvertPages: failed to find range 1D1F80000 - 1D893CFFF
```

Decisive readings from those lines:

- **UEFI finds the bootloader.** `BdsDxe: starting Boot0001 "UEFI Misc Device"` followed by the
  systemd-boot menu titled `IncusOS 202608102114`. The splice did not break bootability.
- **No TPM.** `Tpm2SubmitCommand - Tcg2 - Not Found` / `Tpm2GetCapabilityPcrs fail!` — the firmware
  has no TCG2 protocol, so the guest has no TPM 2.0 to measure into or seal against.
- The `Image type X64 can't be loaded on AARCH64` and `Error: Image at ... start failed` lines are
  edk2 enumerating the ESP and rejecting the x86 shim/loader copies in the multi-arch ESP —
  benign, it proceeds to the correct aarch64 entry.
- **After the final `ConvertPages` (kernel/UKI handoff), the serial line is silent forever.**

### The guest is alive but idle — evidence

Because nothing appeared on `ttyAMA0`, liveness was established through the QEMU monitor instead
of the console.

Host CPU for the qemu process sat at **0.1%**, i.e. the vCPUs are in WFI, not spinning:

```
$ ps -Ao pid,%cpu,etime,comm | grep qemu
75984  0.1 01:36    qemu-system-aar
```

`info registers` on the stalled guest shows EL1 (kernel) execution and leaks the string data that
was last passed through the SIMD registers by `memcpy`:

```
PSTATE=00000000614000c5 -ZC- EL1h  BTYPE=0     FPCR=00000000 FPSR=00000000
Q03=2e2e2e2974696d69:6c206f6e202f2073
Q04=7265732e65676173:73656d2d746f6f62
Q05=696e6e7572207472:6174732f65636976
```

Decoding those little-endian 64-bit halves as ASCII (Q04 → Q05 → Q03):

```
Q04: "boot-mes" "sage.ser"
Q05: "vice/sta" "rt runni"
Q03: "s / no l" "imit)..."

=> "...boot-message.service/start running (<n>s / no limit)..."
```

That is systemd's job-status text. A second vCPU carried:

```
Q00=746f6e2f646d6574:7379732f6e75722f   -> "/run/sys" "temd/not"
Q01=00796669746f6e2f:646d65747379732f   -> "/systemd" "/notify\0"
```

`/run/systemd/notify` — the sd_notify socket. **systemd PID 1 is running.** The unit name could
not be resolved further: `grep -a 'boot-message'` over the whole 3.2 GB raw image returns nothing,
because the IncusOS root filesystem is a compressed/verity-protected image, so unit names are not
present as plain text on the disk.

Across two register samples ~50 s apart, all vCPUs sat at the *same* PC:

```
sample 1: PC=ffff80008170ed14 (x4)   X30=ffff80008170ed74
sample 2: PC=ffff80008170ed14 (x4)   X30=ffff80008170ed74
```

Same address, low host CPU: the kernel idle loop. Nothing is making progress.

## Attempt 1b — add a display device (was the console just invisible?)

Hypothesis: kernel output was going to a framebuffer rather than `ttyAMA0`. `-M virt` has no
graphics device by default, so one was added and the console dumped through the monitor.

```
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 4096 -smp 4 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=/tmp/spike1e/scratch.qcow2,if=virtio,format=qcow2 \
  -device virtio-gpu-pci,id=gpu0 \
  -serial file:/tmp/spike1e/serial-1b.log -monitor stdio -display none
```

Timing: started 15:12:08, killed 15:14:46 — **2m38s**.

Serial log: byte-for-byte the same as attempt 1 (same UEFI lines, same menu, same silence after
handoff — only the PCI slot changed, `Pci(0x3,0x0)`).

`screendump /tmp/spike1e/s2.ppm gpu0` at ~95 s after start rendered a black 1280x800 frame with the
single centred string:

```
Display output is not active.
```

i.e. the guest kernel never programmed the GPU either. **The hypothesis is refuted: there is no
hidden console — the guest genuinely stops producing output right after kernel handoff.**

## Attempt 2 — rescue media attached + network observation

```
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 4096 -smp 4 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=/tmp/spike1e/scratch.qcow2,if=virtio,format=qcow2 \
  -drive file=/tmp/spike1e/rescue.img,format=raw,if=virtio,readonly=on \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0 \
  -object filter-dump,id=d0,netdev=n0,file=/tmp/spike1e/net.pcap \
  -serial file:/tmp/spike1e/serial-2.log -monitor stdio -display none
```

(`/tmp/spike1e/rescue.img` is a copy of `spikes/rescue/out/rescue.img`; the original was never
opened by qemu.)

Timing: started 15:15:04, killed 15:19:08 — **4m04s**.

Results after ~4 minutes of runtime:

- Serial log: identical to attempts 1 and 1b. **No `RESCUE_DATA` line, no incus-osd line, nothing.**
  Rescue detection is therefore **untested**, not "failed" — the guest never reached the code path
  and never had a channel to report on.
- **Zero network packets:**

  ```
  $ ls -la /tmp/spike1e/net.pcap
  -rw-r--r--  1 josh  wheel  24 Aug 15 15:15 /tmp/spike1e/net.pcap
  ```

  24 bytes is the bare pcap file header. Not one frame — no DHCP DISCOVER, no IPv6 RS, nothing.
  The network section of the seed was definitively **not** applied.
- **Zero disk writes**, across all three attempts combined (the overlay was never re-created):

  ```
  $ ls -la /tmp/spike1e/scratch.qcow2
  -rw-r--r--  1 josh  wheel  196664 Aug 15 15:07 /tmp/spike1e/scratch.qcow2
  ```

  196664 bytes is the size at `qemu-img create` time, 15:07, before the first boot. A copy-on-write
  overlay that has taken a single guest write grows. The install section of the seed was
  definitively **not** applied — incus-osd never touched the target disk.
- **No response to ACPI shutdown.** `system_powerdown` on the monitor, then 25 s later:

  ```
  (qemu) system_powerdown
  (qemu) info status
  VM status: running
  ```

  A healthy systemd guest powers off within a couple of seconds. It is wedged, not merely quiet.

## Where it stops, and the most probable why

Reached: UEFI ✓ → systemd-boot menu ✓ → UKI/kernel handoff ✓ → kernel ✓ → systemd PID 1 ✓.
Stopped: an early systemd job that is `running (<n>s / no limit)` — i.e. a job systemd will wait on
indefinitely — before any console, disk, or network activity.

The strongest correlated observation is the firmware TPM failure: `Tpm2SubmitCommand - Tcg2 - Not
Found` and `Tpm2GetCapabilityPcrs fail!`. IncusOS is an immutable, measured-boot OS that seals its
disk encryption to TPM 2.0 PCRs. With no TPM in the machine at all there is nothing to unseal
against, and an unlock/measure unit blocking with no timeout is exactly the shape of what was
observed. This is consistent but **not proven** — proving it requires the console, which requires a
guest that boots far enough to open one. `-M virt` on QEMU offers no TPM without an external
`swtpm`, which is not installable/available on this host, so the one obvious variation (attach a
TPM) is **not executable here**. Attempt 1b (add a display device) was spent instead, and ruled out
the competing "console is just invisible" explanation.

Secondary, unresolved possibility worth noting for whoever picks this up with a TPM: the seeded
image is an *install* image with only one disk attached, so an installer waiting for a distinct
target disk would also hang silently. A follow-up should attach a second blank virtio disk as the
install target.

## Implication for a CI boot gate

The evidence says a boot gate is **not achievable on macOS/HVF as configured**, and the blockers
are infrastructure, not code:

1. **A TPM 2.0 device is mandatory to even find out.** Every attempt died upstream of any
   IncusOS-specific behaviour, and the only guest-visible hardware fault is the missing TCG2
   protocol. Any gate runner must provide `swtpm` + `-tpmdev emulator` (Linux) or an equivalent.
   `swtpm` is not available on this macOS host, which alone disqualifies developer laptops as the
   gate environment.
2. **A serial console that actually carries kernel output is mandatory.** On `-M virt`/aarch64 the
   guest emitted nothing on `ttyAMA0` and never lit a framebuffer, so this run had no channel to
   assert on — all the findings above had to be reconstructed from QEMU monitor registers, pcap
   size, and overlay size. That is not something to build assertions on. A gate needs a
   platform/console combination where the kernel demonstrably logs (in practice: x86_64 with
   `console=ttyS0`, which is what IncusOS is primarily exercised on upstream).
3. **Therefore: a Linux runner.** Recommended shape, in increasing order of confidence:
   - GitHub-hosted `ubuntu-latest` x86_64 + **software** QEMU/KVM. Nested virt is *not* required
     if the gate boots the image under plain QEMU with `swtpm`; GitHub's Azure runners do now
     expose KVM on the standard Linux images, but even TCG (no accel) is viable if the gate only
     needs "did it consume the seed", accepting a slow boot. This is the cheapest option and should
     be tried first.
   - A **self-hosted Linux runner** with `/dev/kvm` + `swtpm`, if hosted-runner KVM proves flaky or
     too slow. Buys wall-clock and removes the accel uncertainty.
   - **Incus on a Linux host** (the upstream-native path): `incus launch` of the IncusOS image as a
     VM gives TPM, UEFI, and console handling for free and is the closest match to how IncusOS is
     actually tested. This requires a Linux host to run the Incus server — macOS is client-only —
     so it implies self-hosted or a Linux CI job, not a developer macOS machine.
4. **Cheap non-boot assertions retain value regardless.** Even on macOS, the overlay-size and pcap
   checks used here are good, cheap oracles: a successful seeded install *must* write to the target
   disk, and a successful network seed *must* emit at least a DHCP frame. Those make solid gate
   assertions once the boot itself is unblocked, and they need no in-guest agent.
5. **Timing is not the constraint.** UEFI→kernel handoff was under 10 s and each full attempt was
   ~3–4 minutes wall clock, of which most was deliberate observation. A working gate should
   comfortably fit a few minutes per boot.

Rescue-media detection (`/dev/disk/by-partlabel/RESCUE_DATA`, `/dev/disk/by-label/RESCUE_DATA`)
remains **entirely unverified** and should be re-attempted first on whichever Linux/TPM environment
is chosen, since it is a pure log-observation check once a console exists.

## Cleanup

All qemu processes terminated (`ps -Ao pid,comm | grep -c '[q]emu-system'` → `0`), the
`claude-agent` tmux server killed, and `/tmp/spike1e` (overlay, rescue copy, logs, screendumps)
removed. `spikes/splice/out/seeded.img` and `spikes/rescue/out/*` were never opened for writing.

---

## DECISION (1.E — boot acceptance gate)

**The boot acceptance gate is a documented release-checklist item for v1, with a
time-boxed CI attempt scheduled inside Phase 5 — not a merge blocker.**

Rationale from the evidence above:

1. Developer macOS hosts are disqualified: no swtpm (TPM 2.0 emulation), and
   aarch64 `-M virt` never produced a usable console; the guest wedges on a
   no-timeout systemd start job consistent with TPM-sealed disk unlock.
2. The plausible CI shape (GitHub-hosted x86_64 runner + QEMU + swtpm +
   `console=ttyS0`, or a self-hosted Linux runner with Incus) is UNPROVEN.
   Phase 5.2 gets one time-boxed attempt at the ubuntu-runner variant; if it
   does not produce observable seed consumption within that box, the checklist
   stands.
3. The checklist run requires a real Linux host with Incus (`incus launch`
   provides UEFI+TPM+console for free). The exact commands and the two cheap
   oracles proven here — target-disk write growth and ≥1 DHCP frame — go into
   the Phase 6 how-to.

Consequence for Phase 5.1: everything except the two boot checks (osd seed
consumption, recovery-path acceptance) is automatable locally and stays in the
merge-gating T3 suite.
