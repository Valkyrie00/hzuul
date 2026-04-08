package views

import "github.com/gdamore/tcell/v2"

var (
	ColorBg        = tcell.ColorDefault
	ColorNavBg     = tcell.ColorDefault
	ColorHeaderBg  = tcell.ColorDefault
	ColorAccent    = tcell.NewRGBColor(56, 132, 244)
	ColorMuted     = tcell.NewRGBColor(120, 120, 140)
	ColorDim       = tcell.NewRGBColor(90, 90, 110)
	ColorSep       = tcell.NewRGBColor(50, 50, 65)
	ColorSelectBg = tcell.NewRGBColor(50, 52, 70)
)

var SelectedStyle = tcell.StyleDefault.
	Background(ColorSelectBg).
	Foreground(tcell.ColorWhite)
