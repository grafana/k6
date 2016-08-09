package postman

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"golang.org/x/net/context"
	"strings"
)

type ErrorWithLineNumber struct {
	Wrapped error
	Line    int
}

func (e ErrorWithLineNumber) Error() string {
	return fmt.Sprintf("%s (line %d)", e.Wrapped.Error(), e.Line)
}

type Runner struct {
	Collection Collection
}

type VU struct {
	Runner *Runner
}

func New(source []byte) (*Runner, error) {
	var collection Collection
	if err := json.Unmarshal(source, &collection); err != nil {
		switch e := err.(type) {
		case *json.SyntaxError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		case *json.UnmarshalTypeError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		}
		return nil, err
	}

	return &Runner{
		Collection: collection,
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	return &VU{Runner: r}, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	for _, item := range u.Runner.Collection.Item {
		if err := u.runItem(item, u.Runner.Collection.Auth); err != nil {
			return err
		}
	}

	return nil
}

func (u *VU) runItem(i Item, a Auth) error {
	if i.Auth.Type != "" {
		a = i.Auth
	}

	if i.Request.URL != "" {
		log.WithField("url", i.Request.URL).Info("Request!")
	}

	for _, item := range i.Item {
		if err := u.runItem(item, a); err != nil {
			return err
		}
	}

	return nil
}
