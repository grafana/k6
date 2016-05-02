package global

import (
	"time"
)

func Sleep(t float64) {
	time.Sleep(time.Duration(t) * time.Second)
}
