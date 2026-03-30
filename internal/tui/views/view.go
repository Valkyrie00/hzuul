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
