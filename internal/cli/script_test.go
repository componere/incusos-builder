package cli_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/componere/incusos-builder/internal/cli"
	"github.com/componere/incusos-builder/internal/testfixture"
)

const (
	envMirror     = "INCUSOS_MIRROR"
	envVersion    = "INCUSOS_VERSION"
	envSOPSAgeKey = "SOPS_AGE_KEY"
	envCacheDir   = "INCUSOS_CACHE"
	envEncrypted  = "INCUSOS_ENCRYPTED"
)

// scriptMirror is the shared local-dir fixture used by every script.
type scriptMirror struct {
	// dir is the generated update-server tree.
	dir string
	// sopsAgeKey is the AGE-SECRET-KEY line for encrypted fixtures.
	sopsAgeKey string
	// encrypted is the absolute path of the committed encrypted.yaml.
	encrypted string
}

// TestMain registers the incusos-builder testscript binary.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"incusos-builder": runIncusOSBuilder,
	})
}

// runIncusOSBuilder is the testscript entry point for incusos-builder.
func runIncusOSBuilder() {
	os.Exit(cli.Execute(context.Background(), cli.Options{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}))
}

// TestScript runs testdata/script against the shared local-dir mirror.
func TestScript(t *testing.T) {
	mirror := loadScriptMirror(t)

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/script",
		RequireExplicitExec: true,
		Setup: func(env *testscript.Env) error {
			return setupScript(env, mirror)
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"cmpsha256":   cmdCmpSHA256,
			"execstdout":  cmdExecStdout,
			"extractjson": cmdExtractJSON,
			"exits":       cmdExits,
			"jsonfield":   cmdJSONField,
			"jsonlines":   cmdJSONLines,
		},
	})
}

// loadScriptMirror writes the shared fixture mirror and locates SOPS testdata.
func loadScriptMirror(t *testing.T) scriptMirror {
	t.Helper()
	dir := t.TempDir()
	if _, err := testfixture.Generate(dir); err != nil {
		t.Fatalf("generate shared fixture mirror: %v", err)
	}
	keyPath := filepath.Join("..", "config", "testdata", "age.key")
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("generate shared fixture mirror: %v", err)
	}
	enc, err := filepath.Abs(filepath.Join("..", "config", "testdata", "encrypted.yaml"))
	if err != nil {
		t.Fatalf("generate shared fixture mirror: %v", err)
	}
	return scriptMirror{
		dir:        dir,
		sopsAgeKey: ageSecretLine(key),
		encrypted:  enc,
	}
}

// ageSecretLine returns the AGE-SECRET-KEY line from an age key file.
func ageSecretLine(raw []byte) string {
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			return line
		}
	}
	return strings.TrimSpace(string(raw))
}

// setupScript injects mirror, cache, and SOPS env vars into one script.
func setupScript(env *testscript.Env, mirror scriptMirror) error {
	cache := filepath.Join(env.WorkDir, ".cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	env.Setenv(envMirror, mirror.dir)
	env.Setenv(envVersion, testfixture.Version)
	env.Setenv(envSOPSAgeKey, mirror.sopsAgeKey)
	env.Setenv(envCacheDir, cache)
	env.Setenv(envEncrypted, mirror.encrypted)
	env.Setenv("GNUPGHOME", filepath.Join(env.WorkDir, ".gnupg"))
	env.Values[scriptVarsKey{}] = append([]string(nil), env.Vars...)
	return nil
}

// cmdExits runs a program and asserts its process exit code.
//
//	exits 2 incusos-builder build
func cmdExits(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: exits code program [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		ts.Fatalf("exits: %v", err)
	}
	err = ts.Exec(args[1], args[2:]...)
	got := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("exec %s: %v", args[1], err)
		}
		got = ee.ExitCode()
	}
	if (got == want) == neg {
		if neg {
			ts.Fatalf("exit code unexpectedly %d", got)
		}
		ts.Fatalf("exit code %d, want %d", got, want)
	}
}

// scriptVarsKey stores the script environment on [testscript.Env.Values].
type scriptVarsKey struct{}

// scriptEnviron returns the env slice captured by setupScript.
func scriptEnviron(ts *testscript.TestScript) []string {
	vars, _ := ts.Value(scriptVarsKey{}).([]string)
	return vars
}

// cmdExecStdout runs a program with stdout redirected to a file so large
// artifacts are never captured as testscript's in-memory stdout.
//
//	execstdout streamed.img incusos-builder build -o -
func cmdExecStdout(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: execstdout file program [args...]")
	}
	out, err := os.Create(ts.MkAbs(args[0]))
	if err != nil {
		ts.Fatalf("create %s: %v", args[0], err)
	}
	defer out.Close()

	program, err := lookScriptPath(ts, args[1])
	if err != nil {
		ts.Fatalf("execstdout: %v", err)
	}
	cmd := exec.Command(program, args[2:]...)
	cmd.Dir = ts.MkAbs(".")
	cmd.Env = append(scriptEnviron(ts), "PWD="+cmd.Dir)
	cmd.Stdout = out
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err = cmd.Run()
	if stderr.Len() > 0 {
		ts.Logf("[stderr]\n%s", stderr.String())
	}
	if err == nil && neg {
		ts.Fatalf("unexpected command success")
	}
	if err != nil && !neg {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("exec %s: %v", args[1], err)
		}
		ts.Fatalf("unexpected command failure: %v\n%s", err, stderr.String())
	}
}

// lookScriptPath resolves name on the script PATH, like testscript exec.
func lookScriptPath(ts *testscript.TestScript, name string) (string, error) {
	if filepath.Base(name) != name {
		return name, nil
	}
	for dir := range strings.SplitSeq(ts.Getenv("PATH"), string(os.PathListSeparator)) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, name)
		info, err := os.Stat(cand)
		if err == nil && !info.IsDir() {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s: not found in PATH", name)
}

// cmdCmpSHA256 compares a file's SHA-256 against a 64-hex digest, a
// digest file, or another artifact.
//
//	cmpsha256 <file> <digest-or-file>
func cmdCmpSHA256(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: cmpsha256 file digest-or-file")
	}
	got, err := fileSHA256(ts.MkAbs(args[0]))
	if err != nil {
		ts.Fatalf("hash %s: %v", args[0], err)
	}
	want, err := resolveDigest(ts.MkAbs(args[1]), args[1])
	if err != nil {
		ts.Fatalf("digest %s: %v", args[1], err)
	}
	if (got == want) == neg {
		if neg {
			ts.Fatalf("%s digest unexpectedly matches %s", args[0], want)
		}
		ts.Fatalf("%s digest %s, want %s", args[0], got, want)
	}
}

// resolveDigest accepts a 64-hex digest, a digest file, or an artifact path.
func resolveDigest(abs, raw string) (string, error) {
	info, err := os.Stat(abs)
	if err != nil {
		if isHexDigest(raw) {
			return strings.ToLower(raw), nil
		}
		return "", err
	}
	// Digest files are one 64-hex line. Artifact files are hashed in a stream.
	if info.Size() <= 65 {
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		text := strings.TrimSpace(string(data))
		if isHexDigest(text) {
			return strings.ToLower(text), nil
		}
	}
	return fileSHA256(abs)
}

// isHexDigest reports whether s is a 64-character hex SHA-256.
func isHexDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// fileSHA256 streams path and returns the lowercase hex digest.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cmdExtractJSON writes a dotted JSON path's value to stdout.
//
//	extractjson stdout result.sha256
func cmdExtractJSON(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! extractjson")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: extractjson file path")
	}
	got, err := jsonPathString(readScriptFile(ts, args[0]), args[1])
	if err != nil {
		ts.Fatalf("extractjson %s: %v", args[1], err)
	}
	_, _ = fmt.Fprint(ts.Stdout(), got)
}

// cmdJSONField asserts that a JSON document has a dotted path equal to value.
//
//	jsonfield stdout result.sha256 $DIGEST
func cmdJSONField(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 3 {
		ts.Fatalf("usage: jsonfield file path value")
	}
	got, err := jsonPathString(readScriptFile(ts, args[0]), args[1])
	if err != nil {
		ts.Fatalf("jsonfield %s: %v", args[1], err)
	}
	want := args[2]
	if (got == want) == neg {
		if neg {
			ts.Fatalf("%s %s unexpectedly equals %q", args[0], args[1], want)
		}
		ts.Fatalf("%s %s = %q, want %q", args[0], args[1], got, want)
	}
}

// cmdJSONLines asserts that a document is exactly one JSON value.
//
//	jsonlines stdout
func cmdJSONLines(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: jsonlines file")
	}
	raw := readScriptFile(ts, args[0])
	dec := json.NewDecoder(strings.NewReader(raw))
	var first any
	ok := dec.Decode(&first) == nil
	if ok {
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			ok = false
		}
	}
	if ok == neg {
		if neg {
			ts.Fatalf("%s is unexpectedly a single JSON document", args[0])
		}
		ts.Fatalf("%s is not exactly one JSON document", args[0])
	}
}

// readScriptFile reads a script-relative file as a string.
func readScriptFile(ts *testscript.TestScript, name string) string {
	return ts.ReadFile(name)
}

// jsonPathString returns the dotted JSON path as a string.
func jsonPathString(raw, path string) (string, error) {
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	cur := doc
	for part := range strings.SplitSeq(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s: not an object at %s", path, part)
		}
		next, ok := m[part]
		if !ok {
			return "", fmt.Errorf("%s: missing %s", path, part)
		}
		cur = next
	}
	switch v := cur.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatInt(int64(v), 10), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
