package global

var members = map[string]interface{}{
	"sleep": Sleep,
}

func New() map[string]interface{} {
	return members
}
