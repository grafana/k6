package ansi

// SelectGraphicRendition (SGR) is a command that sets display attributes.
//
// Default is 0.
//
//	CSI Ps ; Ps ... m
//
// See: https://vt100.net/docs/vt510-rm/SGR.html
func SelectGraphicRendition(ps ...Attr) string {
	if len(ps) == 0 {
		return ResetStyle
	}

	return NewStyle(ps...).String()
}

// SGR is an alias for [SelectGraphicRendition].
func SGR(ps ...Attr) string {
	return SelectGraphicRendition(ps...)
}

var attrStrings = map[int]string{
	ResetAttr:                        resetAttr,
	BoldAttr:                         boldAttr,
	FaintAttr:                        faintAttr,
	ItalicAttr:                       italicAttr,
	UnderlineAttr:                    underlineAttr,
	SlowBlinkAttr:                    slowBlinkAttr,
	RapidBlinkAttr:                   rapidBlinkAttr,
	ReverseAttr:                      reverseAttr,
	ConcealAttr:                      concealAttr,
	StrikethroughAttr:                strikethroughAttr,
	NormalIntensityAttr:              normalIntensityAttr,
	NoItalicAttr:                     noItalicAttr,
	NoUnderlineAttr:                  noUnderlineAttr,
	NoBlinkAttr:                      noBlinkAttr,
	NoReverseAttr:                    noReverseAttr,
	NoConcealAttr:                    noConcealAttr,
	NoStrikethroughAttr:              noStrikethroughAttr,
	BlackForegroundColorAttr:         blackForegroundColorAttr,
	RedForegroundColorAttr:           redForegroundColorAttr,
	GreenForegroundColorAttr:         greenForegroundColorAttr,
	YellowForegroundColorAttr:        yellowForegroundColorAttr,
	BlueForegroundColorAttr:          blueForegroundColorAttr,
	MagentaForegroundColorAttr:       magentaForegroundColorAttr,
	CyanForegroundColorAttr:          cyanForegroundColorAttr,
	WhiteForegroundColorAttr:         whiteForegroundColorAttr,
	ExtendedForegroundColorAttr:      extendedForegroundColorAttr,
	DefaultForegroundColorAttr:       defaultForegroundColorAttr,
	BlackBackgroundColorAttr:         blackBackgroundColorAttr,
	RedBackgroundColorAttr:           redBackgroundColorAttr,
	GreenBackgroundColorAttr:         greenBackgroundColorAttr,
	YellowBackgroundColorAttr:        yellowBackgroundColorAttr,
	BlueBackgroundColorAttr:          blueBackgroundColorAttr,
	MagentaBackgroundColorAttr:       magentaBackgroundColorAttr,
	CyanBackgroundColorAttr:          cyanBackgroundColorAttr,
	WhiteBackgroundColorAttr:         whiteBackgroundColorAttr,
	ExtendedBackgroundColorAttr:      extendedBackgroundColorAttr,
	DefaultBackgroundColorAttr:       defaultBackgroundColorAttr,
	ExtendedUnderlineColorAttr:       extendedUnderlineColorAttr,
	DefaultUnderlineColorAttr:        defaultUnderlineColorAttr,
	BrightBlackForegroundColorAttr:   brightBlackForegroundColorAttr,
	BrightRedForegroundColorAttr:     brightRedForegroundColorAttr,
	BrightGreenForegroundColorAttr:   brightGreenForegroundColorAttr,
	BrightYellowForegroundColorAttr:  brightYellowForegroundColorAttr,
	BrightBlueForegroundColorAttr:    brightBlueForegroundColorAttr,
	BrightMagentaForegroundColorAttr: brightMagentaForegroundColorAttr,
	BrightCyanForegroundColorAttr:    brightCyanForegroundColorAttr,
	BrightWhiteForegroundColorAttr:   brightWhiteForegroundColorAttr,
	BrightBlackBackgroundColorAttr:   brightBlackBackgroundColorAttr,
	BrightRedBackgroundColorAttr:     brightRedBackgroundColorAttr,
	BrightGreenBackgroundColorAttr:   brightGreenBackgroundColorAttr,
	BrightYellowBackgroundColorAttr:  brightYellowBackgroundColorAttr,
	BrightBlueBackgroundColorAttr:    brightBlueBackgroundColorAttr,
	BrightMagentaBackgroundColorAttr: brightMagentaBackgroundColorAttr,
	BrightCyanBackgroundColorAttr:    brightCyanBackgroundColorAttr,
	BrightWhiteBackgroundColorAttr:   brightWhiteBackgroundColorAttr,
}
