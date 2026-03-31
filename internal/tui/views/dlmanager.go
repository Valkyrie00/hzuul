package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
)

type DLStatus string

const (
	DLDownloading DLStatus = "downloading"
	DLCompleted   DLStatus = "completed"
	DLFailed      DLStatus = "failed"
	DLCancelled   DLStatus = "cancelled"
)

type DownloadRecord struct {
	UUID       string   `json:"uuid"`
	JobName    string   `json:"job_name"`
	Project    string   `json:"project"`
	Tenant     string   `json:"tenant"`
	Status     DLStatus `json:"status"`
	TotalFiles int      `json:"total_files"`
	DoneFiles  int      `json:"done_files"`
	TotalBytes int64    `json:"total_bytes"`
	DoneBytes  int64    `json:"done_bytes"`
	DestDir    string   `json:"dest_dir"`
	StartedAt  string   `json:"started_at"`
	EndedAt    string   `json:"ended_at,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type DownloadManager struct {
	mu        sync.Mutex
	records   []DownloadRecord
	stopChs   map[string]chan struct{}
	app       *tview.Application
	listeners []func()
}

func NewDownloadManager(app *tview.Application) *DownloadManager {
	dm := &DownloadManager{
		app:     app,
		stopChs: make(map[string]chan struct{}),
	}
	dm.loadHistory()
	return dm
}

// SetOnChange registers a callback fired after any record changes.
// Multiple listeners can be registered.
func (dm *DownloadManager) SetOnChange(fn func()) {
	dm.listeners = append(dm.listeners, fn)
}

func (dm *DownloadManager) Records() []DownloadRecord {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	out := make([]DownloadRecord, len(dm.records))
	copy(out, dm.records)
	return out
}

func (dm *DownloadManager) IsDownloading(uuid string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	_, ok := dm.stopChs[uuid]
	return ok
}

func (dm *DownloadManager) ActiveDownloadCount() int {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return len(dm.stopChs)
}

// Start begins a background download and returns immediately.
// progressUI is called on the tview goroutine with each progress tick;
// it can be nil if no UI progress display is needed.
func (dm *DownloadManager) Start(
	client *api.Client,
	build *api.Build,
	destDir string,
	progressUI func(api.DownloadProgress),
) {
	stopCh := make(chan struct{})

	rec := DownloadRecord{
		UUID:      build.UUID,
		JobName:   build.JobName,
		Project:   build.Ref.Project,
		Tenant:    client.Tenant(),
		Status:    DLDownloading,
		DestDir:   destDir,
		StartedAt: time.Now().Format(time.RFC3339),
	}

	dm.mu.Lock()
	dm.stopChs[build.UUID] = stopCh
	dm.records = append([]DownloadRecord{rec}, dm.records...)
	dm.mu.Unlock()
	dm.notify()

	go func() {
		err := client.DownloadBuildLogs(build, destDir, stopCh, func(p api.DownloadProgress) {
			dm.mu.Lock()
			if idx := dm.findLocked(build.UUID); idx >= 0 {
				dm.records[idx].TotalFiles = p.TotalFiles
				dm.records[idx].DoneFiles = p.DoneFiles
				dm.records[idx].TotalBytes = p.TotalBytes
				dm.records[idx].DoneBytes = p.DoneBytes
			}
			dm.mu.Unlock()

			dm.app.QueueUpdateDraw(func() {
				dm.notify()
				if progressUI != nil {
					progressUI(p)
				}
			})
		})

		dm.mu.Lock()
		delete(dm.stopChs, build.UUID)
		if idx := dm.findLocked(build.UUID); idx >= 0 {
			dm.records[idx].EndedAt = time.Now().Format(time.RFC3339)
			if err != nil {
				select {
				case <-stopCh:
					dm.records[idx].Status = DLCancelled
				default:
					dm.records[idx].Status = DLFailed
					dm.records[idx].Error = err.Error()
				}
			} else {
				dm.records[idx].Status = DLCompleted
			}
		}
		dm.mu.Unlock()

		dm.saveHistory()
		dm.app.QueueUpdateDraw(func() {
			dm.notify()
		})
	}()
}

func (dm *DownloadManager) Cancel(uuid string) {
	dm.mu.Lock()
	if ch, ok := dm.stopChs[uuid]; ok {
		close(ch)
		delete(dm.stopChs, uuid)
	}
	dm.mu.Unlock()
}

func (dm *DownloadManager) Remove(uuid string) {
	dm.mu.Lock()
	if idx := dm.findLocked(uuid); idx >= 0 {
		if dm.records[idx].Status == DLDownloading {
			dm.mu.Unlock()
			return
		}
		dm.records = append(dm.records[:idx], dm.records[idx+1:]...)
	}
	dm.mu.Unlock()
	dm.saveHistory()
	dm.notify()
}

func (dm *DownloadManager) findLocked(uuid string) int {
	for i := range dm.records {
		if dm.records[i].UUID == uuid {
			return i
		}
	}
	return -1
}

func (dm *DownloadManager) notify() {
	for _, fn := range dm.listeners {
		fn()
	}
}

func historyPath() string {
	return filepath.Join(config.DataDir(), "history.json")
}

func (dm *DownloadManager) loadHistory() {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return
	}
	var records []DownloadRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return
	}
	// Only load finished records (downloading state is transient)
	for i := range records {
		if records[i].Status == DLDownloading {
			records[i].Status = DLFailed
			records[i].Error = "interrupted"
		}
	}
	dm.records = records
}

func (dm *DownloadManager) saveHistory() {
	dm.mu.Lock()
	var toSave []DownloadRecord
	for _, r := range dm.records {
		if r.Status != DLDownloading {
			toSave = append(toSave, r)
		}
	}
	dm.mu.Unlock()

	dir := filepath.Dir(historyPath())
	_ = os.MkdirAll(dir, 0o755)

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(historyPath(), data, 0o644)
}

func DefaultDownloadDir(tenant, uuid string) string {
	return filepath.Join(config.DataDir(), "logs", tenant, uuid)
}

func FormatBytes(b int64) string {
	mb := float64(b) / 1024 / 1024
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", mb/1024)
	}
	if mb >= 1 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	kb := float64(b) / 1024
	return fmt.Sprintf("%.0f KB", kb)
}
