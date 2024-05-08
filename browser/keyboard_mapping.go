package browser

import "github.com/grafana/xk6-browser/common"

func mapKeyboard(_ moduleVU, kb *common.Keyboard) mapping {
	return mapping{
		"up":         kb.Up,
		"down":       kb.Down,
		"press":      kb.Press,
		"type":       kb.Type,
		"insertText": kb.InsertText,
	}
}
