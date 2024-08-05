package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/pyroscope-go/upstream"
)

var errCloudTokenRequired = errors.New("please provide an authentication token. You can find it here: https://pyroscope.io/cloud")

const cloudHostnameSuffix = "pyroscope.cloud"

type Remote struct {
	cfg    Config
	jobs   chan *upstream.UploadJob
	client *http.Client
	logger Logger

	done chan struct{}
	wg   sync.WaitGroup

	flushWG sync.WaitGroup
}

type Config struct {
	AuthToken         string
	BasicAuthUser     string // http basic auth user
	BasicAuthPassword string // http basic auth password
	TenantID          string
	HTTPHeaders       map[string]string
	Threads           int
	Address           string
	Timeout           time.Duration
	Logger            Logger
}

type Logger interface {
	Infof(_ string, _ ...interface{})
	Debugf(_ string, _ ...interface{})
	Errorf(_ string, _ ...interface{})
}

func NewRemote(cfg Config) (*Remote, error) {
	r := &Remote{
		cfg:  cfg,
		jobs: make(chan *upstream.UploadJob, 20),
		client: &http.Client{
			Transport: &http.Transport{
				MaxConnsPerHost: cfg.Threads,
			},
			// Don't follow redirects
			// Since the go http client strips the Authorization header when doing redirects (eg http -> https)
			// https://github.com/golang/go/blob/a41763539c7ad09a22720a517a28e6018ca4db0f/src/net/http/client_test.go#L1764
			// making an authorized server return a 401
			// which is confusing since the user most likely already set up an API Key
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: cfg.Timeout,
		},
		logger: cfg.Logger,
		done:   make(chan struct{}),
	}

	// parse the upstream address
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}

	// authorize the token first
	if cfg.AuthToken == "" && requiresAuthToken(u) {
		return nil, errCloudTokenRequired
	}

	return r, nil
}

func (r *Remote) Start() {
	r.wg.Add(r.cfg.Threads)
	for i := 0; i < r.cfg.Threads; i++ {
		go r.handleJobs()
	}
}

func (r *Remote) Stop() {
	if r.done != nil {
		close(r.done)
	}

	// wait for uploading goroutines exit
	r.wg.Wait()
}

func (r *Remote) Upload(j *upstream.UploadJob) {
	r.flushWG.Add(1)
	select {
	case r.jobs <- j:
	default:
		r.flushWG.Done()
		r.logger.Errorf("remote upload queue is full, dropping a profile job")
	}
}

func (r *Remote) Flush() {
	if r.done == nil {
		return
	}
	r.flushWG.Wait()
}

func (r *Remote) uploadProfile(j *upstream.UploadJob) error {
	u, err := url.Parse(r.cfg.Address)
	if err != nil {
		return fmt.Errorf("url parse: %v", err)
	}

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("profile", "profile.pprof")
	if err != nil {
		return err
	}
	fw.Write(j.Profile)
	if j.PrevProfile != nil {
		fw, err = writer.CreateFormFile("prev_profile", "profile.pprof")
		if err != nil {
			return err
		}
		fw.Write(j.PrevProfile)
	}
	if j.SampleTypeConfig != nil {
		fw, err = writer.CreateFormFile("sample_type_config", "sample_type_config.json")
		if err != nil {
			return err
		}
		b, err := json.Marshal(j.SampleTypeConfig)
		if err != nil {
			return err
		}
		fw.Write(b)
	}
	writer.Close()

	q := u.Query()
	q.Set("name", j.Name)
	// TODO: I think these should be renamed to startTime / endTime
	q.Set("from", strconv.FormatInt(j.StartTime.UnixNano(), 10))
	q.Set("until", strconv.FormatInt(j.EndTime.UnixNano(), 10))
	q.Set("spyName", j.SpyName)
	q.Set("sampleRate", strconv.Itoa(int(j.SampleRate)))
	q.Set("units", j.Units)
	q.Set("aggregationType", j.AggregationType)

	u.Path = path.Join(u.Path, "ingest")
	u.RawQuery = q.Encode()

	r.logger.Debugf("uploading at %s", u.String())
	// new a request for the job
	request, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return fmt.Errorf("new http request: %v", err)
	}
	contentType := writer.FormDataContentType()
	r.logger.Debugf("content type: %s", contentType)
	request.Header.Set("Content-Type", contentType)
	// request.Header.Set("Content-Type", "binary/octet-stream+"+string(j.Format))

	if r.cfg.AuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+r.cfg.AuthToken)
	} else if r.cfg.BasicAuthUser != "" && r.cfg.BasicAuthPassword != "" {
		request.SetBasicAuth(r.cfg.BasicAuthUser, r.cfg.BasicAuthPassword)
	}
	if r.cfg.TenantID != "" {
		request.Header.Set("X-Scope-OrgID", r.cfg.TenantID)
	}
	for k, v := range r.cfg.HTTPHeaders {
		request.Header.Set(k, v)
	}

	// do the request and get the response
	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("do http request: %v", err)
	}
	defer response.Body.Close()

	// read all the response body
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %v", err)
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("failed to upload. server responded with statusCode: '%d' and body: '%s'", response.StatusCode, string(respBody))
	}

	return nil
}

// handle the jobs
func (r *Remote) handleJobs() {
	for {
		select {
		case <-r.done:
			r.wg.Done()
			return
		case job := <-r.jobs:
			r.safeUpload(job)
			r.flushWG.Done()
		}
	}
}

func requiresAuthToken(u *url.URL) bool {
	return strings.HasSuffix(u.Host, cloudHostnameSuffix)
}

// do safe upload
func (r *Remote) safeUpload(job *upstream.UploadJob) {
	defer func() {
		if catch := recover(); catch != nil {
			r.logger.Errorf("recover stack: %v: %v", catch, string(debug.Stack()))
		}
	}()

	// update the profile data to server
	if err := r.uploadProfile(job); err != nil {
		r.logger.Errorf("upload profile: %v", err)
	}
}
