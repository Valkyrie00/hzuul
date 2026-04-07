package views

import (
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
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

// BookmarkAwareView can be implemented by views that contain a BuildLogView
// and need the BookmarkManager injected after construction.
type BookmarkAwareView interface {
	SetBookmarkManager(bm *BookmarkManager)
}
