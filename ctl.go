package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ghodss/yaml"
	"github.com/loadimpact/k6/api/v1"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
	"os"
	"strconv"
)

var commandStatus = cli.Command{
	Name:      "status",
	Usage:     "Looks up the status of a running test",
	ArgsUsage: " ",
	Action:    actionStatus,
	Description: `Status will print the status of a running test to stdout in YAML format.

   Use the global --address/-a flag to specify the host to connect to; the
   default is port 6565 on the local machine.

   Endpoint: /v1/status`,
}

var commandStats = cli.Command{
	Name:      "stats",
	Usage:     "Prints stats for a running test",
	ArgsUsage: "[name]",
	Action:    actionStats,
	Description: `Stats will print metrics about a running test to stdout in YAML format.

   The result is a dictionary of metrics. If a name is specified, only that one
   metric is fetched, otherwise every metric is printed in no particular order.

   Endpoint: /v1/metrics
             /v1/metrics/:id`,
}

var commandScale = cli.Command{
	Name:      "scale",
	Usage:     "Scales a running test",
	ArgsUsage: "vus",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "max, m",
			Usage: "update the max number of VUs allowed",
		},
	},
	Action: actionScale,
	Description: `Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test. Use --max if you want to do this.

   Endpoint: /v1/status`,
}

var commandPause = cli.Command{
	Name:      "pause",
	Usage:     "Pauses a running test",
	ArgsUsage: " ",
	Action:    actionPause,
	Description: `Pause pauses a running test.

   Running VUs will finish their current iterations, then suspend themselves
   until woken by the test's resumption. A sleeping VU will consume no CPU
   cycles, but will still occupy memory.

   Endpoint: /v1/status`,
}

var commandStart = cli.Command{
	Name:      "start",
	Usage:     "Starts a paused test",
	ArgsUsage: " ",
	Action:    actionStart,
	Description: `Start starts a paused test.

   This is the opposite of the pause command, and will do nothing to an already
   running test.

   Endpoint: /v1/status`,
}

func dumpYAML(v interface{}) error {
	bytes, err := yaml.Marshal(v)
	if err != nil {
		log.WithError(err).Error("Serialization Error")
		return err
	}
	_, _ = os.Stdout.Write(bytes)
	return nil
}

func actionStatus(cc *cli.Context) error {
	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.Status()
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}
	return dumpYAML(status)
}

func actionStats(cc *cli.Context) error {
	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	if len(cc.Args()) > 0 {
		metric, err := client.Metric(cc.Args()[0])
		if err != nil {
			log.WithError(err).Error("Error")
			return err
		}
		return dumpYAML(metric)
	}

	metricList, err := client.Metrics()
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}

	metrics := make(map[string]stats.Metric)
	for _, metric := range metricList {
		metrics[metric.Name] = metric
	}
	return dumpYAML(metrics)
}

func actionScale(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	vus, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}

	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	update := lib.Status{VUs: null.IntFrom(vus)}
	if cc.IsSet("max") {
		update.VUsMax = null.IntFrom(cc.Int64("max"))
	}

	status, err := client.UpdateStatus(update)
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}
	return dumpYAML(status)
}

func actionCap(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}
	max, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}

	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.UpdateStatus(lib.Status{VUsMax: null.IntFrom(max)})
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}
	return dumpYAML(status)
}

func actionPause(cc *cli.Context) error {
	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.UpdateStatus(lib.Status{Running: null.BoolFrom(false)})
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}
	return dumpYAML(status)
}

func actionStart(cc *cli.Context) error {
	client, err := v1.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.UpdateStatus(lib.Status{Running: null.BoolFrom(true)})
	if err != nil {
		log.WithError(err).Error("Error")
		return err
	}
	return dumpYAML(status)
}
