package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repo       = "Valkyrie00/hzuul"
	binaryName = "hzuul"
)

var apiURL = "https://api.github.com/repos/" + repo + "/releases/latest"

type Result struct {
	Available bool
	Current   string
	Latest    string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func Check(current string) (*Result, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current = strings.TrimPrefix(current, "v")

	return &Result{
		Available: latest != current && current != "dev" && compareVersions(latest, current) > 0,
		Current:   current,
		Latest:    latest,
	}, nil
}

// SelfUpdate downloads the latest release and replaces the running binary.
func SelfUpdate(current string) error {
	res, err := Check(current)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if !res.Available {
		fmt.Printf("hzuul %s is already up to date.\n", current)
		return nil
	}

	fmt.Printf("Updating hzuul %s → %s ...\n", res.Current, res.Latest)

	assetName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, res.Latest, runtime.GOOS, runtime.GOARCH)
	assetURL := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, res.Latest, assetName)
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/v%s/checksums.txt", repo, res.Latest)

	tmpDir, err := os.MkdirTemp("", "hzuul-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(archivePath, assetURL); err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	if err := verifyChecksum(archivePath, assetName, checksumURL); err != nil {
		return fmt.Errorf("checksum verification: %w", err)
	}
	fmt.Println("Checksum verified ✓")

	newBinary := filepath.Join(tmpDir, binaryName)
	if err := extractBinary(archivePath, newBinary); err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating current binary: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	if err := replaceBinary(exePath, newBinary); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	fmt.Printf("Updated to hzuul %s ✓\n", res.Latest)
	return nil
}

func downloadFile(dst, url string) (retErr error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %d", url, resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyChecksum(archivePath, assetName, checksumURL string) error {
	resp, err := httpClient.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("fetching checksums.txt: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums.txt not found (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var expected string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, assetName) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				expected = parts[0]
			}
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum found for %s", assetName)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func extractBinary(archivePath, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

// replaceBinary atomically swaps the old binary with the new one.
// On Unix this works via rename on the same filesystem, falling back
// to remove-and-rename when the target sits on a different device.
func replaceBinary(oldPath, newPath string) error {
	info, err := os.Stat(oldPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(oldPath)
	tmp, err := os.CreateTemp(dir, ".hzuul-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()

	cleanup := func() { _ = os.Remove(tmpPath) }

	src, err := os.Open(newPath)
	if err != nil {
		cleanup()
		return err
	}

	dst, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		_ = src.Close()
		cleanup()
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = src.Close()
		_ = dst.Close()
		cleanup()
		return err
	}
	_ = src.Close()
	if err := dst.Close(); err != nil {
		cleanup()
		return err
	}

	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		cleanup()
		return err
	}

	if err := os.Rename(tmpPath, oldPath); err != nil {
		cleanup()
		return err
	}
	return nil
}

// compareVersions compares two dotted version strings (e.g. "0.4.1", "0.5.0").
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")

	max := len(pa)
	if len(pb) > max {
		max = len(pb)
	}

	for i := 0; i < max; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			va = atoi(pa[i])
		}
		if i < len(pb) {
			vb = atoi(pb[i])
		}
		if va != vb {
			return va - vb
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
