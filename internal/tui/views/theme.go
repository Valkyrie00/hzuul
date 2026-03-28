package views

import "github.com/gdamore/tcell/v2"

var (
	ColorBg       = tcell.NewRGBColor(24, 24, 32)
	ColorNavBg    = tcell.NewRGBColor(32, 32, 44)
	ColorHeaderBg = tcell.NewRGBColor(20, 20, 28)
	ColorAccent   = tcell.NewRGBColor(56, 132, 244)
	ColorMuted    = tcell.NewRGBColor(120, 120, 140)
	ColorDim      = tcell.NewRGBColor(90, 90, 110)
	ColorSep      = tcell.NewRGBColor(50, 50, 65)
	ColorSelectBg = tcell.NewRGBColor(30, 30, 42)
	ColorJobBg    = tcell.NewRGBColor(28, 28, 38)
	ColorSectionBg = tcell.NewRGBColor(30, 30, 42)
)

var SelectedStyle = tcell.StyleDefault.
	Background(ColorSelectBg).
	Foreground(tcell.ColorWhite).
	Attributes(tcell.AttrBold)
