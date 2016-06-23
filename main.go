package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/simple"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/influxdb"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"
)

const (
	typeURL = "url"
	typeJS  = "js"
)

// Help text template
const (
	helpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[options] {{end}}filename|url{{end}}
   {{if .Version}}{{if not .HideVersion}}
VERSION:
   {{.Version}}
   {{end}}{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}`
)

func pollVURamping(ctx context.Context, t lib.Test) <-chan int {
	ch := make(chan int)
	startTime := time.Now()

	go func() {
		defer close(ch)

		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				ch <- t.VUsAt(time.Since(startTime))
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

func parseBackend(out string) (stats.Backend, error) {
	switch {
	case out == "-":
		return stats.NewJSONBackend(os.Stdout), nil
	case strings.HasPrefix(out, "influxdb+"):
		url := strings.TrimPrefix(out, "influxdb+")
		return influxdb.NewFromURL(url)
	default:
		f, err := os.Create(out)
		if err != nil {
			return nil, err
		}
		return stats.NewJSONBackend(f), nil
	}
}

func guessType(arg string) string {
	switch {
	case strings.Contains(arg, "://"):
		return typeURL
	case strings.HasSuffix(arg, ".js"):
		return typeJS
	}
	return ""
}

func readAll(filename string) ([]byte, error) {
	if filename == "-" {
		return ioutil.ReadAll(os.Stdin)
	}

	return ioutil.ReadFile(filename)
}

func makeRunner(t lib.Test, filename, typ string) (lib.Runner, error) {
	if typ == typeURL {
		return simple.New(filename), nil
	}

	bytes, err := readAll(filename)
	if err != nil {
		return nil, err
	}

	switch typ {
	case typeJS:
		return js.New(filename, string(bytes)), nil
	default:
		return nil, errors.New("Type ambiguous, please specify -t/--type")
	}
}

func action(cc *cli.Context) error {
	if cc.IsSet("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	for _, out := range cc.StringSlice("metrics") {
		backend, err := parseBackend(out)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		stats.DefaultRegistry.Backends = append(stats.DefaultRegistry.Backends, backend)
	}

	t := lib.Test{
		Stages: []lib.TestStage{
			lib.TestStage{
				Duration: cc.Duration("duration"),
				StartVUs: cc.Int("vus"),
				EndVUs:   cc.Int("vus"),
			},
		},
	}

	var r lib.Runner
	switch len(cc.Args()) {
	case 0:
		cli.ShowAppHelp(cc)
		return nil
	case 1:
		filename := cc.Args()[0]
		typ := cc.String("type")
		if typ == "" {
			typ = guessType(filename)
		}

		if filename == "-" && typ == "" {
			return cli.NewExitError("Reading from stdin requires a -t/--type flag", 1)
		}

		runner, err := makeRunner(t, filename, typ)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		r = runner
	default:
		return cli.NewExitError("Too many arguments!", 1)
	}

	vus := lib.VUGroup{
		Pool: lib.VUPool{
			New: r.NewVU,
		},
		RunOnce: func(ctx context.Context, vu lib.VU) {
			if err := vu.RunOnce(ctx); err != nil {
				log.WithError(err).Error("Uncaught Error")
			}
		},
	}

	for i := 0; i < t.MaxVUs(); i++ {
		vu, err := vus.Pool.New()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		vus.Pool.Put(vu)
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.TotalDuration())

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				if err := stats.Submit(); err != nil {
					log.WithError(err).Error("[Couldn't submit stats]")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit)

	vus.Start(ctx)
	scaleTo := pollVURamping(ctx, t)
	mVUs := stats.Stat{Name: "vus", Type: stats.GaugeType}
mainLoop:
	for {
		select {
		case num := <-scaleTo:
			vus.Scale(num)
			stats.Add(stats.Point{
				Stat:   &mVUs,
				Values: stats.Value(float64(num)),
			})
		case <-quit:
			cancel()
		case <-ctx.Done():
			break mainLoop
		}
	}

	vus.Stop()

	return nil
}

func main() {
	// Free up -v and -h for our own flags
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help, ?"
	cli.AppHelpTemplate = helpTemplate
	// Bootstrap the app from commandline flags
	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "1.0.0-mvp1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Usage: "Input file type, if not evident (url or js)",
		},
		cli.IntFlag{
			Name:  "vus, u",
			Usage: "Number of VUs to simulate",
			Value: 10,
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "Test duration",
			Value: time.Duration(10) * time.Second,
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "More verbose output",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress the summary at the end of a test",
		},
		cli.StringSliceFlag{
			Name:  "metrics, m",
			Usage: "Write metrics to a file or database",
		},
	}
	app.Action = action
	app.Run(os.Args)
}
