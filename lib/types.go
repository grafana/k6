package lib

import (
	"gopkg.in/guregu/null.v3"
)

type Options struct {
	VUs      null.Int
	VUsMax   null.Int
	Duration null.String
}
