package views

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
)

type BookmarkRecord struct {
	UUID      string `json:"uuid"`
	JobName   string `json:"job_name"`
	Project   string `json:"project"`
	Branch    string `json:"branch"`
	Pipeline  string `json:"pipeline"`
	Result    string `json:"result"`
	Change    string `json:"change,omitempty"`
	RefURL    string `json:"ref_url,omitempty"`
	LogURL    string `json:"log_url,omitempty"`
	Tenant    string `json:"tenant"`
	StartTime string `json:"start_time"`
	SavedAt   string `json:"saved_at"`
}

type BookmarkManager struct {
	mu        sync.Mutex
	records   []BookmarkRecord
	listeners []func()
}

func NewBookmarkManager() *BookmarkManager {
	bm := &BookmarkManager{}
	bm.loadBookmarks()
	return bm
}

func (bm *BookmarkManager) SetOnChange(fn func()) {
	bm.listeners = append(bm.listeners, fn)
}

func (bm *BookmarkManager) Records() []BookmarkRecord {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	out := make([]BookmarkRecord, len(bm.records))
	copy(out, bm.records)
	return out
}

func (bm *BookmarkManager) IsBookmarked(uuid string) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.findLocked(uuid) >= 0
}

func (bm *BookmarkManager) Toggle(client *api.Client, build *api.Build) bool {
	bm.mu.Lock()
	if idx := bm.findLocked(build.UUID); idx >= 0 {
		bm.records = append(bm.records[:idx], bm.records[idx+1:]...)
		bm.mu.Unlock()
		bm.saveBookmarks()
		bm.notify()
		return false
	}

	tenant := ""
	if client != nil {
		tenant = client.Tenant()
	}

	rec := BookmarkRecord{
		UUID:      build.UUID,
		JobName:   build.JobName,
		Project:   build.Ref.Project,
		Branch:    build.Ref.Branch,
		Pipeline:  build.Pipeline,
		Result:    build.Result,
		Change:    formatBuildChange(build),
		RefURL:    build.Ref.RefURL,
		LogURL:    build.LogURL,
		Tenant:    tenant,
		StartTime: build.StartTime,
		SavedAt:   time.Now().Format(time.RFC3339),
	}
	bm.records = append([]BookmarkRecord{rec}, bm.records...)
	bm.mu.Unlock()
	bm.saveBookmarks()
	bm.notify()
	return true
}

func (bm *BookmarkManager) Update(uuid string, build *api.Build) {
	bm.mu.Lock()
	idx := bm.findLocked(uuid)
	if idx < 0 {
		bm.mu.Unlock()
		return
	}
	r := &bm.records[idx]
	r.JobName = build.JobName
	r.Project = build.Ref.Project
	r.Branch = build.Ref.Branch
	r.Pipeline = build.Pipeline
	r.Result = build.Result
	r.LogURL = build.LogURL
	r.StartTime = build.StartTime
	if build.Ref.RefURL != "" {
		r.RefURL = build.Ref.RefURL
	}
	if c := formatBuildChange(build); c != "" {
		r.Change = c
	}
	bm.mu.Unlock()
	bm.saveBookmarks()
	bm.notify()
}

func (bm *BookmarkManager) Remove(uuid string) {
	bm.mu.Lock()
	if idx := bm.findLocked(uuid); idx >= 0 {
		bm.records = append(bm.records[:idx], bm.records[idx+1:]...)
	}
	bm.mu.Unlock()
	bm.saveBookmarks()
	bm.notify()
}

func (bm *BookmarkManager) findLocked(uuid string) int {
	for i := range bm.records {
		if bm.records[i].UUID == uuid {
			return i
		}
	}
	return -1
}

func (bm *BookmarkManager) notify() {
	for _, fn := range bm.listeners {
		fn()
	}
}

func bookmarksPath() string {
	return filepath.Join(config.DataDir(), "bookmarks.json")
}

func (bm *BookmarkManager) loadBookmarks() {
	data, err := os.ReadFile(bookmarksPath())
	if err != nil {
		return
	}
	var records []BookmarkRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return
	}
	bm.records = records
}

func (bm *BookmarkManager) saveBookmarks() {
	bm.mu.Lock()
	toSave := make([]BookmarkRecord, len(bm.records))
	copy(toSave, bm.records)
	bm.mu.Unlock()

	dir := filepath.Dir(bookmarksPath())
	_ = os.MkdirAll(dir, 0o755)

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(bookmarksPath(), data, 0o644)
}

func formatBuildChange(build *api.Build) string {
	if build.Ref.Change == nil {
		return ""
	}
	return formatChange(build.Ref)
}
