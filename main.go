package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/js"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/loadimpact/speedboat/sampler/influxdb"
	"github.com/loadimpact/speedboat/sampler/stream"
	"github.com/loadimpact/speedboat/simple"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	typeURL = "url"
	typeYML = "yml"
	typeJS  = "js"
)

// Configure the global logger.
func configureLogging(c *cli.Context) {
	log.SetLevel(log.InfoLevel)
	if c.GlobalBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}
}

// Configure the global sampler.
func configureSampler(c *cli.Context) {
	sampler.DefaultSampler.OnError = func(err error) {
		log.WithError(err).Error("[Sampler error]")
	}

	for _, output := range c.GlobalStringSlice("metrics") {
		parts := strings.SplitN(output, "+", 2)
		switch parts[0] {
		case "influxdb":
			out, err := influxdb.NewFromURL(parts[1])
			if err != nil {
				log.WithError(err).Fatal("Couldn't create InfluxDB client")
			}
			sampler.DefaultSampler.Outputs = append(sampler.DefaultSampler.Outputs, out)
		default:
			var writer io.WriteCloser
			switch output {
			case "stdout", "-":
				writer = os.Stdout
			default:
				file, err := os.Create(output)
				if err != nil {
					log.WithError(err).Fatal("Couldn't create output file")
				}
				writer = file
			}

			var out sampler.Output
			switch c.GlobalString("format") {
			case "json":
				out = &stream.JSONOutput{Output: writer}
			case "csv":
				out = &stream.CSVOutput{Output: writer}
			default:
				log.Fatal("Unknown output format")
			}
			sampler.DefaultSampler.Outputs = append(sampler.DefaultSampler.Outputs, out)
		}
	}
}

func guessType(arg string) string {
	switch {
	case strings.Contains(arg, "://"):
		return typeURL
	case strings.HasSuffix(arg, ".js"):
		return typeJS
	case strings.HasSuffix(arg, ".yml"):
		return typeYML
	}
	return ""
}

func parse(cc *cli.Context) (conf Config, err error) {
	if len(cc.Args()) == 0 {
		return conf, errors.New("Nothing to do!")
	}

	conf.VUs = cc.Int("vus")
	conf.Duration = cc.Duration("duration").String()

	arg := cc.Args()[0]
	argType := cc.String("type")
	if argType == "" {
		argType = guessType(arg)
	}

	switch argType {
	case typeYML:
		bytes, err := ioutil.ReadFile(cc.Args()[0])
		if err != nil {
			return conf, errors.New("Couldn't read config file")
		}
		if err := yaml.Unmarshal(bytes, &conf); err != nil {
			return conf, errors.New("Couldn't parse config file")
		}
	case typeURL:
		conf.URL = arg
	case typeJS:
		conf.Script = arg
	default:
		return conf, errors.New("Unsure of what to do, try specifying --type")
	}

	return conf, nil
}

func headlessController(c context.Context, t *lib.Test) <-chan int {
	ch := make(chan int)

	go func() {
		defer close(ch)

		select {
		case ch <- t.VUsAt(0):
		case <-c.Done():
			return
		}

		startTime := time.Now()
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				ch <- t.VUsAt(time.Since(startTime))
			case <-c.Done():
				return
			}
		}
	}()

	return ch
}

func action(cc *cli.Context) error {
	if len(cc.Args()) == 0 {
		cli.ShowAppHelp(cc)
		return nil
	}

	conf, err := parse(cc)
	if err != nil {
		log.WithError(err).Fatal("Invalid arguments; see --help")
	}

	t, err := conf.MakeTest()
	if err != nil {
		log.WithError(err).Fatal("Configuration error")
	}

	// Inspect the test to find a suitable runner; additional ones can easily be added
	var runner lib.Runner
	switch {
	case t.Script == "":
		runner = simple.New(t)
	case strings.HasSuffix(t.Script, ".js"):
		src, err := ioutil.ReadFile(t.Script)
		if err != nil {
			log.WithError(err).Fatal("Couldn't read script")
		}
		runner = js.New(t, t.Script, string(src))
	default:
		log.Fatal("No suitable runner found!")
	}

	// Context that expires at the end of the test
	ctx, cancel := context.WithTimeout(context.Background(), t.TotalDuration())

	// Configure the VU logger
	logger := &log.Logger{
		Out:       os.Stderr,
		Level:     log.DebugLevel,
		Formatter: &log.TextFormatter{},
	}
	ctx = lib.WithLogger(ctx, logger)

	// Store metrics unless the --quiet flag is specified
	quiet := cc.Bool("quiet")
	sampler.DefaultSampler.Accumulate = !quiet

	// Commit metrics to any configured backends once per second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				commitMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Use a "headless controller" to scale VUs by polling the test ramp
	mVUs := sampler.Gauge("vus")
	vus := []context.CancelFunc{}
	for scale := range headlessController(ctx, &t) {
		for i := len(vus); i < scale; i++ {
			log.WithField("id", i).Debug("Spawning VU")
			vuCtx, vuCancel := context.WithCancel(ctx)
			vus = append(vus, vuCancel)

			go func(ctx context.Context) {
				defer func() {
					if v := recover(); v != nil {
						switch err := v.(type) {
						case lib.FlowControl:
							switch err {
							case lib.AbortTest:
								log.Error("Test aborted")
								cancel()
							}
						default:
							panic(err)
						}
					}
				}()

				vu, err := runner.NewVU()
				if err != nil {
					log.WithError(err).Error("Couldn't spawn VU")
					return
				}

				vu.Reconfigure(int64(i))
				for {
					select {
					case <-ctx.Done():
						return
					default:
						if err := vu.RunOnce(ctx); err != nil {
							log.WithError(err).Error("Script error")
						}
					}
				}
			}(vuCtx)
		}
		for i := len(vus); i > scale; i-- {
			log.WithField("id", i-1).Debug("Dropping VU")
			vus[i-1]()
			vus = vus[:i-1]
		}
		mVUs.Int(len(vus))
	}

	// Wait until the end of the test
	<-ctx.Done()

	// Print and commit final metrics
	if !quiet {
		printMetrics()
	}
	commitMetrics()
	closeMetrics()

	return nil
}

func main() {
	// Free up -v and -h for our own flags
	cli.VersionFlag.Name = "version"
	cli.HelpFlag.Name = "help, ?"
	cli.AppHelpTemplate = `NAME:
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

	// Bootstrap using action-registered commandline flags
	app := cli.NewApp()
	app.Commands = nil
	app.Name = "speedboat"
	app.Usage = "A next-generation load generator"
	app.Version = "1.0.0-mvp1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Usage: "Input file type, if not evident (url, yml or js)",
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
		cli.StringFlag{
			Name:  "format, f",
			Usage: "Metric output format (json or csv)",
			Value: "json",
		},
	}
	app.Before = func(c *cli.Context) error {
		configureLogging(c)
		configureSampler(c)
		return nil
	}
	app.Action = action
	app.Run(os.Args)
}
