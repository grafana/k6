package browser

import (
	"github.com/grafana/xk6-browser/common"
)

func mapMouse(_ moduleVU, m *common.Mouse) mapping {
	return mapping{
		"click":    m.Click,
		"dblClick": m.DblClick,
		"down":     m.Down,
		"up":       m.Up,
		"move":     m.Move,
	}
}
