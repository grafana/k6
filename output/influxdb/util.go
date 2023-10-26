package influxdb

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	client "github.com/influxdata/influxdb1-client/v2"
	"gopkg.in/guregu/null.v3"
)

func MakeClient(conf Config) (client.Client, error) {
	if strings.HasPrefix(conf.Addr.String, "udp://") {
		return client.NewUDPClient(client.UDPConfig{
			Addr:        strings.TrimPrefix(conf.Addr.String, "udp://"),
			PayloadSize: int(conf.PayloadSize.Int64),
		})
	}
	if conf.Addr.String == "" {
		conf.Addr = null.StringFrom("http://localhost:8086")
	}
	clientHTTPConfig := client.HTTPConfig{
		Addr:               conf.Addr.String,
		Username:           conf.Username.String,
		Password:           conf.Password.String,
		UserAgent:          "k6",
		InsecureSkipVerify: conf.Insecure.Bool,
	}
	if conf.Proxy.Valid {
		parsedProxyURL, err := url.Parse(conf.Proxy.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the http proxy URL: %w", err)
		}
		clientHTTPConfig.Proxy = http.ProxyURL(parsedProxyURL)
	}
	return client.NewHTTPClient(clientHTTPConfig)
}

func MakeBatchConfig(conf Config) client.BatchPointsConfig {
	if !conf.DB.Valid || conf.DB.String == "" {
		conf.DB = null.StringFrom("k6")
	}
	return client.BatchPointsConfig{
		Precision:        conf.Precision.String,
		Database:         conf.DB.String,
		RetentionPolicy:  conf.Retention.String,
		WriteConsistency: conf.Consistency.String,
	}
}

func checkDuplicatedTypeDefinitions(fieldKinds map[string]FieldKind, tag string) error {
	if _, found := fieldKinds[tag]; found {
		return fmt.Errorf("a tag name (%s) shows up more than once in InfluxDB field type configurations", tag)
	}
	return nil
}

// MakeFieldKinds reads the Config and returns a lookup map of tag names to
// the field type their values should be converted to.
func MakeFieldKinds(conf Config) (map[string]FieldKind, error) {
	fieldKinds := make(map[string]FieldKind)
	for _, tag := range conf.TagsAsFields {
		var fieldName, fieldType string
		s := strings.SplitN(tag, ":", 2)
		if len(s) == 1 {
			fieldName, fieldType = s[0], "string"
		} else {
			fieldName, fieldType = s[0], s[1]
		}

		err := checkDuplicatedTypeDefinitions(fieldKinds, fieldName)
		if err != nil {
			return nil, err
		}

		switch fieldType {
		case "string":
			fieldKinds[fieldName] = String
		case "bool":
			fieldKinds[fieldName] = Bool
		case "float":
			fieldKinds[fieldName] = Float
		case "int":
			fieldKinds[fieldName] = Int
		default:
			return nil, fmt.Errorf("an invalid type (%s) is specified for an InfluxDB field (%s)",
				fieldType, fieldName)
		}
	}

	return fieldKinds, nil
}
