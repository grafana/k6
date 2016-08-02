// This will be removed or reworked in the final version.
// Stat submission for the closed beta.
package main

import (
	"encoding/json"
	"github.com/urfave/cli"
	"github.com/valyala/fasthttp"
	"time"
)

type Invocation struct {
	Args  []string `json:"args"`
	Flags struct {
		Plan     bool          `json:"plan"`
		Type     string        `json:"type"`
		VUs      []string      `json:"vus"`
		Duration time.Duration `json:"duration"`
		Verbose  bool          `json:"verbose"`
		Quiet    bool          `json:"quiet"`
		Format   string        `json:"format"`
		Interval time.Duration `json:"interval"`
		Out      []string      `json:"out"`
		Raw      bool          `json:"raw"`
		Select   []string      `json:"select"`
		Exclude  []string      `json:"exclude"`
		GroupBy  []string      `json:"group_by"`
		Tag      []string      `json:"tag"`
	} `json:"flags"`
	Error string `json:"error"`
}

func (i *Invocation) PopulateWithContext(cc *cli.Context) {
	i.Args = []string(cc.Args())
	i.Flags.Plan = cc.GlobalBool("plan")
	i.Flags.Type = cc.GlobalString("type")
	i.Flags.VUs = cc.GlobalStringSlice("vus")
	i.Flags.Duration = cc.GlobalDuration("duration")
	i.Flags.Verbose = cc.GlobalBool("verbose")
	i.Flags.Quiet = cc.GlobalBool("quiet")
	i.Flags.Format = cc.GlobalString("format")
	i.Flags.Interval = cc.GlobalDuration("interval")
	i.Flags.Out = cc.GlobalStringSlice("out")
	i.Flags.Raw = cc.GlobalBool("raw")
	i.Flags.Select = cc.GlobalStringSlice("select")
	i.Flags.Exclude = cc.GlobalStringSlice("exclude")
	i.Flags.GroupBy = cc.GlobalStringSlice("group-by")
	i.Flags.Tag = cc.GlobalStringSlice("tag")
}

func (i *Invocation) Submit(url string) error {
	body, err := json.Marshal(i)
	if err != nil {
		return err
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(url + "/invoked")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("text/json")
	req.SetBody(body)

	if err := fasthttp.Do(req, nil); err != nil {
		return err
	}

	return nil
}
