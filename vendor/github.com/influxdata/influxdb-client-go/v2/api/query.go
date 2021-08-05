// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	http2 "github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/api/query"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/influxdata/influxdb-client-go/v2/internal/log"
	ilog "github.com/influxdata/influxdb-client-go/v2/log"
)

const (
	stringDatatype       = "string"
	doubleDatatype       = "double"
	boolDatatype         = "boolean"
	longDatatype         = "long"
	uLongDatatype        = "unsignedLong"
	durationDatatype     = "duration"
	base64BinaryDataType = "base64Binary"
	timeDatatypeRFC      = "dateTime:RFC3339"
	timeDatatypeRFCNano  = "dateTime:RFC3339Nano"
)

// QueryAPI provides methods for performing synchronously flux query against InfluxDB server.
type QueryAPI interface {
	// QueryRaw executes flux query on the InfluxDB server and returns complete query result as a string with table annotations according to dialect
	QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error)
	// Query executes flux query on the InfluxDB server and returns QueryTableResult which parses streamed response into structures representing flux table parts
	Query(ctx context.Context, query string) (*QueryTableResult, error)
}

// NewQueryAPI returns new query client for querying buckets belonging to org
func NewQueryAPI(org string, service http2.Service) QueryAPI {
	return &queryAPI{
		org:         org,
		httpService: service,
	}
}

// queryAPI implements QueryAPI interface
type queryAPI struct {
	org         string
	httpService http2.Service
	url         string
	lock        sync.Mutex
}

func (q *queryAPI) QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error) {
	queryURL, err := q.queryURL()
	if err != nil {
		return "", err
	}
	queryType := domain.QueryTypeFlux
	qr := domain.Query{Query: query, Type: &queryType, Dialect: dialect}
	qrJSON, err := json.Marshal(qr)
	if err != nil {
		return "", err
	}
	if log.Level() >= ilog.DebugLevel {
		log.Debugf("Query: %s", qrJSON)
	}
	var body string
	perror := q.httpService.DoPostRequest(ctx, queryURL, bytes.NewReader(qrJSON), func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
	},
		func(resp *http.Response) error {
			if resp.Header.Get("Content-Encoding") == "gzip" {
				resp.Body, err = gzip.NewReader(resp.Body)
				if err != nil {
					return err
				}
			}
			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			body = string(respBody)
			return nil
		})
	if perror != nil {
		return "", perror
	}
	return body, nil
}

// DefaultDialect return flux query Dialect with full annotations (datatype, group, default), header and comma char as a delimiter
func DefaultDialect() *domain.Dialect {
	annotations := []domain.DialectAnnotations{domain.DialectAnnotationsDatatype, domain.DialectAnnotationsGroup, domain.DialectAnnotationsDefault}
	delimiter := ","
	header := true
	return &domain.Dialect{
		Annotations: &annotations,
		Delimiter:   &delimiter,
		Header:      &header,
	}
}

func (q *queryAPI) Query(ctx context.Context, query string) (*QueryTableResult, error) {
	var queryResult *QueryTableResult
	queryURL, err := q.queryURL()
	if err != nil {
		return nil, err
	}
	queryType := domain.QueryTypeFlux
	qr := domain.Query{Query: query, Type: &queryType, Dialect: DefaultDialect()}
	qrJSON, err := json.Marshal(qr)
	if err != nil {
		return nil, err
	}
	if log.Level() >= ilog.DebugLevel {
		log.Debugf("Query: %s", qrJSON)
	}
	perror := q.httpService.DoPostRequest(ctx, queryURL, bytes.NewReader(qrJSON), func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
	},
		func(resp *http.Response) error {
			if resp.Header.Get("Content-Encoding") == "gzip" {
				resp.Body, err = gzip.NewReader(resp.Body)
				if err != nil {
					return err
				}
			}
			csvReader := csv.NewReader(resp.Body)
			csvReader.FieldsPerRecord = -1
			queryResult = &QueryTableResult{Closer: resp.Body, csvReader: csvReader}
			return nil
		})
	if perror != nil {
		return queryResult, perror
	}
	return queryResult, nil
}

func (q *queryAPI) queryURL() (string, error) {
	if q.url == "" {
		u, err := url.Parse(q.httpService.ServerAPIURL())
		if err != nil {
			return "", err
		}
		u.Path = path.Join(u.Path, "query")

		params := u.Query()
		params.Set("org", q.org)
		u.RawQuery = params.Encode()
		q.lock.Lock()
		q.url = u.String()
		q.lock.Unlock()
	}
	return q.url, nil
}

// QueryTableResult parses streamed flux query response into structures representing flux table parts
// Walking though the result is done by repeatedly calling Next() until returns false.
// Actual flux table info (columns with names, data types, etc) is returned by TableMetadata() method.
// Data are acquired by Record() method.
// Preliminary end can be caused by an error, so when Next() return false, check Err() for an error
type QueryTableResult struct {
	io.Closer
	csvReader     *csv.Reader
	tablePosition int
	tableChanged  bool
	table         *query.FluxTableMetadata
	record        *query.FluxRecord
	err           error
}

// TablePosition returns actual flux table position in the result, or -1 if no table was found yet
// Each new table is introduced by an annotation in csv
func (q *QueryTableResult) TablePosition() int {
	if q.table != nil {
		return q.table.Position()
	}
	return -1
}

// TableMetadata returns actual flux table metadata
func (q *QueryTableResult) TableMetadata() *query.FluxTableMetadata {
	return q.table
}

// TableChanged returns true if last call of Next() found also new result table
// Table information is available via TableMetadata method
func (q *QueryTableResult) TableChanged() bool {
	return q.tableChanged
}

// Record returns last parsed flux table data row
// Use Record methods to access value and row properties
func (q *QueryTableResult) Record() *query.FluxRecord {
	return q.record
}

type parsingState int

const (
	parsingStateNormal parsingState = iota
	parsingStateAnnotation
	parsingStateNameRow
	parsingStateError
)

// Next advances to next row in query result.
// During the first time it is called, Next creates also table metadata
// Actual parsed row is available through Record() function
// Returns false in case of end or an error, otherwise true
func (q *QueryTableResult) Next() bool {
	var row []string
	// set closing query in case of preliminary return
	closer := func() {
		if err := q.Close(); err != nil {
			message := err.Error()
			if q.err != nil {
				message = fmt.Sprintf("%s,%s", message, q.err.Error())
			}
			q.err = errors.New(message)
		}
	}
	defer func() {
		closer()
	}()
	parsingState := parsingStateNormal
	q.tableChanged = false
	dataTypeAnnotationFound := false
readRow:
	row, q.err = q.csvReader.Read()
	if q.err == io.EOF {
		q.err = nil
		return false
	}
	if q.err != nil {
		return false
	}

	if len(row) <= 1 {
		goto readRow
	}
	if len(row[0]) > 0 && row[0][0] == '#' {
		if parsingState == parsingStateNormal {
			q.table = query.NewFluxTableMetadata(q.tablePosition)
			q.tablePosition++
			q.tableChanged = true
			for i := range row[1:] {
				q.table.AddColumn(query.NewFluxColumn(i))
			}
			parsingState = parsingStateAnnotation
		}
	}
	if q.table == nil {
		q.err = errors.New("parsing error, annotations not found")
		return false
	}
	if len(row)-1 != len(q.table.Columns()) {
		q.err = fmt.Errorf("parsing error, row has different number of columns than the table: %d vs %d", len(row)-1, len(q.table.Columns()))
		return false
	}
	switch row[0] {
	case "":
		switch parsingState {
		case parsingStateAnnotation:
			if !dataTypeAnnotationFound {
				q.err = errors.New("parsing error, datatype annotation not found")
				return false
			}
			parsingState = parsingStateNameRow
			fallthrough
		case parsingStateNameRow:
			if row[1] == "error" {
				parsingState = parsingStateError
			} else {
				for i, n := range row[1:] {
					if q.table.Column(i) != nil {
						q.table.Column(i).SetName(n)
					}
				}
				parsingState = parsingStateNormal
			}
			goto readRow
		case parsingStateError:
			var message string
			if len(row) > 1 && len(row[1]) > 0 {
				message = row[1]
			} else {
				message = "unknown query error"
			}
			reference := ""
			if len(row) > 2 && len(row[2]) > 0 {
				reference = fmt.Sprintf(",%s", row[2])
			}
			q.err = fmt.Errorf("%s%s", message, reference)
			return false
		}
		values := make(map[string]interface{})
		for i, v := range row[1:] {
			if q.table.Column(i) != nil {
				values[q.table.Column(i).Name()], q.err = toValue(stringTernary(v, q.table.Column(i).DefaultValue()), q.table.Column(i).DataType(), q.table.Column(i).Name())
				if q.err != nil {
					return false
				}
			}
		}
		q.record = query.NewFluxRecord(q.table.Position(), values)
	case "#datatype":
		dataTypeAnnotationFound = true
		for i, d := range row[1:] {
			if q.table.Column(i) != nil {
				q.table.Column(i).SetDataType(d)
			}
		}
		goto readRow
	case "#group":
		for i, g := range row[1:] {
			if q.table.Column(i) != nil {
				q.table.Column(i).SetGroup(g == "true")
			}
		}
		goto readRow
	case "#default":
		for i, c := range row[1:] {
			if q.table.Column(i) != nil {
				q.table.Column(i).SetDefaultValue(c)
			}
		}
		goto readRow
	}
	// don't close query
	closer = func() {}
	return true
}

// Err returns an error raised during flux query response parsing
func (q *QueryTableResult) Err() error {
	return q.err
}

// Close reads remaining data and closes underlying Closer
func (q *QueryTableResult) Close() error {
	var err error
	for err == nil {
		_, err = q.csvReader.Read()
	}
	return q.Closer.Close()
}

// stringTernary returns a if not empty, otherwise b
func stringTernary(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

// toValues converts s into type by t
func toValue(s, t, name string) (interface{}, error) {
	if s == "" {
		return nil, nil
	}
	switch t {
	case stringDatatype:
		return s, nil
	case timeDatatypeRFC:
		return time.Parse(time.RFC3339, s)
	case timeDatatypeRFCNano:
		return time.Parse(time.RFC3339Nano, s)
	case durationDatatype:
		return time.ParseDuration(s)
	case doubleDatatype:
		return strconv.ParseFloat(s, 64)
	case boolDatatype:
		if strings.ToLower(s) == "false" {
			return false, nil
		}
		return true, nil
	case longDatatype:
		return strconv.ParseInt(s, 10, 64)
	case uLongDatatype:
		return strconv.ParseUint(s, 10, 64)
	case base64BinaryDataType:
		return base64.StdEncoding.DecodeString(s)
	default:
		return nil, fmt.Errorf("%s has unknown data type %s", name, t)
	}
}
