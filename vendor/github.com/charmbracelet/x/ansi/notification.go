package ansi

// Notify sends a desktop notification using iTerm's OSC 9.
//
//	OSC 9 ; Mc ST
//	OSC 9 ; Mc BEL
//
// Where Mc is the notification body.
//
// See: https://iterm2.com/documentation-escape-codes.html
func Notify(s string) string {
	return "\x1b]9;" + s + "\x07"
}
