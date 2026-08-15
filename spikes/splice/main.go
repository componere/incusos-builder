package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/klauspost/pgzip"
	"github.com/lxc/incus-os/incus-osd/api"
	apicustomizer "github.com/lxc/incus-os/incus-osd/api/customizer"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"go.yaml.in/yaml/v4"
)

const (
	bufSize           = 4 * 1024 * 1024
	customizerSeedOff = int64(2148532224) // image-customizer hardcoded splice offset
	defaultVersion    = "202608102114"
	defaultArch       = "aarch64"
	defaultServer     = "https://images.linuxcontainers.org/os"
	defaultGZName     = "IncusOS_202608102114.img.gz"
	defaultSHA256     = "4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "gpt":
		err = cmdGPT(os.Args[2:])
	case "seed":
		err = cmdSeed(os.Args[2:])
	case "splice":
		err = cmdSplice(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "all":
		err = cmdAll(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  splice gpt    -gz FILE -img FILE
  splice seed   -out FILE [-ks]
  splice splice -in IMG -tar TAR -out IMG -offset N
  splice verify -in IMG -src IMG -offset N -expect TAR
  splice all    [-dir out]
`)
}

func cmdGPT(args []string) error {
	fs := flag.NewFlagSet("gpt", flag.ExitOnError)
	gzPath := fs.String("gz", "", "compressed image")
	imgPath := fs.String("img", "", "decompressed image output")
	skipDecomp := fs.Bool("skip-decompress", false, "parse existing -img")
	_ = fs.Parse(args)
	if *imgPath == "" {
		return fmt.Errorf("need -img")
	}
	buf := make([]byte, bufSize)
	if !*skipDecomp {
		if *gzPath == "" {
			return fmt.Errorf("need -gz (or -skip-decompress)")
		}
		t0 := time.Now()
		if err := decompressPGZip(*gzPath, *imgPath, buf); err != nil {
			return err
		}
		fmt.Printf("pgzip decompress: %s -> %s in %s\n", *gzPath, *imgPath, time.Since(t0).Round(time.Millisecond))
	}
	st, err := os.Stat(*imgPath)
	if err != nil {
		return err
	}
	fmt.Printf("image size: %d bytes\n", st.Size())

	t0 := time.Now()
	hand, secsz, err := handParseGPT(*imgPath)
	if err != nil {
		return fmt.Errorf("hand parse: %w", err)
	}
	handDur := time.Since(t0)
	fmt.Printf("\n=== hand GPT (sector %d, parse %s) ===\n", secsz, handDur.Round(time.Microsecond))
	printParts("hand", hand)

	t1 := time.Now()
	diskfsParts, lss, err := diskfsParseGPT(*imgPath)
	if err != nil {
		return fmt.Errorf("diskfs parse: %w", err)
	}
	diskfsDur := time.Since(t1)
	fmt.Printf("\n=== go-diskfs GPT (logical sector %d, parse %s) ===\n", lss, diskfsDur.Round(time.Microsecond))
	printParts("diskfs", diskfsParts)

	fmt.Printf("\n=== GPT parser diff ===\n")
	if err := diffParts(hand, diskfsParts); err != nil {
		return err
	}
	fmt.Println("AGREE: both parsers report the same name/start-byte/length for every populated entry")

	seed, ok := findPart(hand, "seed-data")
	if !ok {
		return fmt.Errorf("ASSERT FAIL: no seed-data partition")
	}
	fmt.Printf("\nseed-data start_byte=%d length=%d (%d MiB)\n", seed.StartByte, seed.Length, seed.Length/1024/1024)
	fmt.Printf("customizer hardcoded offset: %d\n", customizerSeedOff)
	if int64(seed.StartByte) == customizerSeedOff {
		fmt.Println("OFFSET MATCH: seed-data start == 2148532224 (no drift)")
	} else {
		drift := int64(seed.StartByte) - customizerSeedOff
		fmt.Printf("OFFSET DRIFT: seed-data start - 2148532224 = %d\n", drift)
	}
	return nil
}

func cmdSeed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	out := fs.String("out", "", "output tar path")
	ks := fs.Bool("ks", false, "also write kernel.yaml and security.yaml")
	cmpDir := fs.String("cmp-dir", "", "if set, write writeSeed clone + ours and cmp")
	_ = fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("need -out")
	}
	entries := minimalInstallNetwork()
	if *ks {
		entries = append(entries, kernelEntry(), securityEntry())
	}

	ours, err := renderEntries(entries)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, ours, 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d bytes) ks=%v\n", *out, len(ours), *ks)
	dumpTarList(ours)

	if *ks {
		fmt.Println("kernel/security: CLI-exclusive filenames (writeSeed does not emit these)")
		return nil
	}

	// Byte-compat vs a verbatim copy of image-customizer writeSeed.
	seeds := apicustomizer.ImagesPostSeeds{
		Install: sampleInstall(),
		Network: sampleNetwork(),
	}
	var buf bytes.Buffer
	n, err := writeSeed(&buf, seeds)
	if err != nil {
		return err
	}
	upstream := buf.Bytes()
	fmt.Printf("writeSeed clone size=%d (counter=%d)\n", len(upstream), n)
	if *cmpDir != "" {
		if err := os.MkdirAll(*cmpDir, 0o755); err != nil {
			return err
		}
		p1 := filepath.Join(*cmpDir, "writeseed.tar")
		p2 := filepath.Join(*cmpDir, "ours.tar")
		if err := os.WriteFile(p1, upstream, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(p2, ours, 0o600); err != nil {
			return err
		}
		fmt.Printf("cmp paths: %s %s\n", p1, p2)
	}
	if bytes.Equal(ours, upstream) {
		fmt.Println("CMP: ours == writeSeed clone (byte-identical)")
	} else {
		fmt.Printf("CMP FAIL: ours=%d writeSeed=%d\n", len(ours), len(upstream))
		hexDumpDiff(ours, upstream)
		return fmt.Errorf("tar bytes differ from writeSeed")
	}
	return nil
}

func cmdSplice(args []string) error {
	fs := flag.NewFlagSet("splice", flag.ExitOnError)
	inPath := fs.String("in", "", "source image")
	tarPath := fs.String("tar", "", "seed tar")
	outPath := fs.String("out", "", "spliced image")
	offset := fs.Int64("offset", customizerSeedOff, "seed-data start byte")
	_ = fs.Parse(args)
	if *inPath == "" || *tarPath == "" || *outPath == "" {
		return fmt.Errorf("need -in -tar -out")
	}
	tarBytes, err := os.ReadFile(*tarPath)
	if err != nil {
		return err
	}
	t0 := time.Now()
	if err := spliceImage(*inPath, *outPath, *offset, tarBytes); err != nil {
		return err
	}
	st, err := os.Stat(*outPath)
	if err != nil {
		return err
	}
	inSt, _ := os.Stat(*inPath)
	fmt.Printf("splice: in=%s out=%s offset=%d tar=%d out_size=%d in_size=%d wall=%s\n",
		*inPath, *outPath, *offset, len(tarBytes), st.Size(), inSt.Size(), time.Since(t0).Round(time.Millisecond))
	if inSt != nil && st.Size() != inSt.Size() {
		return fmt.Errorf("spliced size %d != source size %d", st.Size(), inSt.Size())
	}
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	inPath := fs.String("in", "", "spliced image")
	srcPath := fs.String("src", "", "source image (untouched-region digests)")
	tarPath := fs.String("expect", "", "expected tar")
	offset := fs.Int64("offset", customizerSeedOff, "seed-data start byte")
	_ = fs.Parse(args)
	if *inPath == "" || *tarPath == "" || *srcPath == "" {
		return fmt.Errorf("need -in -src -expect")
	}
	expect, err := os.ReadFile(*tarPath)
	if err != nil {
		return err
	}
	t0 := time.Now()
	got, files, err := readTarAt(*inPath, *offset, int64(len(expect)))
	if err != nil {
		return err
	}
	fmt.Printf("verify: read %d bytes at offset %d in %s\n", len(got), *offset, time.Since(t0).Round(time.Millisecond))
	if bytes.Equal(got, expect) {
		fmt.Println("TAR BYTES: spliced region prefix == input tar (exact)")
	} else {
		fmt.Printf("TAR BYTES MISMATCH got=%d expect=%d\n", len(got), len(expect))
		hexDumpDiff(got, expect)
		return fmt.Errorf("spliced tar bytes differ")
	}
	fmt.Println("untar + strict yaml.WithKnownFields():")
	for _, f := range files {
		fmt.Printf("  %s mode=%04o size=%d\n", f.name, f.mode, len(f.body))
		if err := strictDecode(f.name, f.body); err != nil {
			return fmt.Errorf("strict decode %s: %w", f.name, err)
		}
		fmt.Printf("    strict-decode OK into upstream type\n")
	}
	tReg := time.Now()
	if err := verifyUntouchedRegions(*srcPath, *inPath, *offset, int64(len(expect))); err != nil {
		return err
	}
	fmt.Printf("untouched-region digests: %s\n", time.Since(tReg).Round(time.Millisecond))
	fmt.Println("round-trip: PASS")
	return nil
}

func cmdAll(args []string) error {
	fs := flag.NewFlagSet("all", flag.ExitOnError)
	dir := fs.String("dir", "out", "artifact dir")
	_ = fs.Parse(args)
	gz := filepath.Join(*dir, defaultGZName)
	img := filepath.Join(*dir, strings.TrimSuffix(defaultGZName, ".gz"))
	seedTar := filepath.Join(*dir, "seed-install-network.tar")
	seedKS := filepath.Join(*dir, "seed-ks.tar")
	cmpDir := filepath.Join(*dir, "cmp")
	seeded := filepath.Join(*dir, "seeded.img")
	seededKS := filepath.Join(*dir, "seeded-ks.img")
	manifest := filepath.Join(*dir, "MANIFEST.txt")

	fmt.Println("######## gpt ########")
	tGPT := time.Now()
	if err := cmdGPT([]string{"-gz", gz, "-img", img}); err != nil {
		return err
	}
	gptWall := time.Since(tGPT)
	fmt.Printf("TIMING decompress+GPT: %s\n", gptWall.Round(time.Millisecond))

	fmt.Println("\n######## seed (install+network) ########")
	tSeed := time.Now()
	if err := cmdSeed([]string{"-out", seedTar, "-cmp-dir", cmpDir}); err != nil {
		return err
	}
	fmt.Printf("TIMING seed: %s\n", time.Since(tSeed).Round(time.Millisecond))

	fmt.Println("\n######## splice ########")
	tSplice := time.Now()
	if err := cmdSplice(
		[]string{"-in", img, "-tar", seedTar, "-out", seeded, "-offset", fmt.Sprintf("%d", customizerSeedOff)},
	); err != nil {
		return err
	}
	spliceWall := time.Since(tSplice)
	fmt.Printf("TIMING splice: %s\n", spliceWall.Round(time.Millisecond))

	fmt.Println("\n######## verify ########")
	tVer := time.Now()
	if err := cmdVerify(
		[]string{"-in", seeded, "-src", img, "-expect", seedTar, "-offset", fmt.Sprintf("%d", customizerSeedOff)},
	); err != nil {
		return err
	}
	verWall := time.Since(tVer)
	fmt.Printf("TIMING verify: %s\n", verWall.Round(time.Millisecond))

	fmt.Println("\n######## seed+splice+verify kernel+security ########")
	if err := cmdSeed([]string{"-out", seedKS, "-ks"}); err != nil {
		return err
	}
	if err := cmdSplice(
		[]string{"-in", img, "-tar", seedKS, "-out", seededKS, "-offset", fmt.Sprintf("%d", customizerSeedOff)},
	); err != nil {
		return err
	}
	if err := cmdVerify(
		[]string{"-in", seededKS, "-src", img, "-expect", seedKS, "-offset", fmt.Sprintf("%d", customizerSeedOff)},
	); err != nil {
		return err
	}

	sum, err := sha256File(gz)
	if err != nil {
		return err
	}
	st, err := os.Stat(gz)
	if err != nil {
		return err
	}
	sst, err := os.Stat(seeded)
	if err != nil {
		return err
	}
	ssum, err := sha256File(seeded)
	if err != nil {
		return err
	}
	url := defaultServer + "/" + defaultVersion + "/" + defaultArch + "/" + defaultGZName
	man := fmt.Sprintf(`version: %s
arch: %s
filename: %s
url: %s
sha256_gz: %s
size_gz: %d
sha256_seeded: %s
size_seeded: %d
seed-data offset: %d
`, defaultVersion, defaultArch, defaultArch+"/"+defaultGZName, url, sum, st.Size(), ssum, sst.Size(), customizerSeedOff)
	if err := os.WriteFile(manifest, []byte(man), 0o644); err != nil {
		return err
	}
	fmt.Printf("\n######## MANIFEST ########\n%s", man)
	fmt.Printf("TIMING_SUMMARY gpt(decompress+parse)=%s splice=%s verify=%s\n",
		gptWall.Round(time.Millisecond), spliceWall.Round(time.Millisecond), verWall.Round(time.Millisecond))
	return nil
}

type partInfo struct {
	Name      string
	FirstLBA  uint64
	LastLBA   uint64
	StartByte uint64
	Length    uint64
}

func printParts(label string, parts []partInfo) {
	fmt.Printf("%s: %d populated partitions\n", label, len(parts))
	fmt.Printf("%-3s %-34s %12s %12s %14s %14s\n", "#", "name", "first_lba", "last_lba", "start_byte", "length")
	for i, p := range parts {
		fmt.Printf("%-3d %-34s %12d %12d %14d %14d\n", i, p.Name, p.FirstLBA, p.LastLBA, p.StartByte, p.Length)
	}
}

func findPart(parts []partInfo, name string) (partInfo, bool) {
	for _, p := range parts {
		if p.Name == name {
			return p, true
		}
	}
	return partInfo{}, false
}

func diffParts(a, b []partInfo) error {
	if len(a) != len(b) {
		return fmt.Errorf("count mismatch hand=%d diskfs=%d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].StartByte != b[i].StartByte || a[i].Length != b[i].Length {
			return fmt.Errorf("entry %d differ hand=%+v diskfs=%+v", i, a[i], b[i])
		}
	}
	return nil
}

func handParseGPT(path string) ([]partInfo, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	// GPT header is at LBA1. Probe the logical sector sizes IncusOS actually
	// ships: 512 (raw), 2048 (ISO via losetup --sector-size 2048), 4096.
	hdr := make([]byte, 92)
	secsz := 0
	for _, sz := range []int{512, 2048, 4096} {
		if _, err := f.ReadAt(hdr, int64(sz)); err != nil {
			continue
		}
		if string(hdr[:8]) == "EFI PART" {
			secsz = sz
			break
		}
	}
	if secsz == 0 {
		return nil, 0, fmt.Errorf("no EFI PART signature at LBA1 (512, 2048, or 4096)")
	}
	partLBA := binary.LittleEndian.Uint64(hdr[72:80])
	nparts := binary.LittleEndian.Uint32(hdr[80:84])
	esize := binary.LittleEndian.Uint32(hdr[84:88])
	if esize < 128 || nparts == 0 || nparts > 4096 {
		return nil, 0, fmt.Errorf("implausible GPT: nparts=%d esize=%d", nparts, esize)
	}
	entries := make([]byte, int(nparts)*int(esize))
	if _, err := f.ReadAt(entries, int64(partLBA)*int64(secsz)); err != nil {
		return nil, 0, err
	}
	var out []partInfo
	for i := 0; i < int(nparts); i++ {
		e := entries[i*int(esize) : (i+1)*int(esize)]
		zero := true
		for _, b := range e[:16] {
			if b != 0 {
				zero = false
				break
			}
		}
		if zero {
			continue
		}
		first := binary.LittleEndian.Uint64(e[32:40])
		last := binary.LittleEndian.Uint64(e[40:48])
		name := gptName(e[56 : 56+72])
		out = append(out, partInfo{
			Name:      name,
			FirstLBA:  first,
			LastLBA:   last,
			StartByte: first * uint64(secsz),
			Length:    (last - first + 1) * uint64(secsz),
		})
	}
	return out, secsz, nil
}

func gptName(b []byte) string {
	u := make([]uint16, 36)
	for i := 0; i < 36; i++ {
		u[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	n := 36
	for i, c := range u {
		if c == 0 {
			n = i
			break
		}
	}
	return string(utf16.Decode(u[:n]))
}

func diskfsParseGPT(path string) ([]partInfo, int, error) {
	d, err := diskfs.Open(path, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return nil, 0, err
	}
	table, err := d.GetPartitionTable()
	if err != nil {
		return nil, 0, err
	}
	gt, ok := table.(*gpt.Table)
	if !ok {
		return nil, 0, fmt.Errorf("not a GPT table: %T", table)
	}
	secsz := gt.LogicalSectorSize
	if secsz == 0 {
		secsz = 512
	}
	var out []partInfo
	for _, p := range gt.Partitions {
		if p == nil || p.Type == gpt.Unused || (p.Start == 0 && p.End == 0 && p.Name == "") {
			continue
		}
		length := p.Size
		if length == 0 {
			length = (p.End - p.Start + 1) * uint64(secsz)
		}
		out = append(out, partInfo{
			Name:      p.Name,
			FirstLBA:  p.Start,
			LastLBA:   p.End,
			StartByte: p.Start * uint64(secsz),
			Length:    length,
		})
	}
	return out, secsz, nil
}

func decompressPGZip(gzPath, imgPath string, buf []byte) error {
	in, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := pgzip.NewReader(in)
	if err != nil {
		return err
	}
	defer zr.Close()
	out, err := os.Create(imgPath)
	if err != nil {
		return err
	}
	defer out.Close()
	for {
		n, err := zr.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type seedFile struct {
	Name string
	Body []byte
}

func sampleInstall() *apiseed.Install {
	return &apiseed.Install{
		Version:      "1",
		ForceInstall: true,
		ForceReboot:  true,
	}
}

func sampleNetwork() *apiseed.Network {
	return &apiseed.Network{Version: "1"}
}

func sampleKernel() *apiseed.Kernel {
	return &apiseed.Kernel{
		Version: "1",
		Console: []api.SystemKernelConfigConsole{{Device: "/dev/ttyS0", BaudRate: 115200}},
	}
}

func sampleSecurity() *apiseed.Security {
	return &apiseed.Security{
		Version: "1",
		SystemSecurityConfig: api.SystemSecurityConfig{
			CustomCACerts: []string{"spike-ca"},
		},
	}
}

func dumpYAML(v any) ([]byte, error) {
	return yaml.Dump(v, yaml.WithV2Defaults())
}

func minimalInstallNetwork() []seedFile {
	ib, err := dumpYAML(sampleInstall())
	must(err)
	nb, err := dumpYAML(sampleNetwork())
	must(err)
	return []seedFile{{Name: "install.yaml", Body: ib}, {Name: "network.yaml", Body: nb}}
}

func kernelEntry() seedFile {
	b, err := dumpYAML(sampleKernel())
	must(err)
	return seedFile{Name: "kernel.yaml", Body: b}
}

func securityEntry() seedFile {
	b, err := dumpYAML(sampleSecurity())
	must(err)
	return seedFile{Name: "security.yaml", Body: b}
}

func renderEntries(files []seedFile) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0o600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(file.Body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeSeed is a verbatim copy of incus-osd/cmd/image-customizer writeSeed.
func writeSeed(writer io.Writer, seeds apicustomizer.ImagesPostSeeds) (int, error) {
	archiveContents := [][]string{}

	if seeds.Applications != nil {
		yamlContents, err := yaml.Dump(seeds.Applications, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"applications.yaml", string(yamlContents)})
	}
	if seeds.Incus != nil {
		yamlContents, err := yaml.Dump(seeds.Incus, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"incus.yaml", string(yamlContents)})
	}
	if seeds.OperationsCenter != nil {
		yamlContents, err := yaml.Dump(seeds.OperationsCenter, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"operations-center.yaml", string(yamlContents)})
	}
	if seeds.MigrationManager != nil {
		yamlContents, err := yaml.Dump(seeds.MigrationManager, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"migration-manager.yaml", string(yamlContents)})
	}
	if seeds.Install != nil {
		yamlContents, err := yaml.Dump(seeds.Install, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"install.yaml", string(yamlContents)})
	}
	if seeds.Network != nil {
		yamlContents, err := yaml.Dump(seeds.Network, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"network.yaml", string(yamlContents)})
	}
	if seeds.Provider != nil {
		yamlContents, err := yaml.Dump(seeds.Provider, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"provider.yaml", string(yamlContents)})
	}
	if seeds.Services != nil {
		yamlContents, err := yaml.Dump(seeds.Services, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"services.yaml", string(yamlContents)})
	}
	if seeds.Update != nil {
		yamlContents, err := yaml.Dump(seeds.Update, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}
		archiveContents = append(archiveContents, []string{"update.yaml", string(yamlContents)})
	}

	wc := &writeCounter{}
	tw := tar.NewWriter(io.MultiWriter(wc, writer))
	for _, file := range archiveContents {
		hdr := &tar.Header{
			Name: file[0],
			Mode: 0o600,
			Size: int64(len(file[1])),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return -1, err
		}
		if _, err := tw.Write([]byte(file[1])); err != nil {
			return -1, err
		}
	}
	if err := tw.Close(); err != nil {
		return -1, err
	}
	return wc.size, nil
}

type writeCounter struct{ size int }

func (wc *writeCounter) Write(buf []byte) (int, error) {
	size := len(buf)
	wc.size += size
	return size, nil
}

func spliceImage(inPath, outPath string, offset int64, tarBytes []byte) error {
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	buf := make([]byte, bufSize)
	if _, err := copyN(out, in, offset, buf); err != nil {
		return fmt.Errorf("copy prefix: %w", err)
	}
	if _, err := out.Write(tarBytes); err != nil {
		return err
	}
	if _, err := copyN(io.Discard, in, int64(len(tarBytes)), buf); err != nil {
		return fmt.Errorf("skip overwritten: %w", err)
	}
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func copyN(dst io.Writer, src io.Reader, n int64, buf []byte) (int64, error) {
	var copied int64
	for copied < n {
		chunk := int64(len(buf))
		if remain := n - copied; remain < chunk {
			chunk = remain
		}
		nr, err := io.ReadFull(src, buf[:chunk])
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			copied += int64(nw)
			if werr != nil {
				return copied, werr
			}
		}
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return copied, io.ErrUnexpectedEOF
			}
			return copied, err
		}
	}
	return copied, nil
}

type tarFile struct {
	name string
	mode int64
	body []byte
}

func readTarAt(img string, offset, tarLen int64) ([]byte, []tarFile, error) {
	f, err := os.Open(img)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	raw := make([]byte, tarLen)
	if _, err := f.ReadAt(raw, offset); err != nil {
		return nil, nil, err
	}
	tr := tar.NewReader(bytes.NewReader(raw))
	var files []tarFile
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, tarFile{name: hdr.Name, mode: hdr.Mode, body: body})
	}
	return raw, files, nil
}

func strictDecode(name string, body []byte) error {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	loader, err := yaml.NewLoader(bytes.NewReader(body), yaml.WithKnownFields())
	if err != nil {
		return err
	}
	var target any
	switch stem {
	case "install":
		target = &apiseed.Install{}
	case "network":
		target = &apiseed.Network{}
	case "kernel":
		target = &apiseed.Kernel{}
	case "security":
		target = &apiseed.Security{}
	case "applications":
		target = &apiseed.Applications{}
	case "incus":
		target = &apiseed.Incus{}
	case "provider":
		target = &apiseed.Provider{}
	case "services":
		target = &apiseed.Services{}
	case "update":
		target = &apiseed.Update{}
	case "operations-center":
		target = &apiseed.OperationsCenter{}
	case "migration-manager":
		target = &apiseed.MigrationManager{}
	default:
		return fmt.Errorf("unknown seed section %q", name)
	}
	if err := loader.Load(target); err != nil {
		return err
	}
	// Re-dump vs original YAML is not required to be identical (alias/zero-value
	// presentation). DeepEqual against a freshly decoded copy of the same bytes is.
	loader2, err := yaml.NewLoader(bytes.NewReader(body), yaml.WithKnownFields())
	if err != nil {
		return err
	}
	target2 := reflect.New(reflect.TypeOf(target).Elem()).Interface()
	if err := loader2.Load(target2); err != nil {
		return err
	}
	if !reflect.DeepEqual(target, target2) {
		return fmt.Errorf("non-deterministic decode")
	}
	jb, _ := json.Marshal(target)
	fmt.Printf("    decoded %s\n", jb)
	return nil
}

func dumpTarList(b []byte) {
	tr := tar.NewReader(bytes.NewReader(b))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Printf("tar list err: %v\n", err)
			return
		}
		fmt.Printf("  tar entry name=%q mode=%04o size=%d format=%v typeflag=%q\n",
			hdr.Name, hdr.Mode, hdr.Size, hdr.Format, string(hdr.Typeflag))
	}
}

func hexDumpDiff(a, b []byte) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := i - 16
			if lo < 0 {
				lo = 0
			}
			hi := i + 16
			if hi > n {
				hi = n
			}
			fmt.Printf("first differ at byte %d\n a: %x\n b: %x\n", i, a[lo:hi], b[lo:hi])
			return
		}
	}
	fmt.Printf("common prefix %d; lens %d vs %d\n", n, len(a), len(b))
}

func verifyUntouchedRegions(srcPath, outPath string, offset, tarLen int64) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Open(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	srcSt, err := src.Stat()
	if err != nil {
		return err
	}
	outSt, err := out.Stat()
	if err != nil {
		return err
	}
	if srcSt.Size() != outSt.Size() {
		return fmt.Errorf("size mismatch src=%d out=%d", srcSt.Size(), outSt.Size())
	}
	suffixStart := offset + tarLen
	if suffixStart > srcSt.Size() {
		return fmt.Errorf("offset+tarLen %d exceeds image size %d", suffixStart, srcSt.Size())
	}
	suffixLen := srcSt.Size() - suffixStart
	buf := make([]byte, bufSize)

	srcPrefix, err := digestRange(src, 0, offset, buf)
	if err != nil {
		return fmt.Errorf("src prefix: %w", err)
	}
	outPrefix, err := digestRange(out, 0, offset, buf)
	if err != nil {
		return fmt.Errorf("out prefix: %w", err)
	}
	srcSuffix, err := digestRange(src, suffixStart, suffixLen, buf)
	if err != nil {
		return fmt.Errorf("src suffix: %w", err)
	}
	outSuffix, err := digestRange(out, suffixStart, suffixLen, buf)
	if err != nil {
		return fmt.Errorf("out suffix: %w", err)
	}

	fmt.Printf("untouched regions (sha256, streamed, reused %d-byte buffer):\n", bufSize)
	fmt.Printf("  [0, %d) len=%d\n", offset, offset)
	fmt.Printf("    src=%s\n", srcPrefix)
	fmt.Printf("    out=%s\n", outPrefix)
	if srcPrefix != outPrefix {
		return fmt.Errorf("prefix region mismatch")
	}
	fmt.Println("    PREFIX: identical")
	fmt.Printf("  [%d, EOF) len=%d\n", suffixStart, suffixLen)
	fmt.Printf("    src=%s\n", srcSuffix)
	fmt.Printf("    out=%s\n", outSuffix)
	if srcSuffix != outSuffix {
		return fmt.Errorf("suffix region mismatch")
	}
	fmt.Println("    SUFFIX: identical")
	return nil
}

func digestRange(r io.ReaderAt, start, n int64, buf []byte) (string, error) {
	h := sha256.New()
	copied, err := io.CopyBuffer(h, io.NewSectionReader(r, start, n), buf)
	if err != nil {
		return "", err
	}
	if copied != n {
		return "", fmt.Errorf("short digest read: got %d want %d", copied, n)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, bufSize)
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
