package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/postman"
	"github.com/loadimpact/speedboat/simple"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/accumulate"
	"github.com/loadimpact/speedboat/stats/influxdb"
	"github.com/loadimpact/speedboat/stats/writer"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

const (
	typeURL     = "url"
	typeJS      = "js"
	typePostman = "postman"
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

var mVUs = stats.Stat{Name: "vus", Type: stats.GaugeType}

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
	case strings.HasPrefix(out, "influxdb+"):
		url := strings.TrimPrefix(out, "influxdb+")
		return influxdb.NewFromURL(url)
	default:
		return nil, errors.New("Unknown output destination")
	}
}

func parseStages(vus []string, total time.Duration) (stages []lib.TestStage, err error) {
	if len(vus) == 0 {
		return []lib.TestStage{
			lib.TestStage{Duration: total, StartVUs: 10, EndVUs: 10},
		}, nil
	}

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

func parseTags(lines []string) stats.Tags {
	tags := make(stats.Tags)
	for _, line := range lines {
		idx := strings.IndexAny(line, ":=")
		if idx == -1 {
			tags[line] = line
			continue
		}

		key := line[:idx]
		val := line[idx+1:]
		if key == "" {
			key = val
		}
		tags[key] = val
	}
	return tags
}

func guessType(arg string) string {
	switch {
	case strings.Contains(arg, "://"):
		return typeURL
	case strings.HasSuffix(arg, ".js"):
		return typeJS
	case strings.HasSuffix(arg, ".postman_collection.json"):
		return typePostman
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
	case typePostman:
		return postman.New(bytes)
	default:
		return nil, errors.New("Type ambiguous, please specify -t/--type")
	}
}

func action(cc *cli.Context) error {
	once := cc.Bool("once")

	if cc.IsSet("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	for _, out := range cc.StringSlice("out") {
		backend, err := parseBackend(out)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		stats.DefaultRegistry.Backends = append(stats.DefaultRegistry.Backends, backend)
	}

	var formatter writer.Formatter
	switch cc.String("format") {
	case "":
	case "json":
		formatter = writer.JSONFormatter{}
	case "prettyjson":
		formatter = writer.PrettyJSONFormatter{}
	case "yaml":
		formatter = writer.YAMLFormatter{}
	default:
		return cli.NewExitError("Unknown output format", 1)
	}

	stats.DefaultRegistry.ExtraTags = parseTags(cc.StringSlice("tag"))

	var summarizer *Summarizer
	if formatter != nil {
		filter := stats.MakeFilter(cc.StringSlice("exclude"), cc.StringSlice("select"))
		if cc.Bool("raw") {
			backend := &writer.Backend{
				Writer:    os.Stdout,
				Formatter: formatter,
			}
			backend.Filter = filter
			stats.DefaultRegistry.Backends = append(stats.DefaultRegistry.Backends, backend)
		} else {
			accumulator := accumulate.New()
			accumulator.Filter = filter
			accumulator.GroupBy = cc.StringSlice("group-by")
			stats.DefaultRegistry.Backends = append(stats.DefaultRegistry.Backends, accumulator)

			summarizer = &Summarizer{
				Accumulator: accumulator,
				Formatter:   formatter,
			}
		}
	}

	stages, err := parseStages(cc.StringSlice("vus"), cc.Duration("duration"))
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if once {
		stages = []lib.TestStage{
			lib.TestStage{Duration: 0, StartVUs: 1, EndVUs: 1},
		}
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

	if cc.Bool("plan") {
		data, err := yaml.Marshal(map[string]interface{}{
			"stages": stages,
		})
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		os.Stdout.Write(data)
		return nil
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
	if once {
		ctx, cancel = context.WithCancel(context.Background())
	}

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

	interval := cc.Duration("interval")
	if interval > 0 && summarizer != nil {
		go func() {
			ticker := time.NewTicker(interval)
			for {
				select {
				case <-ticker.C:
					if err := summarizer.Print(os.Stdout); err != nil {
						log.WithError(err).Error("Couldn't print statistics!")
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		quit := make(chan os.Signal)
		signal.Notify(quit)

		select {
		case <-quit:
			cancel()
		case <-ctx.Done():
		}
	}()

	if !cc.Bool("quiet") {
		log.WithFields(log.Fields{
			"at":     time.Now(),
			"length": t.TotalDuration(),
		}).Info("Starting test...")
	}

	if once {
		stats.Add(stats.Sample{Stat: &mVUs, Values: stats.Value(1)})

		vu, _ := vus.Pool.Get()
		if err := vu.RunOnce(ctx); err != nil {
			log.WithError(err).Error("Uncaught Error")
		}
	} else {
		vus.Start(ctx)
		scaleTo := pollVURamping(ctx, t)
	mainLoop:
		for {
			select {
			case num := <-scaleTo:
				vus.Scale(num)
				stats.Add(stats.Sample{
					Stat:   &mVUs,
					Values: stats.Value(float64(num)),
				})
			case <-ctx.Done():
				break mainLoop
			}
		}

		vus.Stop()
	}

	stats.Add(stats.Sample{Stat: &mVUs, Values: stats.Value(0)})
	stats.Submit()

	if summarizer != nil {
		if err := summarizer.Print(os.Stdout); err != nil {
			log.WithError(err).Error("Couldn't print statistics!")
		}
	}

	return nil
}

func main() {
	// Submit usage statistics for the closed beta
	invocation := Invocation{}
	invocationError := make(chan error, 1)

	go func() {
		// Set SUBMIT=false to prevent stat collection
		submitURL := os.Getenv("SB_SUBMIT")
		switch submitURL {
		case "false", "no":
			return
		case "":
			submitURL = "http://52.209.216.227:8080"
		}

		// Wait at most 2s for an invocation error to be reported
		select {
		case err := <-invocationError:
			invocation.Error = err.Error()
		case <-time.After(2 * time.Second):
		}

		// Submit stats to a specified server
		if err := invocation.Submit(submitURL); err != nil {
			log.WithError(err).Debug("Couldn't submit statistics")
		}
	}()

	// Free up -v and -h for our own flags
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help, ?"
	cli.AppHelpTemplate = helpTemplate

	// Bootstrap the app from commandline flags
	app := cli.NewApp()
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "plan",
			Usage: "Don't run anything, just show the test plan",
		},
		cli.BoolFlag{
			Name:  "once",
			Usage: "Run only a single test iteration, with one VU",
		},
		cli.StringFlag{
			Name:  "type, t",
			Usage: "Input file type, if not evident (url, js or postman)",
		},
		cli.StringSliceFlag{
			Name:  "vus, u",
			Usage: "Number of VUs to simulate",
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
		cli.StringFlag{
			Name:  "format, f",
			Usage: "Format for printed metrics (yaml, json, prettyjson)",
			Value: "yaml",
		},
		cli.DurationFlag{
			Name:  "interval, i",
			Usage: "Write periodic summaries",
		},
		cli.StringSliceFlag{
			Name:  "out, o",
			Usage: "Write metrics to a database",
		},
		cli.BoolFlag{
			Name:  "raw",
			Usage: "Instead of summaries, dump raw samples to stdout",
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
		cli.StringSliceFlag{
			Name:  "tag",
			Usage: "Additional metric tags",
		},
	}
	app.Before = func(cc *cli.Context) error {
		invocation.PopulateWithContext(cc)
		return nil
	}
	app.Action = action
	invocationError <- app.Run(os.Args)
}
