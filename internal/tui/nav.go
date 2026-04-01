package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/tui/views"
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
	"Downloads",
	"Bookmarks",
}

type NavBar struct {
	*tview.TextView
	active int
	badges map[int]string
	onTab  func(index int)
}

func NewNavBar(onTab func(int)) *NavBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(views.ColorNavBg)

	n := &NavBar{
		TextView: tv,
		active:   0,
		badges:   make(map[int]string),
		onTab:    onTab,
	}
	n.render()
	return n
}

func (n *NavBar) SetBadge(index int, badge string) {
	if badge == "" {
		delete(n.badges, index)
	} else {
		n.badges[index] = badge
	}
	n.render()
}

func tabShortcut(i int) string {
	switch i {
	case 9:
		return "0"
	case 10:
		return "b"
	default:
		return fmt.Sprintf("%d", i+1)
	}
}

func (n *NavBar) render() {
	n.Clear()
	fmt.Fprint(n, " ")
	for i, name := range tabNames {
		if i > 0 {
			fmt.Fprint(n, "  ")
		}
		shortcut := tabShortcut(i)
		badge := ""
		if b, ok := n.badges[i]; ok {
			badge = fmt.Sprintf(" [yellow::b](%s)[-::-]", b)
		}
		if i == n.active {
			if badge != "" {
				fmt.Fprintf(n, "[yellow::b]%s·%s%s[-::-]", shortcut, name, badge)
			} else {
				fmt.Fprintf(n, "[#DCDCE6::b]%s·%s[-::-]", shortcut, name)
			}
		} else {
			if badge != "" {
				fmt.Fprintf(n, "[yellow]%s[-][yellow]·%s%s[-]", shortcut, name, badge)
			} else {
				fmt.Fprintf(n, "[#3884F4]%s[-][#78788C]·%s[-]", shortcut, name)
			}
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
	if r == '0' && len(tabNames) >= 10 {
		n.SetActive(9)
		return true
	}
	if r == 'b' && len(tabNames) >= 11 {
		n.SetActive(10)
		return true
	}
	return false
}
