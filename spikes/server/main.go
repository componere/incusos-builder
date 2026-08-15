// Spike 1.C: live update-server layout + metadata sizes.
//
// Throwaway scratch against https://images.linuxcontainers.org/os.
// Fetches metadata only (index + per-version update.json/update.sjson + HEAD
// of one small asset). Does not download full images.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/smallstep/pkcs7"
)

const (
	serverURL   = "https://images.linuxcontainers.org/os"
	httpTimeout = 30 * time.Second
	// Planned ReleaseMetadata cap under test; used as a LimitReader on metadata GETs.
	metadataCap = 8 << 20
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	client := &http.Client{Timeout: httpTimeout}

	fmt.Println("=== Spike 1.C — live update-server layout + metadata sizes ===")
	fmt.Println("server:", serverURL)
	fmt.Println()

	indexBody, indexLen, err := getCapped(client, serverURL+"/index.json")
	if err != nil {
		return fmt.Errorf("GET /index.json: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(indexBody))
	dec.DisallowUnknownFields()
	var index apiimages.Index
	if err := dec.Decode(&index); err != nil {
		return fmt.Errorf("strict-decode index.json: %w", err)
	}
	if err := expectEOF(dec, "index.json"); err != nil {
		return err
	}

	fmt.Println("--- 1. index.json structure ---")
	fmt.Printf("bytes: %d\n", indexLen)
	fmt.Printf("format: %q\n", index.Format)
	fmt.Printf("updates: %d\n", len(index.Updates))

	urlOverrides := 0
	channels := map[string]int{}
	versions := make([]string, 0, len(index.Updates))

	for i, u := range index.Updates {
		versions = append(versions, u.Version)
		if u.URL != "" {
			urlOverrides++
		}
		for _, ch := range u.Channels {
			channels[ch]++
		}
		fmt.Printf("\nupdate[%d]:\n", i)
		fmt.Printf("  format=%q version=%q origin=%q severity=%q published_at=%s\n",
			u.Format, u.Version, u.Origin, u.Severity, u.PublishedAt.UTC().Format(time.RFC3339Nano))
		fmt.Printf("  channels=%v\n", u.Channels)
		fmt.Printf("  url (UpdateFull.URL json:\"url,omitempty\")=%q (present=%v)\n", u.URL, u.URL != "")
		fmt.Printf("  files=%d\n", len(u.Files))

		archs := map[string]int{}
		types := map[string]int{}
		comps := map[string]int{}
		var smallest apiimages.UpdateFile
		for j, f := range u.Files {
			arch := string(f.Architecture)
			if arch == "" {
				arch = "<empty>"
			}
			archs[arch]++
			types[string(f.Type)]++
			comps[string(f.Component)]++
			if j == 0 || f.Size < smallest.Size {
				smallest = f
			}
		}
		fmt.Printf("  architectures: %s\n", formatCounts(archs))
		fmt.Printf("  types: %s\n", formatCounts(types))
		fmt.Printf("  components: %s\n", formatCounts(comps))
		fmt.Printf("  smallest file: %s size=%d type=%s sha256=%s\n",
			smallest.Filename, smallest.Size, smallest.Type, smallest.Sha256)
	}

	fmt.Println("\nper-file JSON field names: architecture, component, filename, sha256, size, type")
	fmt.Printf("updates carrying UpdateFull.URL: %d / %d\n", urlOverrides, len(index.Updates))
	fmt.Printf("channels observed (update membership count): %s\n", formatCounts(channels))
	fmt.Printf("versions listed (index order): %s\n", strings.Join(versions, ", "))

	if len(index.Updates) == 0 {
		return fmt.Errorf("index.json has no updates")
	}
	newest := index.Updates[0]
	fmt.Printf("\nnewest version (first index entry): %s\n", newest.Version)

	var probe apiimages.UpdateFile
	for _, f := range newest.Files {
		if probe.Filename == "" || f.Size < probe.Size {
			probe = f
		}
	}
	if probe.Filename == "" {
		return fmt.Errorf("newest update has no files")
	}

	assetURL, err := url.JoinPath(serverURL, newest.Version, probe.Filename)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("--- 2. asset URL shape ---")
	fmt.Printf("constructed: %s\n", assetURL)
	fmt.Printf("index Size: %d\n", probe.Size)

	headStatus, headLen, err := head(client, assetURL)
	if err != nil {
		return fmt.Errorf("HEAD asset: %w", err)
	}
	fmt.Printf("HEAD status: %d  Content-Length: %s  matches index Size: %v\n",
		headStatus, headLen, headLen == fmt.Sprintf("%d", probe.Size))

	rangeStatus, rangeCL, rangeCR, rangeN, err := rangeGET(client, assetURL, "bytes=0-15")
	if err != nil {
		return fmt.Errorf("Range GET asset: %w", err)
	}
	fmt.Printf("Range GET bytes=0-15 status: %d  Content-Length: %s  Content-Range: %s  body bytes: %d\n",
		rangeStatus, rangeCL, rangeCR, rangeN)

	fmt.Println()
	fmt.Println("--- 3. per-version metadata ---")
	updateJSONURL, err := url.JoinPath(serverURL, newest.Version, "update.json")
	if err != nil {
		return err
	}
	updateSJSONURL, err := url.JoinPath(serverURL, newest.Version, "update.sjson")
	if err != nil {
		return err
	}

	updateJSON, updateJSONLen, err := getCapped(client, updateJSONURL)
	if err != nil {
		return fmt.Errorf("GET update.json: %w", err)
	}
	updateSJSON, updateSJSONLen, err := getCapped(client, updateSJSONURL)
	if err != nil {
		return fmt.Errorf("GET update.sjson: %w", err)
	}
	fmt.Printf("GET %s  bytes=%d\n", updateJSONURL, updateJSONLen)
	fmt.Printf("GET %s  bytes=%d\n", updateSJSONURL, updateSJSONLen)

	indexSJSONURL := serverURL + "/index.sjson"
	_, indexSJSONLen, err := getCapped(client, indexSJSONURL)
	if err != nil {
		return fmt.Errorf("GET index.sjson: %w", err)
	}
	fmt.Printf("GET %s  bytes=%d (supporting; not the ReleaseMetadata cap target)\n", indexSJSONURL, indexSJSONLen)

	fmt.Println()
	fmt.Println("--- 4. parse update.sjson as multipart/signed ---")
	fmt.Println("approach: net/mail.ReadMessage for RFC 822 headers, mime.ParseMediaType,")
	fmt.Println("then mime/multipart.NewReader on the body using the boundary param.")

	payload, sigDER, mimeInfo, err := parseSJSON(updateSJSON)
	if err != nil {
		return fmt.Errorf("parse sjson: %w", err)
	}
	fmt.Printf("Content-Type: %s\n", mimeInfo.contentType)
	fmt.Printf("protocol: %s\n", mimeInfo.protocol)
	fmt.Printf("micalg (digest algorithm): %s\n", mimeInfo.micalg)
	fmt.Printf("boundary: %s\n", mimeInfo.boundary)
	fmt.Printf("parts: %d  payload bytes: %d  pkcs7 DER bytes: %d\n",
		mimeInfo.parts, len(payload), len(sigDER))

	p7, err := pkcs7.Parse(sigDER)
	if err != nil {
		return fmt.Errorf("pkcs7.Parse: %w", err)
	}
	fmt.Printf("PKCS#7 certificates present: %d\n", len(p7.Certificates))
	for i, cert := range p7.Certificates {
		fmt.Printf("  cert[%d] subject=%q issuer=%q sigalg=%s notAfter=%s\n",
			i, cert.Subject.String(), cert.Issuer.String(),
			cert.SignatureAlgorithm.String(), cert.NotAfter.UTC().Format(time.RFC3339))
	}
	if signer := p7.GetOnlySigner(); signer != nil {
		fmt.Printf("PKCS#7 GetOnlySigner: subject=%q serial=%s\n",
			signer.Subject.String(), signer.SerialNumber.Text(16))
	} else {
		fmt.Println("PKCS#7 GetOnlySigner: none")
	}
	fmt.Printf("PKCS#7 attached content bytes: %d (detached signature => 0)\n", len(p7.Content))

	payloadDec := json.NewDecoder(bytes.NewReader(payload))
	payloadDec.DisallowUnknownFields()
	var fromSJSON apiimages.Update
	if err := payloadDec.Decode(&fromSJSON); err != nil {
		return fmt.Errorf("strict-decode sjson payload as apiimages.Update: %w", err)
	}
	if err := expectEOF(payloadDec, "sjson payload"); err != nil {
		return err
	}
	fmt.Println("strict-decode sjson payload as apiimages.Update: OK")

	jsonDec := json.NewDecoder(bytes.NewReader(updateJSON))
	jsonDec.DisallowUnknownFields()
	var fromJSON apiimages.Update
	if err := jsonDec.Decode(&fromJSON); err != nil {
		return fmt.Errorf("strict-decode update.json: %w", err)
	}
	if err := expectEOF(jsonDec, "update.json"); err != nil {
		return err
	}

	fmt.Printf("sjson payload version: %s\n", fromSJSON.Version)
	fmt.Printf("update.json version:   %s\n", fromJSON.Version)
	fmt.Printf("version match: %v\n", fromSJSON.Version == fromJSON.Version)
	fmt.Printf("file-count match: sjson=%d json=%d equal=%v\n",
		len(fromSJSON.Files), len(fromJSON.Files), len(fromSJSON.Files) == len(fromJSON.Files))
	fmt.Printf("sjson channels: %v\n", fromSJSON.Channels)

	emptyArch := 0
	for _, f := range fromSJSON.Files {
		if f.Architecture == "" {
			emptyArch++
			fmt.Printf("empty-architecture file: filename=%s type=%s size=%d\n",
				f.Filename, f.Type, f.Size)
		}
	}
	fmt.Printf("empty-architecture files: %d\n", emptyArch)

	fmt.Println()
	fmt.Println("--- 5. structural validation (every file: parseable sha256 hex + positive size) ---")
	anomalies := validateFiles(fromSJSON.Files)
	if len(anomalies) == 0 {
		fmt.Printf("validation: OK (%d files, no anomalies)\n", len(fromSJSON.Files))
	} else {
		label := "ies"
		if len(anomalies) == 1 {
			label = "y"
		}
		fmt.Printf("validation: %d anomal%s\n", len(anomalies), label)
		for _, a := range anomalies {
			fmt.Printf("  - %s\n", a)
		}
	}

	fmt.Println()
	fmt.Println("--- 6. metadata size cap ---")
	fmt.Printf("index.json:     %d bytes (%.2f KiB)\n", indexLen, float64(indexLen)/1024)
	fmt.Printf("index.sjson:    %d bytes (%.2f KiB)\n", indexSJSONLen, float64(indexSJSONLen)/1024)
	fmt.Printf("update.json:    %d bytes (%.2f KiB)\n", updateJSONLen, float64(updateJSONLen)/1024)
	fmt.Printf("update.sjson:   %d bytes (%.2f KiB)\n", updateSJSONLen, float64(updateSJSONLen)/1024)
	largest := updateSJSONLen
	if updateJSONLen > largest {
		largest = updateJSONLen
	}
	fmt.Printf("largest ReleaseMetadata document: %d bytes\n", largest)
	fmt.Printf("vs 1 MiB:  %.1fx headroom\n", float64(1<<20)/float64(largest))
	fmt.Printf("vs 8 MiB:  %.1fx headroom\n", float64(8<<20)/float64(largest))
	fmt.Println("DECISION: Phase 3a should cap update.json / update.sjson at 1 MiB.")
	fmt.Println("Observed max is well under 1 MiB; 1 MiB leaves ~70x headroom and is tighter")
	fmt.Println("than the 8 MiB placeholder. Index stays at the planned 64 MiB LimitReader.")

	fmt.Println()
	fmt.Println("--- trimmed index excerpt (newest update, first 2 files) ---")
	excerpt := trimmedExcerpt(index, 2)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(excerpt); err != nil {
		return err
	}

	fmt.Println("=== done ===")
	return nil
}

type mimeInfo struct {
	contentType string
	protocol    string
	micalg      string
	boundary    string
	parts       int
}

func parseSJSON(raw []byte) (payload, sigDER []byte, info mimeInfo, err error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, info, fmt.Errorf("mail.ReadMessage: %w", err)
	}
	ct := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, nil, info, fmt.Errorf("ParseMediaType: %w", err)
	}
	info.contentType = mediaType
	info.protocol = params["protocol"]
	info.micalg = params["micalg"]
	info.boundary = params["boundary"]
	if !strings.EqualFold(mediaType, "multipart/signed") {
		return nil, nil, info, fmt.Errorf("expected multipart/signed, got %q", mediaType)
	}
	if info.boundary == "" {
		return nil, nil, info, fmt.Errorf("missing multipart boundary")
	}

	mr := multipart.NewReader(msg.Body, info.boundary)
	var bodies [][]byte
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, info, fmt.Errorf("NextPart: %w", err)
		}
		// Read immediately: NextPart invalidates the previous part's body.
		body, err := io.ReadAll(p)
		if err != nil {
			return nil, nil, info, fmt.Errorf("read part %d: %w", len(bodies), err)
		}
		fmt.Printf("  part[%d] Content-Type=%q encoding=%q disposition=%q bytes=%d\n",
			len(bodies), p.Header.Get("Content-Type"),
			p.Header.Get("Content-Transfer-Encoding"),
			p.Header.Get("Content-Disposition"), len(body))
		bodies = append(bodies, body)
	}
	info.parts = len(bodies)
	if len(bodies) != 2 {
		return nil, nil, info, fmt.Errorf("expected 2 parts, got %d", len(bodies))
	}

	payload = bytes.TrimSpace(bodies[0])

	sigRaw := bytes.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, bodies[1])
	sigDER, err = base64.StdEncoding.DecodeString(string(sigRaw))
	if err != nil {
		return nil, nil, info, fmt.Errorf("base64-decode pkcs7: %w", err)
	}
	return payload, sigDER, info, nil
}

func validateFiles(files []apiimages.UpdateFile) []string {
	var out []string
	for i, f := range files {
		prefix := fmt.Sprintf("files[%d] %s", i, f.Filename)
		if f.Filename == "" {
			out = append(out, prefix+": empty filename")
		}
		sum, err := hex.DecodeString(f.Sha256)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: sha256 not parseable hex: %v", prefix, err))
		} else if len(sum) != 32 {
			out = append(out, fmt.Sprintf("%s: sha256 decoded to %d bytes, want 32", prefix, len(sum)))
		}
		if f.Size <= 0 {
			out = append(out, fmt.Sprintf("%s: size %d is not positive", prefix, f.Size))
		}
	}
	return out
}

func getCapped(client *http.Client, rawURL string) ([]byte, int, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%s: HTTP %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, metadataCap+1))
	if err != nil {
		return nil, 0, err
	}
	if int64(len(body)) > metadataCap {
		return nil, 0, fmt.Errorf("%s: body exceeded %d-byte cap", rawURL, metadataCap)
	}
	return body, len(body), nil
}

func head(client *http.Client, rawURL string) (status int, contentLength string, err error) {
	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Content-Length"), nil
}

func rangeGET(client *http.Client, rawURL, byteRange string) (status int, contentLength, contentRange string, n int, err error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", "", 0, err
	}
	req.Header.Set("Range", byteRange)
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return resp.StatusCode, resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), 0, err
	}
	return resp.StatusCode, resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), len(body), nil
}

func expectEOF(dec *json.Decoder, name string) error {
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s: trailing JSON after first value", name)
		}
		return fmt.Errorf("%s: after first value: %w", name, err)
	}
	return nil
}

func formatCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func trimmedExcerpt(index apiimages.Index, maxFiles int) map[string]any {
	if len(index.Updates) == 0 {
		return map[string]any{"format": index.Format, "updates": []any{}}
	}
	u := index.Updates[0]
	files := u.Files
	truncated := false
	if len(files) > maxFiles {
		files = files[:maxFiles]
		truncated = true
	}
	fileMaps := make([]map[string]any, 0, len(files))
	for _, f := range files {
		fileMaps = append(fileMaps, map[string]any{
			"architecture": f.Architecture,
			"component":    f.Component,
			"filename":     f.Filename,
			"sha256":       f.Sha256,
			"size":         f.Size,
			"type":         f.Type,
		})
	}
	update := map[string]any{
		"format":       u.Format,
		"version":      u.Version,
		"origin":       u.Origin,
		"severity":     u.Severity,
		"published_at": u.PublishedAt.UTC().Format(time.RFC3339Nano),
		"channels":     u.Channels,
		"url":          u.URL,
		"files":        fileMaps,
	}
	if truncated {
		update["files_truncated"] = true
		update["files_total"] = len(u.Files)
	}
	return map[string]any{
		"format":        index.Format,
		"updates":       []any{update},
		"updates_total": len(index.Updates),
	}
}
