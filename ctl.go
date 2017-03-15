/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/api/v1"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
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
	ArgsUsage: " ",
	Action:    actionStats,
	Description: `Stats will print metrics about a running test to stdout in YAML format.

   The result is a dictionary of metrics, in no particular order.

   Endpoint: /v1/metrics`,
}

var commandScale = cli.Command{
	Name:      "scale",
	Usage:     "Scales a running test",
	ArgsUsage: "vus",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "vus, u",
			Usage: "update the number of running VUs",
		},
		cli.Int64Flag{
			Name:  "max, m",
			Usage: "update the max number of VUs allowed",
		},
	},
	Action: actionScale,
	Description: `Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test. To raise vus-max, use --max/-m.

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

var commandResume = cli.Command{
	Name:      "resume",
	Usage:     "Resumes a paused test",
	ArgsUsage: " ",
	Action:    actionResume,
	Description: `Resume resumes a paused test.

   This is the opposite of the pause command, and will do nothing to an already
   running test.

   Endpoint: /v1/status`,
}

func endpointURL(cc *cli.Context, endpoint string) string {
	return fmt.Sprintf("http://%s%s", cc.GlobalString("address"), endpoint)
}

func apiCall(cc *cli.Context, method, endpoint string, body []byte, dst interface{}) error {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, endpointURL(cc, endpoint), bodyReader)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		var envelope v1.ErrorResponse
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		return envelope.Errors[0]
	}

	return jsonapi.Unmarshal(data, dst)
}

func actionStatus(cc *cli.Context) error {
	var status v1.Status
	if err := apiCall(cc, "GET", "/v1/status", nil, &status); err != nil {
		return err
	}
	return dumpYAML(status)
}

func actionStats(cc *cli.Context) error {
	var metrics []v1.Metric
	if err := apiCall(cc, "GET", "/v1/metrics", nil, &metrics); err != nil {
		return err
	}
	output := make(map[string]v1.Metric)
	for _, m := range metrics {
		output[m.GetID()] = m
	}
	return dumpYAML(output)
}

func actionScale(cc *cli.Context) error {
	patch := v1.Status{
		VUs:    cliInt64(cc, "vus"),
		VUsMax: cliInt64(cc, "max"),
	}
	if !patch.VUs.Valid && !patch.VUsMax.Valid {
		log.Warn("Neither --vus/-u or --max/-m passed; doing doing nothing")
		return nil
	}

	body, err := jsonapi.Marshal(patch)
	if err != nil {
		log.WithError(err).Error("Serialization error")
		return err
	}

	var status v1.Status
	if err := apiCall(cc, "PATCH", "/v1/status", body, &status); err != nil {
		return err
	}
	return dumpYAML(status)
}

func actionPause(cc *cli.Context) error {
	body, err := jsonapi.Marshal(v1.Status{
		Paused: null.BoolFrom(true),
	})
	if err != nil {
		log.WithError(err).Error("Serialization error")
		return err
	}

	var status v1.Status
	if err := apiCall(cc, "PATCH", "/v1/status", body, &status); err != nil {
		return err
	}
	return dumpYAML(status)
}

func actionResume(cc *cli.Context) error {
	body, err := jsonapi.Marshal(v1.Status{
		Paused: null.BoolFrom(false),
	})
	if err != nil {
		log.WithError(err).Error("Serialization error")
		return err
	}

	var status v1.Status
	if err := apiCall(cc, "PATCH", "/v1/status", body, &status); err != nil {
		return err
	}
	return dumpYAML(status)
}
