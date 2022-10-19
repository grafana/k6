package js

import (
	_ "embed"
)

// QueryAll queries all the elements in a given scope (document by default).
//
//go:embed query_all.js
var QueryAll string
