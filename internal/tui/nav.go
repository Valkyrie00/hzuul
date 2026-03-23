package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var tabNames = []string{
	"Status",
	"Projects",
	"Jobs",
	"Labels",
	"Nodes",
	"Autoholds",
	"Semaphores",
	"Builds",
	"Buildsets",
}

type NavBar struct {
	*tview.TextView
	active int
	onTab  func(index int)
}

func NewNavBar(onTab func(int)) *NavBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(ColorNavBg)

	n := &NavBar{
		TextView: tv,
		active:   0,
		onTab:    onTab,
	}
	n.render()
	return n
}

func (n *NavBar) render() {
	n.Clear()
	fmt.Fprint(n, " ")
	for i, name := range tabNames {
		if i > 0 {
			fmt.Fprint(n, "  ")
		}
		shortcut := fmt.Sprintf("%d", i+1)
		if i == n.active {
			fmt.Fprintf(n, "[#DCDCE6::b]%s·%s[-::-]", shortcut, name)
		} else {
			fmt.Fprintf(n, "[#3884F4]%s[-][#78788C]·%s[-]", shortcut, name)
		}
	}
}

func (n *NavBar) SetActive(index int) {
	if index < 0 || index >= len(tabNames) {
		return
	}
	n.active = index
	n.render()
	if n.onTab != nil {
		n.onTab(index)
	}
}

func (n *NavBar) Active() int {
	return n.active
}

func (n *NavBar) HandleKey(event *tcell.EventKey) bool {
	if event.Key() == tcell.KeyTab {
		next := (n.active + 1) % len(tabNames)
		n.SetActive(next)
		return true
	}
	if event.Key() == tcell.KeyBacktab {
		prev := (n.active - 1 + len(tabNames)) % len(tabNames)
		n.SetActive(prev)
		return true
	}
	r := event.Rune()
	if r >= '1' && r <= '9' {
		idx := int(r - '1')
		if idx < len(tabNames) {
			n.SetActive(idx)
			return true
		}
	}
	return false
}
