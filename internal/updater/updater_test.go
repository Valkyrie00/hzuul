package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not supported on Windows")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"0.10.0", "0.9.0", 1},
		{"0.4.0", "0.4.0", 0},
		{"1.0", "1.0.0", 0},
		{"1.0.0", "1.0", 0},
		{"0.0.1", "0.0.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			switch {
			case tt.want > 0 && got <= 0:
				t.Errorf("compareVersions(%q, %q) = %d, want > 0", tt.a, tt.b, got)
			case tt.want < 0 && got >= 0:
				t.Errorf("compareVersions(%q, %q) = %d, want < 0", tt.a, tt.b, got)
			case tt.want == 0 && got != 0:
				t.Errorf("compareVersions(%q, %q) = %d, want 0", tt.a, tt.b, got)
			}
		})
	}
}

func TestCheck_NewVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v0.5.0"}`))
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	res, err := Check("0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Available {
		t.Error("expected Available = true")
	}
	if res.Latest != "0.5.0" {
		t.Errorf("Latest = %q, want 0.5.0", res.Latest)
	}
	if res.Current != "0.4.0" {
		t.Errorf("Current = %q, want 0.4.0", res.Current)
	}
}

func TestCheck_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v0.4.0"}`))
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	res, err := Check("0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Available {
		t.Error("expected Available = false when versions match")
	}
}

func TestCheck_DevVersionNeverAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	res, err := Check("dev")
	if err != nil {
		t.Fatal(err)
	}
	if res.Available {
		t.Error("expected Available = false for dev version")
	}
}

func TestCheck_CurrentNewerThanLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v0.3.0"}`))
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	res, err := Check("0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Available {
		t.Error("expected Available = false when current is newer")
	}
}

func TestCheck_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	_, err := Check("0.4.0")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestCheck_VPrefixStripped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v0.5.0"}`))
	}))
	defer srv.Close()

	origClient := httpClient
	origURL := apiURL
	httpClient = srv.Client()
	overrideAPIURL(t, srv.URL)
	defer func() { httpClient = origClient; restoreAPIURL(origURL) }()

	res, err := Check("v0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Current != "0.4.0" {
		t.Errorf("Current should have v prefix stripped: %q", res.Current)
	}
	if res.Latest != "0.5.0" {
		t.Errorf("Latest should have v prefix stripped: %q", res.Latest)
	}
}

func TestExtractBinary(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	payload := []byte("#!/bin/sh\necho hello\n")

	createTarGz(t, archivePath, "hzuul", payload)

	dst := filepath.Join(tmpDir, "hzuul")
	if err := extractBinary(archivePath, dst); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("extracted content = %q, want %q", got, payload)
	}
}

func TestExtractBinary_MissingBinary(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")

	createTarGz(t, archivePath, "other-binary", []byte("data"))

	dst := filepath.Join(tmpDir, "hzuul")
	err := extractBinary(archivePath, dst)
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
}

func TestVerifyChecksum_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("binary content here")
	archivePath := filepath.Join(tmpDir, "hzuul_0.5.0_darwin_arm64.tar.gz")
	os.WriteFile(archivePath, content, 0o644)

	h := sha256.Sum256(content)
	checksumLine := fmt.Sprintf("%x  hzuul_0.5.0_darwin_arm64.tar.gz\n", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumLine))
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	err := verifyChecksum(archivePath, "hzuul_0.5.0_darwin_arm64.tar.gz", srv.URL+"/checksums.txt")
	if err != nil {
		t.Fatalf("expected valid checksum, got: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "hzuul_0.5.0_darwin_arm64.tar.gz")
	os.WriteFile(archivePath, []byte("actual content"), 0o644)

	checksumLine := "0000000000000000000000000000000000000000000000000000000000000000  hzuul_0.5.0_darwin_arm64.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumLine))
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	err := verifyChecksum(archivePath, "hzuul_0.5.0_darwin_arm64.tar.gz", srv.URL+"/checksums.txt")
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
}

func TestVerifyChecksum_MissingEntry(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "hzuul_0.5.0_darwin_arm64.tar.gz")
	os.WriteFile(archivePath, []byte("content"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("abc123  some_other_file.tar.gz\n"))
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	err := verifyChecksum(archivePath, "hzuul_0.5.0_darwin_arm64.tar.gz", srv.URL+"/checksums.txt")
	if err == nil {
		t.Fatal("expected error when asset not in checksums.txt")
	}
}

func TestReplaceBinary(t *testing.T) {
	tmpDir := t.TempDir()

	oldPath := filepath.Join(tmpDir, "hzuul")
	os.WriteFile(oldPath, []byte("old"), 0o755)

	newPath := filepath.Join(tmpDir, "hzuul-new")
	os.WriteFile(newPath, []byte("new"), 0o755)

	if err := replaceBinary(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("replaced content = %q, want %q", got, "new")
	}
}

func TestDownloadFile(t *testing.T) {
	body := "downloaded content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	dst := filepath.Join(t.TempDir(), "file")
	if err := downloadFile(dst, srv.URL+"/asset"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != body {
		t.Errorf("downloaded = %q, want %q", got, body)
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	dst := filepath.Join(t.TempDir(), "file")
	err := downloadFile(dst, srv.URL+"/missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

// --- write-error tests ---

func TestDownloadFile_CreateFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	dst := filepath.Join(t.TempDir(), "no-such-dir", "file")
	err := downloadFile(dst, srv.URL+"/asset")
	if err == nil {
		t.Fatal("expected error when destination directory does not exist")
	}
}

func TestExtractBinary_CorruptArchive(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "corrupt.tar.gz")
	os.WriteFile(archivePath, []byte("this is not a gzip file"), 0o644)

	dst := filepath.Join(tmpDir, "hzuul")
	err := extractBinary(archivePath, dst)
	if err == nil {
		t.Fatal("expected error for corrupt archive")
	}
}

func TestReplaceBinary_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	oldPath := filepath.Join(tmpDir, "hzuul")
	os.WriteFile(oldPath, []byte("old"), 0o755)

	err := replaceBinary(oldPath, filepath.Join(tmpDir, "does-not-exist"))
	if err == nil {
		t.Fatal("expected error when source binary does not exist")
	}

	got, _ := os.ReadFile(oldPath)
	if string(got) != "old" {
		t.Errorf("original binary was modified: %q", got)
	}

	matches, _ := filepath.Glob(filepath.Join(tmpDir, ".hzuul-update-*"))
	if len(matches) > 0 {
		t.Errorf("temp file not cleaned up: %v", matches)
	}
}

func TestReplaceBinary_OldPathNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	newPath := filepath.Join(tmpDir, "hzuul-new")
	os.WriteFile(newPath, []byte("new"), 0o755)

	err := replaceBinary(filepath.Join(tmpDir, "no-such-binary"), newPath)
	if err == nil {
		t.Fatal("expected error when old binary does not exist")
	}
}

// Tests that rely on Unix file permissions (chmod, mode bits).
// Skipped entirely on Windows where these concepts don't apply.
func TestUnixPermissions(t *testing.T) {
	skipOnWindows(t)

	t.Run("ExtractBinary_Executable", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.gz")
		createTarGz(t, archivePath, "hzuul", []byte("binary"))

		dst := filepath.Join(tmpDir, "hzuul")
		if err := extractBinary(archivePath, dst); err != nil {
			t.Fatal(err)
		}

		info, _ := os.Stat(dst)
		if info.Mode()&0o100 == 0 {
			t.Error("expected binary to be executable")
		}
	})

	t.Run("ReplaceBinary_PreservesExecutable", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldPath := filepath.Join(tmpDir, "hzuul")
		os.WriteFile(oldPath, []byte("old"), 0o755)

		newPath := filepath.Join(tmpDir, "hzuul-new")
		os.WriteFile(newPath, []byte("new"), 0o755)

		if err := replaceBinary(oldPath, newPath); err != nil {
			t.Fatal(err)
		}

		info, _ := os.Stat(oldPath)
		if info.Mode()&0o111 == 0 {
			t.Error("replaced binary should be executable")
		}
	})

	t.Run("DownloadFile_ReadOnlyDir", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("data"))
		}))
		defer srv.Close()

		origClient := httpClient
		httpClient = srv.Client()
		defer func() { httpClient = origClient }()

		dir := t.TempDir()
		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })

		dst := filepath.Join(dir, "file")
		err := downloadFile(dst, srv.URL+"/asset")
		if err == nil {
			t.Fatal("expected error when directory is read-only")
		}
	})

	t.Run("ExtractBinary_ReadOnlyDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "test.tar.gz")
		createTarGz(t, archivePath, "hzuul", []byte("binary"))

		readOnlyDir := filepath.Join(tmpDir, "readonly")
		os.Mkdir(readOnlyDir, 0o555)
		t.Cleanup(func() { os.Chmod(readOnlyDir, 0o755) })

		dst := filepath.Join(readOnlyDir, "hzuul")
		err := extractBinary(archivePath, dst)
		if err == nil {
			t.Fatal("expected error when output directory is read-only")
		}
	})

	t.Run("ReplaceBinary_ReadOnlyTargetDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		dir := filepath.Join(tmpDir, "readonly")
		os.Mkdir(dir, 0o755)

		oldPath := filepath.Join(dir, "hzuul")
		os.WriteFile(oldPath, []byte("old"), 0o755)

		newPath := filepath.Join(tmpDir, "hzuul-new")
		os.WriteFile(newPath, []byte("new"), 0o755)

		os.Chmod(dir, 0o555)
		t.Cleanup(func() { os.Chmod(dir, 0o755) })

		err := replaceBinary(oldPath, newPath)
		if err == nil {
			t.Fatal("expected error when target directory is read-only")
		}

		os.Chmod(dir, 0o755)
		got, _ := os.ReadFile(oldPath)
		if string(got) != "old" {
			t.Errorf("original binary was modified: %q", got)
		}
	})
}

// --- test helpers ---

func overrideAPIURL(t *testing.T, url string) {
	t.Helper()
	apiURL = url
}

func restoreAPIURL(orig string) {
	apiURL = orig
}

func createTarGz(t *testing.T, path, name string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
}
