package api

type Error struct {
	Title string `json:"title"`
}

type ErrorResponse struct {
	Errors []Error `json:"errors"`
}
