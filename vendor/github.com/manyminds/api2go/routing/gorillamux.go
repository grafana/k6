// +build gorillamux,!gingonic,!echo

package routing

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type gorillamuxRouter struct {
	router *mux.Router
}

func (gm gorillamuxRouter) Handler() http.Handler {
	return gm.router
}

func (gm gorillamuxRouter) Handle(protocol, route string, handler HandlerFunc) {
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, mux.Vars(r), make(map[string]interface{}))
	}

	// The request path will have parameterized segments indicated as :name.  Convert
	// that notation to the {name} notation used by Gorilla mux.
	orig := strings.Split(route, "/")
	var mod []string
	for _, s := range orig {
		if len(s) > 0 && s[0] == ':' {
			s = fmt.Sprintf("{%s}", s[1:])
		}
		mod = append(mod, s)
	}
	modroute := strings.Join(mod, "/")

	gm.router.HandleFunc(modroute, wrappedHandler).Methods(protocol)
}

//Gorilla creates a new api2go router to use with the Gorilla mux framework
func Gorilla(gm *mux.Router) Routeable {
	return &gorillamuxRouter{router: gm}
}
