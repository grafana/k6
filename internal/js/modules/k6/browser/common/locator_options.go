package common

// GetByRoleOptions are the optional options fow when working with the
// GetByRole API.
type GetByRoleOptions struct {
	Checked       *bool   `json:"checked"`
	Disabled      *bool   `json:"disabled"`
	Exact         *bool   `json:"exact"`
	Expanded      *bool   `json:"expanded"`
	IncludeHidden *bool   `json:"includeHidden"`
	Level         *int64  `json:"level"`
	Name          *string `json:"name"`
	Pressed       *bool   `json:"pressed"`
	Selected      *bool   `json:"selected"`
}

// GetByAltTextOptions are the optional options for when working with the
// GetByAltText API.
type GetByAltTextOptions struct {
	Exact *bool `json:"exact"`
}
