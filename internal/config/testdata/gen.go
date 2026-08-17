//go:build ignore

// Command gen regenerates the SOPS-encrypted fixtures in this directory.
//
//	cd internal/config/testdata && go run gen.go
//
// Requires the sops CLI. The throwaway age.key / age.pub pair must already
// exist.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// main regenerates the encrypted fixtures and exits non-zero on failure.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

// run encrypts the valid and wrong-key fixtures, then writes the
// MAC-mismatch copy.
func run() error {
	pub, err := os.ReadFile("age.pub")
	if err != nil {
		return err
	}
	wrongPub, err := os.ReadFile("wrong-age.pub")
	if err != nil {
		return err
	}
	if err := encrypt(strings.TrimSpace(string(pub)), "valid.yaml", "encrypted.yaml"); err != nil {
		return err
	}
	if err := encrypt(strings.TrimSpace(string(wrongPub)), "valid.yaml", "encrypted-wrong-key.yaml"); err != nil {
		return err
	}
	return tamperMAC("encrypted.yaml", "encrypted-mac-mismatch.yaml")
}

// encrypt writes a SOPS-encrypted copy of inPath to outPath for recipient.
func encrypt(recipient, inPath, outPath string) error {
	cmd := exec.Command("sops", "--age", recipient, "-e", inPath)
	cmd.Env = isolatedEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("sops encrypt %s: %w\n%s", inPath, err, ee.Stderr)
		}
		return fmt.Errorf("sops encrypt %s: %w", inPath, err)
	}
	return os.WriteFile(outPath, out, 0o644)
}

// tamperMAC copies inPath to outPath with one ciphertext byte flipped.
func tamperMAC(inPath, outPath string) error {
	src, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	const needle = "mac: ENC["
	i := strings.Index(string(src), needle)
	if i < 0 {
		return fmt.Errorf("%s: mac field not found", inPath)
	}
	pos := i + len(needle) + 20
	if pos >= len(src) {
		return fmt.Errorf("%s: mac field too short", inPath)
	}
	dst := append([]byte(nil), src...)
	dst[pos] ^= 0x01
	return os.WriteFile(outPath, dst, 0o644)
}

// isolatedEnv returns the process environment without SOPS key or home
// overrides.
func isolatedEnv() []string {
	drop := map[string]struct{}{
		"SOPS_AGE_KEY":      {},
		"SOPS_AGE_KEY_FILE": {},
		"SOPS_AGE_KEY_CMD":  {},
		"HOME":              {},
		"GNUPGHOME":         {},
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, skip := drop[key]; skip {
			continue
		}
		env = append(env, kv)
	}
	home := mustTemp()
	return append(env, "HOME="+home, "GNUPGHOME="+home)
}

// mustTemp creates a throwaway home directory for isolated SOPS runs.
func mustTemp() string {
	dir, err := os.MkdirTemp("", "incusos-builder-sops-gen-*")
	if err != nil {
		panic(err)
	}
	return filepath.Clean(dir)
}
