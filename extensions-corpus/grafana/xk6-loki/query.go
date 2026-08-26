package loki

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type QueryType int

const (
	InstantQuery QueryType = iota
	RangeQuery
	LabelsQuery
	LabelValuesQuery
	SeriesQuery
)

func (t QueryType) Endpoint() string {
	switch t {
	case InstantQuery:
		return "/loki/api/v1/query"
	case RangeQuery:
		return "/loki/api/v1/query_range"
	case LabelsQuery:
		return "/loki/api/v1/labels"
	case LabelValuesQuery:
		return "/loki/api/v1/label/%s/values"
	case SeriesQuery:
		return "/loki/api/v1/series"
	default:
		return ""
	}
}

// Query contains all necessary fields to execute instant and range queries and print the results.
type Query struct {
	Type        QueryType
	QueryString string
	Start       time.Time
	End         time.Time
	Limit       int
	PathParams  []interface{}
}

func (q *Query) Endpoint() string {
	return fmt.Sprintf(q.Type.Endpoint(), q.PathParams...)
}

func (q *Query) Values() url.Values {
	v := url.Values{}

	if q.QueryString != "" {
		if q.Type == RangeQuery || q.Type == InstantQuery {
			v.Set("query", q.QueryString)
		}
		if q.Type == SeriesQuery {
			v.Set("match[]", q.QueryString)
		}
	}

	if q.Type == InstantQuery {
		if q.End.Unix() > 0 {
			v.Set("time", strconv.FormatInt(q.End.UnixNano(), 10))
		}
	} else {
		if q.Start.Unix() > 0 {
			v.Set("start", strconv.FormatInt(q.Start.UnixNano(), 10))
		}
		if q.End.Unix() > 0 {
			v.Set("end", strconv.FormatInt(q.End.UnixNano(), 10))
		}
	}

	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	return v
}

// SetInstant makes the Query an instant type
func (q *Query) SetInstant(time time.Time) {
	q.Start = time
	q.End = time
}
