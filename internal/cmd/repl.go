package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/js"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type cmdRepl struct {
	gs *state.GlobalState
}

func getCmdRepl(gs *state.GlobalState) *cobra.Command {
	c := &cmdRepl{
		gs: gs,
	}

	replCmd := &cobra.Command{
		Use:   "repl",
		Short: "Run a REPL",
		RunE:  c.repl,
	}

	return replCmd
}

const script = `
import http from 'k6/http';
import { sleep } from 'k6';

export default function () {
    while (true) { sleep(1); }
}
`

func (c *cmdRepl) repl(cmd *cobra.Command, args []string) (err error) {
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	rtOptions := lib.RuntimeOptions{CompatibilityMode: null.StringFrom("base")}

	runner, err := js.New(
		&lib.TestPreInitState{
			Logger:         log.New(),
			BuiltinMetrics: builtinMetrics,
			Registry:       registry,
			RuntimeOptions: rtOptions,
			Usage:          usage.New(),
		},
		&loader.SourceData{
			URL:  &url.URL{Path: "base", Scheme: "file"},
			Data: []byte(script),
		},
		loader.CreateFilesystems(c.gs.FS),
	)
	if err != nil {
		panic(err)
	}

	vu, err := runner.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	if err != nil {
		panic(err)
	}

	originalVU := vu.(*js.VU)

	go func() {
		err := vu.Activate(&lib.VUActivationParams{RunContext: context.Background()}).RunOnce()
		if err != nil {
			panic(err)
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		result, err := originalVU.Runtime.RunString(input)
		if err == nil {
			fmt.Println(result.String())
		} else {
			fmt.Println(err.Error())
		}
	}
}
