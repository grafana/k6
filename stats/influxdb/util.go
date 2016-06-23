package influxdb

import (
	"errors"
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/speedboat/stats"
	neturl "net/url"
)

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

func makeInfluxPoint(p stats.Point) (*client.Point, error) {
	tags := make(map[string]string)
	for key, val := range p.Tags {
		tags[key] = fmt.Sprint(val)
	}
	fields := make(map[string]interface{})
	for key, val := range p.Values {
		fields[key] = val
	}
	return client.NewPoint(p.Stat.Name, tags, fields, p.Time)
}
