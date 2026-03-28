package tui

import "github.com/gdamore/tcell/v2"

var (
	ColorBg          = tcell.ColorDefault
	ColorFg          = tcell.ColorDefault
	ColorAccent      = tcell.NewRGBColor(56, 132, 244)
	ColorSuccess     = tcell.NewRGBColor(72, 199, 142)
	ColorFailure     = tcell.NewRGBColor(235, 87, 87)
	ColorWarning     = tcell.NewRGBColor(242, 201, 76)
	ColorRunning     = tcell.NewRGBColor(56, 132, 244)
	ColorMuted       = tcell.NewRGBColor(120, 120, 140)
	ColorNavBg       = tcell.ColorDefault
	ColorNavActive   = tcell.NewRGBColor(56, 132, 244)
	ColorHeaderBg    = tcell.ColorDefault
	ColorTableHeader = tcell.NewRGBColor(56, 132, 244)
	ColorBorder      = tcell.NewRGBColor(50, 50, 65)
	ColorSeparator   = tcell.NewRGBColor(50, 50, 65)
	ColorSection     = tcell.NewRGBColor(200, 200, 220)
)

func ResultColor(result string) tcell.Color {
	switch result {
	case "SUCCESS":
		return ColorSuccess
	case "FAILURE", "ERROR", "RETRY_LIMIT":
		return ColorFailure
	case "LOST", "ABORTED", "DISK_FULL", "TIMED_OUT":
		return ColorWarning
	default:
		return ColorRunning
	}
}
