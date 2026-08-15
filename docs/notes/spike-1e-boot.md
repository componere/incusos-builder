# Spike 1.E — Boot gate feasibility: qemu boot of the seeded IncusOS image on macOS/HVF

Date: 2026-08-15 (wall clock below is local `date +%H:%M:%S`)
Host: macOS 25.4.0 arm64, Apple M4 Max. QEMU 10.2.0 (Homebrew), HVF accel (aarch64) / TCG (x86_64).
Firmware: `/opt/homebrew/share/qemu/edk2-aarch64-code.fd`, `edk2-x86_64-code.fd`,
`edk2-x86_64-secure-code.fd` + `edk2-i386-vars.fd` (edk2-stable202408-prebuilt.qemu.org).
TPM 2.0 emulation: **`swtpm` 0.10.1**, `brew install swtpm` (bottled, 4.1 s, no compilation).
Also installed for offline inspection: `mtools` (`mdir`/`mcopy`/`mtype`). No local Incus server.

Artifacts under test:

- `spikes/splice/out/seeded.img` — IncusOS 202608102114 **aarch64**, seed tar spliced at byte
  2,148,532,224 (seed = install + network sections).
- `/tmp/spike1e2/x86.img` — IncusOS 202608102114 **x86_64** `image-raw`, downloaded fresh for round 2
  and spliced with the *same* `spikes/splice/out/seed-install-network.tar` by a throwaway
  `python3` writer in `/tmp` (nothing under `spikes/` was written).

Integrity check — the round-1 artifact was untouched by round 2 as well:

```
$ shasum -a 256 spikes/splice/out/seeded.img
ecd75e4723ead89bfd123186a7f1889d8255a011d7fffe80b2f059f0d4b2d545  spikes/splice/out/seeded.img
```

matches `sha256_seeded` in `spikes/splice/out/MANIFEST.txt`. Every boot ran against a qcow2 overlay,
never the raw file.

## Outcome classification

**Partial boot, on both architectures, with and without a TPM.** UEFI → systemd-boot → UKI →
kernel handoff succeed everywhere. The guest then goes idle (vCPU halted, ~0.5 % host CPU) with
**zero writes to the attached blank target disk, zero guest-originated network frames, and zero
post-handoff console output**. No seed consumption was observed in any configuration.

Round 2 changed three of round 1's conclusions:

1. The silence after kernel handoff is **explained by the shipped kernel command line**, not by a
   wedge: the UKI carries `quiet loglevel=0 systemd.show_status=0` and **no `console=` argument at
   all**, on *both* architectures. A perfectly healthy IncusOS boot is silent here.
2. **A TPM was attached and detected** (`swtpm` + `tpm-tis-device`, firmware enumerated four PCR
   banks). Behaviour did not change. The round-1 "missing TPM is the most probable why" hypothesis
   is **refuted**.
3. **Secure Boot is a hard IncusOS gate, and it is the first thing that actually stops the boot on
   a non-SB firmware** — verbatim: `Unable to determine SecureBoot state. IncusOS will only boot on
   UEFI systems.` With an SB-capable firmware the image self-enrolls its own keys and gets past it.

---

# Offline evidence (no boot required)

## The UKI kernel command line

`spikes/splice/out/seeded.img` is raw GPT at 512 B/sector; the ESP is partition 1 at LBA 2048
(byte 1,048,576), so mtools can read it in place with an `@@offset` image spec.

```
$ mdir -i spikes/splice/out/seeded.img@@1048576 ::/EFI/Linux
 Volume in drive : is ESP
Directory for ::/EFI/Linux

INCUSO~1 EFI  110839736 2026-08-10  22:00  IncusOS_202608102114.efi
```

Extracted (`mcopy`, 0.12 s) and its PE sections dumped with `python3`:

```
machine=0xaa64 nsec=13 optsz=240
.linux     vsize=52101048    rawptr=99328
.initrd    vsize=58635056    rawptr=52200448
.cmdline   vsize=365         rawptr=97792
.uname     vsize=13          rawptr=98304
.pcrpkey   vsize=451         rawptr=98816
.pcrsig    vsize=2100        rawptr=110835712
```

`.cmdline`, verbatim (aarch64):

```
usrhash=d4e154fe5b30c8c166528926f027434ecb0c4628135a488761c1de24bd18fb70 rw vt.handoff=1 intel_iommu=on quiet loglevel=0 systemd.show_status=0 rootflags=noexec,nodev,nosuid rd.systemd.mount-extra=/dev/disk/by-partlabel/esp:/boot:vfat:rw modprobe.blacklist=nvidiafb systemd.image_policy=esp=unprotected:usr=signed:root=encrypted+absent:swap=encrypted+absent:=ignore
```

`.cmdline`, verbatim (x86_64, from the freshly downloaded `image-raw`):

```
usrhash=bb251ec0e78e9ab24de22bf6fe05a4f884d1fad0f42a521ac20dfdb89462c7ca rw vt.handoff=1 intel_iommu=on quiet loglevel=0 systemd.show_status=0 rootflags=noexec,nodev,nosuid rd.systemd.mount-extra=/dev/disk/by-partlabel/esp:/boot:vfat:rw modprobe.blacklist=nvidiafb systemd.image_policy=esp=unprotected:usr=signed:root=encrypted+absent:swap=encrypted+absent:=ignore
```

`.uname` on both: `7.1.8-zabbly+`. `.osrel` on aarch64: `PRETTY_NAME="IncusOS 202608102114" … IMAGE_VERSION="202608102114"`.

**Readings — this is the single most load-bearing finding of round 2:**

- **No `console=` argument on either architecture.** The round-1 CI implication "a gate needs
  x86_64 with `console=ttyS0`, which is what IncusOS is primarily exercised on upstream" is
  **false as stated**: the shipped x86_64 UKI configures no console either. Any expectation of
  kernel log output must come from *adding* a console, and `loader.conf` (below) forbids editing
  the command line at the menu.
- **`quiet loglevel=0 systemd.show_status=0`** suppresses kernel messages and systemd job status
  even if a console existed. Post-handoff silence is therefore *expected output*, not a symptom.
  Round 1's serial silence carries no information about guest health.
- `systemd.image_policy=…usr=signed:root=encrypted+absent…` confirms a measured/encrypted design
  (relevant to TPM), and `rd.systemd.mount-extra=/dev/disk/by-partlabel/esp:/boot` confirms the
  partlabel-based device naming that the rescue-media check (`RESCUE_DATA`) also relies on.

## ESP layout and boot policy

```
$ mdir -i spikes/splice/out/seeded.img@@1048576 ::
EFI <DIR>   loader <DIR>   keys <DIR>

$ mtype -i spikes/splice/out/seeded.img@@1048576 ::/loader/loader.conf
secure-boot-enroll force
timeout 3
editor no

$ mdir -i … ::/keys
SECURE~1 DER  663  secureboot-DB-2025-R1.der
SECURE~2 DER  662  secureboot-DB-2026-R1.der
SECURE~3 DER  661  secureboot-KEK-R1.der
SECURE~4 DER  660  secureboot-PK-R1.der
```

- `secure-boot-enroll force` — systemd-boot will enroll the shipped PK/KEK/DB into a firmware in
  setup mode and reboot. This is why round 2's attempt C behaves differently from B.
- `editor no` — **the kernel command line cannot be edited from the boot menu.** Adding
  `console=ttyS0` for a CI gate requires either rebuilding the UKI or a firmware/DT-level console,
  not an interactive keystroke.

GPT of the seeded image (parsed directly, 512 B sectors):

```
LBA 2048     byte 1048576      2147483648  esp
LBA 4196352  byte 2148532224    104857600  seed-data
LBA 4401152  byte 2253389824        16384  IncusOS_202608102114_verity_sig
LBA 4401184  byte 2253406208    104857600  IncusOS_202608102114_verity
LBA 4605984  byte 2358263808   1073741824  IncusOS_202608102114
```

The x86_64 image has the identical `seed-data` start (byte 2,148,532,224), which is why the same
splice offset worked unchanged across architectures.

---

# Round 1 — history (macOS/HVF, aarch64, no TPM, one disk)

Kept for the record. Two framings below are **corrected** relative to what round 1 wrote.

## Attempt 1 — plain UEFI boot

```
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 4096 -smp 4 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -drive file=/tmp/spike1e/scratch.qcow2,if=virtio,format=qcow2 \
  -serial mon:stdio -display none
```

Timing: 15:08:09 → 15:12:01, **3m52s**. UEFI → kernel handoff in **under 10 s**.

Serial console, verbatim and complete:

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

Still-valid readings: UEFI finds and starts the bootloader (`BdsDxe: starting Boot0001` + the
systemd-boot menu titled `IncusOS 202608102114`) — **the splice did not break bootability**; the
firmware has no TCG2 protocol (`Tpm2SubmitCommand - Tcg2 - Not Found`); the `Image type X64` and
`Error: Image at …` lines are edk2 rejecting the x86 loader copies in the multi-arch ESP and are
benign.

Liveness was established through the QEMU monitor: host CPU 0.1 % (vCPUs in WFI), `info registers`
showing EL1h execution, and SIMD registers carrying systemd strings:

```
Q04: "boot-mes" "sage.ser"   Q05: "vice/sta" "rt runni"   Q03: "s / no l" "imit)..."
=> "...boot-message.service/start running (<n>s / no limit)..."
Q00/Q01: "/run/systemd/notify\0"
```

systemd PID 1 is running. Across two samples ~50 s apart all vCPUs sat at `PC=ffff80008170ed14`.

## Attempt 1b — add a display device

```
… -device virtio-gpu-pci,id=gpu0 -serial file:/tmp/spike1e/serial-1b.log -monitor stdio -display none
```

Timing: 15:12:08 → 15:14:46, **2m38s**. Serial log byte-for-byte identical to attempt 1.
`screendump gpu0` at ~95 s rendered a black 1280x800 frame reading `Display output is not active.`

**Corrected framing (round-1 wrote this as a refutation of "hidden console"; it is narrower than
that).** The screendump rules out a *graphical* console only. It does **not** distinguish "the
guest wedged before opening a console" from "no console was ever configured for this machine".
Round 2 settled that question offline: the UKI carries no `console=` and sets `quiet loglevel=0`,
so **no console was configured** — the second explanation is the correct one. The wedge conclusion
that round 1 drew rests entirely on the register evidence (`boot-message.service/start running`,
`/run/systemd/notify`, unchanging PC, ignored ACPI powerdown), never on the display test.

## Attempt 2 — rescue media attached + network observation

```
… -drive file=/tmp/spike1e/rescue.img,format=raw,if=virtio,readonly=on \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0 \
  -object filter-dump,id=d0,netdev=n0,file=/tmp/spike1e/net.pcap …
```

Timing: 15:15:04 → 15:19:08, **4m04s**. Serial log identical. Results:

- pcap 24 bytes = bare file header, not one frame.
- `/tmp/spike1e/scratch.qcow2` still 196,664 bytes — its size at `qemu-img create` time.
- `system_powerdown` ignored; `info status` → `VM status: running` 25 s later.

**Corrected framing (round-1 wrote "the install section of the seed was definitively not
applied").** The overlay measured was the copy-on-write overlay over the **install media itself**,
and round 1 attached no separate target disk. A *successful* IncusOS install writes to a distinct
target disk and would legitimately leave that overlay untouched, so the measurement cannot support
a claim about the install section. What it does support is the weaker, still-useful statement:
**the guest wrote nothing anywhere** — not to the media, not to the network. Round 2 attached a
blank target disk and re-ran the oracle properly (below).

---

# Round 2 — TPM attached, target disk attached, both architectures

Prerequisites verified first:

```
$ brew install swtpm            # bottled, 4.11 s wall
$ swtpm --version
TPM emulator version 0.10.1, Copyright (c) 2014-2022 IBM Corp. and others
$ qemu-system-aarch64 -tpmdev help
Supported TPM types (choose only one):
    emulator   TPM emulator backend driver
$ qemu-system-aarch64 -device help | grep -i tpm
name "tpm-tis-device", bus System
name "tpm-tis-i2c", bus i2c-bus
$ qemu-system-x86_64 -device help | grep -i tpm
name "tpm-crb"
name "tpm-tis", bus ISA
```

TPM backend for every round-2 attempt:

```
swtpm socket --tpm2 --tpmstate dir=/tmp/spike1e2/tpmstate --ctrl type=unixio,path=/tmp/spike1e2/swtpm.sock --log level=1
```

## Attempt A — aarch64 + swtpm + blank target disk + rescue media + pcap

```
qemu-system-aarch64 -M virt -accel hvf -cpu host -m 4096 -smp 4 \
  -bios /opt/homebrew/share/qemu/edk2-aarch64-code.fd \
  -chardev socket,id=chrtpm,path=/tmp/spike1e2/swtpm.sock \
  -tpmdev emulator,id=tpm0,chardev=chrtpm \
  -device tpm-tis-device,tpmdev=tpm0 \
  -drive file=/tmp/spike1e2/src.qcow2,if=virtio,format=qcow2 \
  -drive file=/tmp/spike1e2/target.qcow2,if=virtio,format=qcow2 \
  -drive file=/tmp/spike1e2/rescue.img,if=virtio,format=raw,readonly=on \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0 \
  -object filter-dump,id=d0,netdev=n0,file=/tmp/spike1e2/net-a.pcap \
  -serial mon:stdio -display none
```

(`src.qcow2` = qcow2 overlay over `seeded.img`; `target.qcow2` = **blank 8 GiB** install target;
`rescue.img` = a copy of `spikes/rescue/out/rescue.img`, attached read-only.)

Timing: 15:47:22 → 15:54:01, **6m39s**.

Serial console, verbatim and complete — compare against round 1's TPM lines:

```
UEFI firmware (version edk2-stable202408-prebuilt.qemu.org built at 16:28:50 on Sep 12 2024)
SyncPcrAllocationsAndPcrMask!
ArmTrngLib could not be correctly initialized.
Error: Image at 0013FDB6000 start failed: 00000001
Error: Image at 0013FD6D000 start failed: Not Found
Error: Image at 0013FC2C000 start failed: Not Found
Error: Image at 0013FB52000 start failed: Aborted
Tpm2GetCapabilityPcrs - 00000004
alg - 4
alg - B
alg - C
alg - D
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

**TCG2 is present.** `Tpm2SubmitCommand - Tcg2 - Not Found` and `Tpm2GetCapabilityPcrs fail!` are
gone; instead the firmware syncs PCR allocations and enumerates four PCR banks
(`alg - 4` = SHA1, `B` = SHA256, `C` = SHA384, `D` = SHA512). `swtpm` persisted state
(`tpmstate/tpm2-00.permall`, 1689 B).

**Boot progress delta versus round 1: none that is observable.** Post-handoff the serial line is
silent again (now known to be the configured behaviour), and:

| oracle | at create | after 6m39s |
|---|---|---|
| `target.qcow2` (blank install target) | 196,736 B | **196,736 B** — no growth |
| `src.qcow2` (overlay over install media) | 196,664 B | **196,664 B** — no growth |
| `net-a.pcap` | 24 B (header) | **24 B** — zero frames, in either direction |

Monitor evidence: `system_powerdown` ignored, `info status` → `VM status: running` 30 s later —
same as round 1. Register samples still show systemd alive and idle; the string
`/run/systemd/notify` reappears in the SIMD registers:

```
Q25=0000000000000000:000000796669746f   Q26=6e2f646d65747379:732f6e75722f0001
=> "/run/systemd/notify"
```

PC sampling differed from round 1 in one respect worth recording: successive `info registers` on
CPU#0 returned different addresses (`ffff80008019f040`, `ffff8000802579f4`, `ffff80008019ef80`,
`ffff80008170d9c8`) rather than round 1's single pinned PC, and one sample carried the string
`/usr/lib…` in Q01. Host CPU stayed at 0.1 %. That is consistent with a kernel idle loop plus
timer ticks, not with forward progress — the I/O oracles are flat.

**Conclusion for attempt A: attaching a TPM 2.0 changes the firmware log and nothing else.** The
round-1 hypothesis that the missing TCG2 protocol caused the stall is refuted by direct experiment.

## Attempt B — x86_64 + swtpm, non-Secure-Boot firmware

The x86_64 `image-raw` was cheap to obtain, so this was **not** skipped:

```
$ curl -sSL -o x86.img.gz https://images.linuxcontainers.org/os/202608102114/x86_64/IncusOS_202608102114.img.gz
   608,288,636 B in under 20 s
$ gunzip -c x86.img.gz > x86.img        # real 0m2.863s → 3,432,026,112 B
$ python3  # write seed-install-network.tar (3072 B) at byte 2148532224; read-back compare == True
```

```
qemu-system-x86_64 -machine q35 -m 4096 -smp 4 \
  -drive if=pflash,format=raw,readonly=on,file=/opt/homebrew/share/qemu/edk2-x86_64-code.fd \
  -drive if=pflash,format=raw,file=/tmp/spike1e2/x86-vars.fd \
  -chardev socket,id=chrtpm,path=/tmp/spike1e2/swtpm-b.sock \
  -tpmdev emulator,id=tpm0,chardev=chrtpm -device tpm-tis,tpmdev=tpm0 \
  -drive file=/tmp/spike1e2/x86-src.qcow2,if=virtio,format=qcow2 \
  -drive file=/tmp/spike1e2/x86-target.qcow2,if=virtio,format=qcow2 \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0 \
  -object filter-dump,id=d0,netdev=n0,file=/tmp/spike1e2/net-b.pcap \
  -serial mon:stdio -display none
```

TCG only — HVF cannot run x86 guests on Apple silicon. Timing: 15:55:47 → 15:58:07, **2m20s**.

Serial console, verbatim and complete:

```
BdsDxe: loading Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
BdsDxe: starting Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
                                        IncusOS 202608102114
                                   Reboot Into Firmware Interface
                             ──────────────────────────────────────────
                                            Boot in 1 s.
IncusOS is starting...
Unable to determine SecureBoot state. IncusOS will only boot on UEFI systems.
```

**This is the first IncusOS-originated diagnostic any run of this spike has produced**, and it is
decisive: IncusOS's EFI stub refuses to proceed on a firmware without Secure Boot support.
`edk2-x86_64-code.fd` is the non-SB build. Oracles after the halt: `x86-target.qcow2` 196,736 B
(no growth), `net-b.pcap` 24 B (zero frames).

Two things follow immediately. First, useful IncusOS output *does* reach the serial port on this
platform even with `quiet loglevel=0` — because it is emitted through UEFI console services before
kernel handoff, not by the kernel. Second, whatever a gate runs on **must provide Secure Boot**.

## Attempt C — x86_64 + swtpm + Secure-Boot-capable firmware

```
qemu-system-x86_64 -machine q35,smm=on -m 4096 -smp 4 \
  -global driver=cfi.pflash01,property=secure,value=on \
  -drive if=pflash,format=raw,readonly=on,file=/opt/homebrew/share/qemu/edk2-x86_64-secure-code.fd \
  -drive if=pflash,format=raw,file=/tmp/spike1e2/x86-vars-sec.fd \
  -chardev socket,id=chrtpm,path=/tmp/spike1e2/swtpm-c.sock \
  -tpmdev emulator,id=tpm0,chardev=chrtpm -device tpm-tis,tpmdev=tpm0 \
  -drive file=/tmp/spike1e2/x86-src2.qcow2,if=virtio,format=qcow2 \
  -drive file=/tmp/spike1e2/x86-target2.qcow2,if=virtio,format=qcow2 \
  -netdev user,id=n0 -device virtio-net-pci,netdev=n0 \
  -object filter-dump,id=d0,netdev=n0,file=/tmp/spike1e2/net-c.pcap \
  -serial mon:stdio -display none
```

(`x86-vars-sec.fd` is a fresh copy of `edk2-i386-vars.fd`, i.e. a varstore in setup mode.)

Timing: 15:58:32 → 16:06:19, **7m47s**.

Serial console, verbatim and complete:

```
BdsDxe: loading Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
BdsDxe: starting Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
Enrolling secure boot keys from directory: \loader\keys\auto
Custom Secure Boot keys successfully enrolled, rebooting the system now!
BdsDxe: loading Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
BdsDxe: starting Boot0001 "UEFI Misc Device" from PciRoot(0x0)/Pci(0x3,0x0)
                                        IncusOS 202608102114
                                   Reboot Into Firmware Interface
                             ──────────────────────────────────────────
                                            Boot in 1 s.
IncusOS is starting...
```

**The furthest any attempt has reached, and the first observed guest write.** `secure-boot-enroll
force` did exactly what `loader.conf` says: enrolled the four shipped DER keys from the ESP,
rebooted, and on the second pass the `Unable to determine SecureBoot state` line is **absent** —
the SB gate passed. Then, as before, kernel handoff and silence.

Oracles for attempt C:

| oracle | at create | final | reading |
|---|---|---|---|
| `x86-src2.qcow2` (overlay over install media) | 196,736 B | **917,504 B** | **grew** — pre-kernel ESP writes (key enrollment + systemd-boot random seed) |
| `x86-target2.qcow2` (blank install target) | 196,736 B | **196,736 B** | no growth — installer never wrote |
| `net-c.pcap` | 24 B | 174 B | **1 frame, inbound only** (see below) |
| host CPU (TCG) after handoff | — | **0.5 %** | vCPU halted, not emulating |
| `info status` | — | `VM status: running` | |
| `info tpm` | — | `tpm0: model=tpm-tis` / `type=emulator,chardev=chrtpm` | TPM attached |
| `info registers` | — | `RIP=ffffffff9d25ad6b … CPL=0 … HLT=1` | 64-bit kernel text, halted in idle |

The single pcap frame is **not** guest traffic — parsed:

```
frame1 len=134 ethertype=0x86dd src=52:56:00:00:00:02 dst=33:33:00:00:00:01
  IPv6 next-hdr=58  ICMPv6 type=134 (Router Advertisement)
```

`52:56:00:00:00:02` is QEMU slirp's own gateway MAC: this is the host-side router advertising *to*
the guest. **The guest emitted zero frames** — no DHCP DISCOVER, no RS, no ARP. The DHCP oracle is
therefore flat on x86_64 too, and the pcap must be filtered by source MAC before it is used as an
assertion, or slirp's own RA will produce a false positive.

## What round 2 proved, and what remains open

Proven:

- `swtpm` 0.10.1 installs from a bottle on this host in seconds; QEMU 10.2.0 exposes
  `-tpmdev emulator` with `tpm-tis-device` (aarch64) and `tpm-tis` (x86_64); a TPM was attached
  and detected by firmware on both architectures. **The "no TPM emulation available here" premise
  is false and the TPM hypothesis is refuted** — the guest behaves identically with one.
- The shipped UKI carries **no `console=` and `quiet loglevel=0 systemd.show_status=0`** on both
  architectures, and `loader.conf` sets `editor no`. **Post-handoff silence is designed, not
  diagnostic**, and it cannot be fixed by a boot-menu keystroke.
- **Secure Boot is a hard gate**: `Unable to determine SecureBoot state. IncusOS will only boot on
  UEFI systems.` on a non-SB firmware; `secure-boot-enroll force` self-enrolls on an SB-capable one
  and clears the gate. A gate environment must supply an SB-capable firmware with a writable,
  setup-mode varstore.
- The **target-disk oracle now exists and is flat**: a blank virtio disk was attached in all three
  round-2 attempts and grew by zero bytes. This is the oracle round 1 should have used.
- The **network oracle needs a source filter**: slirp emits an unsolicited RA that inflates the
  pcap without any guest participation.
- **x86_64 was not skipped.** Download 608,288,636 B in under 20 s, decompress 2.9 s, splice at the
  identical offset 2,148,532,224 (verified by read-back) — obtaining the second architecture cost
  under a minute, so the "x86_64 is untried" gap is closed.
- The splice is bootable on **both** architectures: UEFI → systemd-boot menu → UKI on aarch64 and
  x86_64 alike.

Open, and *not* answered by this spike:

- **Why the kernel goes idle after handoff.** With no console configured, no in-guest agent, and
  no writes to observe, this host offers no channel that would tell us. It is now known *not* to be
  the TPM, and on x86_64 *not* Secure Boot either.
- Whether IncusOS refuses to install because it cannot disambiguate the install target (two virtio
  disks were present in round 2: the media and the blank target) — still unmeasurable without a
  console.
- **Rescue-media detection remains entirely unverified.** It is a pure log-observation check and
  there is still no log.

# Implication for a CI boot gate

Grounded strictly in what round 2 measured:

1. **A gate runner must provide Secure Boot, not just a TPM.** This is the one hard requirement
   round 2 actually observed IncusOS enforcing, verbatim, and it was invisible to round 1 because
   aarch64 `-bios` produced no diagnostic. Requirements: an SB-capable firmware build plus a
   **writable, setup-mode varstore** (OVMF `*_VARS.fd` copy per run) so `secure-boot-enroll force`
   can enroll and reboot. A TPM should still be attached — it costs nothing now (`swtpm`) and the
   image's `image_policy` is built around measured boot — but round 2 shows a TPM alone buys
   nothing.
2. **Do not plan the gate around reading kernel logs.** No shipped IncusOS UKI configures a
   console, `quiet loglevel=0 systemd.show_status=0` suppresses what a console would carry, and
   `editor no` blocks adding one at the menu. Pre-kernel IncusOS stub messages *do* reach the
   serial port (that is how attempt B's Secure Boot line was captured), so serial is worth wiring
   up for firmware/stub diagnostics — but a gate that asserts on kernel or systemd output needs
   either a rebuilt UKI or an in-guest agent, and neither exists today.
3. **The usable oracles are out-of-guest and cheap, with two corrections.** Use *target-disk*
   growth (a blank qcow2 attached as a second virtio disk — the media overlay is the wrong file to
   watch, and on x86_64 it grows from firmware writes alone, which would be a false positive), and
   *guest-originated* frames only (filter the `filter-dump` pcap by source MAC; slirp's own router
   advertisement otherwise lands in it). Both were exercised in round 2 and both correctly reported
   "nothing happened".
4. **Neither macOS architecture reached seed consumption, but the blocker is no longer identified
   as host infrastructure.** aarch64/HVF with a TPM and x86_64/TCG with a TPM and Secure Boot both
   end in the same place: kernel running, halted, no I/O. Round 1 concluded "developer macOS hosts
   are disqualified because swtpm is unavailable"; that reason is void. The honest statement is
   that the stall point is unidentified and undiagnosable without a console channel, on any host.
5. **Timing is still not the constraint.** aarch64 attempt A: 6m39s. x86_64 attempt B: 2m20s to a
   hard halt. x86_64 attempt C: 7m47s including a full SB enrollment and reboot cycle. Even pure
   TCG x86 emulation reached the boot menu in well under a minute, and image acquisition (download
   + decompress + splice) cost under 30 s. Any of these fits comfortably inside a CI job.
6. **The next experiment is a Linux/Incus environment, for the console, not for the TPM.**
   `incus launch` supplies UEFI+SB+TPM+console together, and the console is the missing instrument.
   A GitHub-hosted `ubuntu-latest` runner with QEMU + `swtpm` + an OVMF secure varstore is the
   cheap variant to try first; it is now a port of the exact attempt-C command line rather than a
   guess, with the SB varstore requirement already known.

## Cleanup

All qemu processes terminated (`ps -Ao pid,comm | grep -c '[q]emu-system-aar'` → `0`), all `swtpm`
processes terminated (`ps -Ao pid,comm | grep '[s]wtpm' | wc -l` → `0`), the `claude-agent` tmux
server killed, and `/tmp/spike1e2` (overlays, target disks, rescue copy, x86_64 image and its
gzip, extracted UKIs, varstores, TPM state, pcaps, logs) removed. `spikes/splice/out/seeded.img`
digest re-verified unchanged after round 2; `spikes/` was read-only throughout — the x86_64 splice
was performed on a copy in `/tmp`.

---

## DECISION (1.E — boot acceptance gate)

**Status: the CI-automatability question is OPEN — no environment has yet
demonstrated seed consumption. Interim default: documented release checklist
on a Linux host with Incus. Revisit point: Phase 5.2, after one Linux-hosted
diagnostic run.**

What this decision is grounded in (round 2, not round 1's voided premises):

1. Two macOS configurations with TPM attached — and on x86_64 with Secure
   Boot enrolled and accepted — both end at "kernel running, halted, no
   disk/network I/O". The stall point is unidentified and undiagnosable here
   because no shipped IncusOS UKI configures a console and the boot menu
   editor is disabled. The missing instrument is a console channel, which
   `incus launch` on a Linux host provides (together with UEFI+SB+TPM).
2. Requirements a gate host MUST provide, now known concretely: SB-capable
   firmware with a writable setup-mode varstore (enrollment via
   `secure-boot-enroll force` observed working), swtpm-class TPM, and
   out-of-guest oracles — blank-target-disk growth and guest-originated
   pcap frames (source-MAC filtered).
3. Timing is a non-issue: every attempt fits in minutes; image acquisition
   costs under 30 s.

Plan consequences:

- Phase 5.1: everything except the two boot checks (osd seed consumption,
  recovery-path acceptance) is local and stays merge-gating in T3.
- Phase 5.2: ONE time-boxed attempt on a Linux environment — first choice
  `ubuntu-latest` + QEMU + swtpm + OVMF secure varstore as a direct port of
  attempt C's command line, or a Linux box with Incus if available. If seed
  consumption is observed and asserts cleanly via the two oracles + console,
  promote the boot gate to CI; otherwise the release checklist (Phase 6
  how-to, using the exact commands recorded in this doc) is the v1 gate.
- The rescue-media recovery check rides the same Phase 5.2 run; spike 1.B's
  format decision stays provisional until then (independent RR-reader
  evidence already in hand).
