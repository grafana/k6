package influxdb

import (
	"errors"
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/speedboat/sampler"
	neturl "net/url"
	"sync"
)

type Output struct {
	Client   client.Client
	Database string

	currentBatch client.BatchPoints
	batchMutex   sync.Mutex
}

func New(conf client.HTTPConfig, db string) (*Output, error) {
	c, err := client.NewHTTPClient(conf)
	if err != nil {
		return nil, err
	}

	return &Output{
		Client:   c,
		Database: db,
	}, nil
}

func NewFromURL(url string) (*Output, error) {
	conf, db, err := parseURL(url)
	if err != nil {
		return nil, err
	}
	return New(conf, db)
}

func parseURL(url string) (conf client.HTTPConfig, db string, err error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return conf, db, err
	}

	if u.Path == "" || u.Path == "/" {
		return conf, db, errors.New("No InfluxDB database specified")
	}
	db = u.Path[1:]

	conf.Addr = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.User != nil {
		conf.Username = u.User.Username()
		conf.Password, _ = u.User.Password()
	}
	return conf, db, nil
}

func (o *Output) Write(m *sampler.Metric, e *sampler.Entry) error {
	o.batchMutex.Lock()
	defer o.batchMutex.Unlock()

	if o.currentBatch == nil {
		batch, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database: o.Database,
		})
		if err != nil {
			return err
		}
		o.currentBatch = batch
	}

	tags := make(map[string]string)
	for key, value := range e.Fields {
		tags[key] = fmt.Sprint(value)
	}
	fields := map[string]interface{}{"value": e.Value}

	point, err := client.NewPoint(m.Name, tags, fields, e.Time)
	if err != nil {
		return err
	}
	o.currentBatch.AddPoint(point)
	return nil
}

func (o *Output) Commit() error {
	o.batchMutex.Lock()
	defer o.batchMutex.Unlock()

	err := o.Client.Write(o.currentBatch)
	o.currentBatch = nil
	return err
}
