package api

// HTTPHeaderAPI is a single HTTP header.
type HTTPHeaderAPI struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPMessageSizeAPI are the sizes in bytes of the HTTP message header and body.
type HTTPMessageSizeAPI struct {
	Headers int64 `json:"headers"`
	Body    int64 `json:"body"`
}

// Total returns the total size in bytes of the HTTP message.
func (s HTTPMessageSizeAPI) Total() int64 {
	return s.Headers + s.Body
}

// RectAPI is a rectangle.
type RectAPI struct {
	X      float64 `js:"x"`
	Y      float64 `js:"y"`
	Width  float64 `js:"width"`
	Height float64 `js:"height"`
}
