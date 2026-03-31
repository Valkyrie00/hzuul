package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type ManifestNode struct {
	Name     string         `json:"name"`
	MimeType string         `json:"mimetype,omitempty"`
	Size     int64          `json:"size,omitempty"`
	Children []ManifestNode `json:"children,omitempty"`
}

type ManifestTree struct {
	Tree []ManifestNode `json:"tree"`
}

type FileEntry struct {
	Path string
	Size int64
}

type DownloadProgress struct {
	TotalFiles  int
	DoneFiles   int
	TotalBytes  int64
	DoneBytes   int64
	CurrentFile string
	Err         error
}

func GetManifestURL(build *Build) string {
	for _, a := range build.Artifacts {
		if t, ok := a.Metadata["type"]; ok {
			if ts, ok := t.(string); ok && ts == "zuul_manifest" {
				return a.URL
			}
		}
	}
	return ""
}

func (c *Client) FetchManifest(manifestURL string) (*ManifestTree, error) {
	resp, err := c.RawGet(manifestURL)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	var tree ManifestTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}
	return &tree, nil
}

func CollectFiles(nodes []ManifestNode) []FileEntry {
	var entries []FileEntry
	collectRecursive("", nodes, &entries)
	return entries
}

func collectRecursive(basePath string, nodes []ManifestNode, entries *[]FileEntry) {
	for _, n := range nodes {
		path := basePath + "/" + n.Name
		if len(n.Children) > 0 {
			collectRecursive(path, n.Children, entries)
		} else {
			*entries = append(*entries, FileEntry{Path: path, Size: n.Size})
		}
	}
}

// DownloadBuildLogs downloads all log artifacts for a build into destDir.
// The progress callback is invoked from worker goroutines after each file.
// Close stopCh to cancel the download.
func (c *Client) DownloadBuildLogs(
	build *Build,
	destDir string,
	stopCh <-chan struct{},
	progress func(DownloadProgress),
) error {
	manifestURL := GetManifestURL(build)
	if manifestURL == "" {
		return fmt.Errorf("no log manifest found for this build")
	}

	manifest, err := c.FetchManifest(manifestURL)
	if err != nil {
		return err
	}

	files := CollectFiles(manifest.Tree)
	if len(files) == 0 {
		return fmt.Errorf("manifest is empty — no files to download")
	}

	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
	}

	baseLogURL := strings.TrimRight(build.LogURL, "/")

	var doneFiles atomic.Int64
	var doneBytes atomic.Int64

	const workers = 10
	work := make(chan FileEntry, len(files))
	for _, f := range files {
		work <- f
	}
	close(work)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fe := range work {
				select {
				case <-stopCh:
					return
				default:
				}

				err := c.downloadFile(baseLogURL+fe.Path, filepath.Join(destDir, fe.Path))
				if err != nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("downloading %s: %w", fe.Path, err) })
				}

				done := int(doneFiles.Add(1))
				bytes := doneBytes.Add(fe.Size)
				if progress != nil {
					progress(DownloadProgress{
						TotalFiles:  len(files),
						DoneFiles:   done,
						TotalBytes:  totalBytes,
						DoneBytes:   bytes,
						CurrentFile: fe.Path,
					})
				}
			}
		}()
	}

	wg.Wait()
	return firstErr
}

func (c *Client) downloadFile(fileURL, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	resp, err := c.RawGet(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
