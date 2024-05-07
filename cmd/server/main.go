package main

import (
	"context"
	"github.com/kataras/iris/v12"
	"github.com/liuxd6825/k6server/js"
	"github.com/liuxd6825/k6server/lib"
	"github.com/liuxd6825/k6server/loader"
	"github.com/liuxd6825/k6server/metrics"
	"github.com/sirupsen/logrus"
	"net/url"
)

const script = `
function a(){
	return 0
}

export default function() {
	app.get("/test", function(ictx){
		ictx.json({"name":"test", "address":"cc", time: Date.now()})
	})
}
`

func main() {
	app := iris.New()
	if err := Run(app); err != nil {
		panic(err)
	}
	if err := app.Run(iris.Addr(":8080")); err != nil {
		panic(err)
	}
}

func Run(app *iris.Application) error {
	app.Get("/test1", func(ictx iris.Context) {
		ictx.JSON(map[string]any{
			"name": "lxd",
		})
	})
	piState := getTestPreInitState(logrus.New())
	runner, err := js.New(
		piState,
		&loader.SourceData{
			URL:  &url.URL{Path: "/script.js"},
			Data: []byte(script),
		},
		nil,
	)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vu, err := runner.NewVU(ctx, 1, 10, make(chan metrics.SampleContainer, 1), func(vu lib.VU) error {
		return vu.GetRuntime().Set("app", app)
	})
	if err != nil {
		return err
	}

	params := &lib.VUActivationParams{RunContext: ctx}
	err = vu.Activate(params).RunOnce()
	return err
}

func getTestPreInitState(tb logrus.FieldLogger) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	return &lib.TestPreInitState{
		Logger:         tb,
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}
}
