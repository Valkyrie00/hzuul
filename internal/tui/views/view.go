package views

import (
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

// View is the interface that all TUI views implement.
type View interface {
	Root() tview.Primitive
	Load(client *api.Client)
	SetFilter(term string)
}
