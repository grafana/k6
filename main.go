package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/simple"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/accumulate"
	"github.com/loadimpact/speedboat/stats/influxdb"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

const (
	typeURL = "url"
	typeJS  = "js"
)

// Help text template
const helpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[options] {{end}}filename|url{{end}}
   {{if .Version}}{{if not .HideVersion}}
VERSION:
   {{.Version}}
   {{end}}{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

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

func parseStages(vus []string, total time.Duration) (stages []lib.TestStage, err error) {
	accountedTime := time.Duration(0)
	fluidStages := []int{}
	for i, spec := range vus {
		parts := strings.SplitN(spec, ":", 2)
		countParts := strings.SplitN(parts[0], "-", 2)

		stage := lib.TestStage{}

		// An absent first part means keep going from the last stage's end
		// If it's the first stage, just start with 0
		if countParts[0] == "" {
			if i > 0 {
				stage.StartVUs = stages[i-1].EndVUs
			}
		} else {
			start, err := strconv.ParseInt(countParts[0], 10, 64)
			if err != nil {
				return stages, err
			}
			stage.StartVUs = int(start)
		}

		// If an end is specified, use that, otherwise keep the VU level constant
		if len(countParts) > 1 && countParts[1] != "" {
			end, err := strconv.ParseInt(countParts[1], 10, 64)
			if err != nil {
				return stages, err
			}
			stage.EndVUs = int(end)
		} else {
			stage.EndVUs = stage.StartVUs
		}

		// If a time is specified, use that, otherwise mark the stage as "fluid", allotting it an
		// even slice of what time remains after all fixed stages are accounted for (may be 0)
		if len(parts) > 1 {
			duration, err := time.ParseDuration(parts[1])
			if err != nil {
				return stages, err
			}
			stage.Duration = duration
			accountedTime += duration
		} else {
			fluidStages = append(fluidStages, i)
		}

		stages = append(stages, stage)
	}

	// We're ignoring fluid stages if the fixed stages already take up all the allotted time
	// Otherwise, evenly divide the test's remaining time between all fluid stages
	if len(fluidStages) > 0 && accountedTime < total {
		fluidDuration := (total - accountedTime) / time.Duration(len(fluidStages))
		for _, i := range fluidStages {
			stage := stages[i]
			stage.Duration = fluidDuration
			stages[i] = stage
		}
	}

	return stages, nil
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

	var accumulator *accumulate.Backend
	if !cc.Bool("quiet") {
		accumulator = accumulate.New()
		for _, stat := range cc.StringSlice("select") {
			if stat == "*" {
				accumulator.Only = make(map[string]bool)
				break
			}
			accumulator.Only[stat] = true
		}
		for _, stat := range cc.StringSlice("exclude") {
			accumulator.Exclude[stat] = true
		}
		for _, tag := range cc.StringSlice("group-by") {
			accumulator.GroupBy = append(accumulator.GroupBy, tag)
		}
		stats.DefaultRegistry.Backends = append(stats.DefaultRegistry.Backends, accumulator)
	}

	stages, err := parseStages(cc.StringSlice("vus"), cc.Duration("duration"))
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	t := lib.Test{Stages: stages}

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
			typ = typeJS
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

	stats.Add(stats.Point{Stat: &mVUs, Values: stats.Value(0)})
	stats.Submit()

	if accumulator != nil {
		for stat, dimensions := range accumulator.Data {
			switch stat.Type {
			case stats.CounterType:
				for dname, dim := range dimensions {
					e := log.WithField("count", stats.ApplyIntent(dim.Sum(), stat.Intent))
					if len(dimensions) == 1 {
						e.Infof("Metric: %s", stat.Name)
					} else {
						e.Infof("Metric: %s.%s", stat.Name, *dname)
					}
				}
			case stats.GaugeType:
				for dname, dim := range dimensions {
					last := dim.Last
					if last == 0 {
						continue
					}

					e := log.WithField("val", stats.ApplyIntent(last, stat.Intent))
					if len(dimensions) == 1 {
						e.Infof("Metric: %s", stat.Name)
					} else {
						e.Infof("Metric: %s.%s", stat.Name, *dname)
					}
				}
			case stats.HistogramType:
				first := true
				for dname, dim := range dimensions {
					if first {
						log.WithField("count", len(dim.Values)).Infof("Metric: %s", stat.Name)
						first = false
					}
					log.WithFields(log.Fields{
						"min": stats.ApplyIntent(dim.Min(), stat.Intent),
						"max": stats.ApplyIntent(dim.Max(), stat.Intent),
						"avg": stats.ApplyIntent(dim.Avg(), stat.Intent),
						"med": stats.ApplyIntent(dim.Med(), stat.Intent),
					}).Infof("  - %s", *dname)
				}
			}
		}
	}

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
		cli.StringSliceFlag{
			Name:  "vus, u",
			Usage: "Number of VUs to simulate",
			Value: &cli.StringSlice{"10"},
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
		cli.StringSliceFlag{
			Name:  "select, s",
			Usage: "Include only named metrics",
		},
		cli.StringSliceFlag{
			Name:  "exclude, e",
			Usage: "Exclude named metrics",
		},
		cli.StringSliceFlag{
			Name:  "group-by, g",
			Usage: "Group metrics by tags",
		},
	}
	app.Action = action
	app.Run(os.Args)
}
