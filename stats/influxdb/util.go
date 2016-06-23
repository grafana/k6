package influxdb

import (
	"errors"
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
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
