package js

import (
	_ "embed"
)

//go:embed query_all.js
// QueryAll queries all the elements in a given scope (document by default).
var QueryAll string
