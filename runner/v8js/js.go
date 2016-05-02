package v8js

import (
	"fmt"
)

func jsThrow(msg string) string {
	return fmt.Sprintf(`{"_error": "%s"}`, msg)
}
