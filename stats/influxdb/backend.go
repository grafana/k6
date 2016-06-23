package influxdb

import (
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/speedboat/stats"
)

type Backend struct {
	Client   client.Client
	Database string
}

func New(conf client.HTTPConfig, db string) (*Backend, error) {
	c, err := client.NewHTTPClient(conf)
	if err != nil {
		return nil, err
	}

	return &Backend{
		Client:   c,
		Database: db,
	}, nil
}

func NewFromURL(url string) (*Backend, error) {
	conf, db, err := parseURL(url)
	if err != nil {
		return nil, err
	}
	return New(conf, db)
}

func (b *Backend) Submit(batches [][]stats.Point) error {
	pb, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database: b.Database,
	})
	if err != nil {
		return err
	}

	for _, batch := range batches {
		for _, p := range batch {
			tags := make(map[string]string)
			for key, val := range p.Tags {
				tags[key] = fmt.Sprint(val)
			}
			fields := make(map[string]interface{})
			for key, val := range p.Values {
				fields[key] = val
			}
			pt, err := client.NewPoint(p.Stat.Name, tags, fields, p.Time)
			if err != nil {
				return err
			}

			pb.AddPoint(pt)
		}
	}

	if err := b.Client.Write(pb); err != nil {
		return err
	}

	return nil
}
