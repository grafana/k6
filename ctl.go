package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ghodss/yaml"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/lib"
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

var commandScale = cli.Command{
	Name:      "scale",
	Usage:     "Scales a running test",
	ArgsUsage: "vus",
	Action:    actionScale,
	Description: `Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test, and should thus be done deliberately.

   Endpoint: /v1/status`,
}

var commandCap = cli.Command{
	Name:      "cap",
	Usage:     "Changes the VU cap for a running test",
	ArgsUsage: "max",
	Action:    actionCap,
	Description: `Cap will change the maximum number of VUs for a test.

   Because instantiating new VUs is a potentially very expensive operation,
   both in terms of CPU and RAM, you should be aware that you may see a bump in
   response times and skewed averages if you increase the cap during a running
   test.
   
   It's recommended to pause the test before creating a large number of VUs.
   
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
	client, err := api.NewClient(cc.GlobalString("address"))
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

	client, err := api.NewClient(cc.GlobalString("address"))
	if err != nil {
		log.WithError(err).Error("Couldn't create a client")
		return err
	}

	status, err := client.UpdateStatus(lib.Status{VUs: null.IntFrom(vus)})
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

	client, err := api.NewClient(cc.GlobalString("address"))
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
	client, err := api.NewClient(cc.GlobalString("address"))
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
	client, err := api.NewClient(cc.GlobalString("address"))
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
