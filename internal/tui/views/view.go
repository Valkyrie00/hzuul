package views

import (
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/rivo/tview"
)

// View is the interface that all TUI views implement.
type View interface {
	Root() tview.Primitive
	Load(client *api.Client)
	SetFilter(term string)
}

// ModalView can be implemented by views that have modal/form overlays
// that need exclusive keyboard control (bypassing global shortcuts).
type ModalView interface {
	IsModal() bool
}

// LiveFilterable is implemented by views that hold their data in memory
// and can re-filter instantly on each keystroke without API calls.
type LiveFilterable interface {
	IsLiveFilterable() bool
}

// Reconnectable is implemented by views that can reconnect a dead stream.
type Reconnectable interface {
	CanReconnect() bool
	Reconnect()
}

// KeyHintProvider is implemented by views that declare their key hints
// to the unified KeyBar. KeyHints returns view-specific hints for the
// current state; the app appends global hints automatically.
// Return nil when the view manages the KeyBar directly (e.g. confirm prompts).
type KeyHintProvider interface {
	KeyHints() []KeyHint
}

// StatusUpdater is implemented by views that display a count or status
// in the KeyBar. Called whenever the view becomes active.
type StatusUpdater interface {
	UpdateStatus()
}

// BookmarkAwareView can be implemented by views that contain a BuildLogView
// and need the BookmarkManager injected after construction.
type BookmarkAwareView interface {
	SetBookmarkManager(bm *BookmarkManager)
}
