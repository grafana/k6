package js

import (
	log "github.com/Sirupsen/logrus"
	logtest "github.com/Sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	// "math"
	"os"
	// "strconv"
	"testing"
	"time"
)

func TestNewVU(t *testing.T) {
	r := New("script", "1+1")
	_, err := r.NewVU()
	assert.NoError(t, err)
}

func TestNewVUInvalidJS(t *testing.T) {
	r := New("script", "aiugbauibeuifa")
	_, err := r.NewVU()
	assert.NoError(t, err)
}

func TestReconfigure(t *testing.T) {
	r := New("script", "1+1")
	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	vu.ID = 100
	vu.Iteration = 100

	vu.Reconfigure(1)
	assert.Equal(t, int64(1), vu.ID)
	assert.Equal(t, int64(0), vu.Iteration)
}

func TestRunOnceIncreasesIterations(t *testing.T) {
	r := New("script", "1+1")
	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	assert.Equal(t, int64(0), vu.Iteration)
	vu.RunOnce(context.Background())
	assert.Equal(t, int64(1), vu.Iteration)
}

func TestRunOnceInvalidJS(t *testing.T) {
	r := New("script", "diyfsybfbub")
	vu, err := r.NewVU()
	assert.NoError(t, err)
	err = vu.RunOnce(context.Background())
	assert.Error(t, err)
}

func TestAPILogDebug(t *testing.T) {
	r := New("script", `$log.debug("test");`)
	logger, hook := logtest.NewNullLogger()
	logger.Level = log.DebugLevel
	r.logger = logger

	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))

	e := hook.LastEntry()
	assert.NotNil(t, e)
	assert.Equal(t, log.DebugLevel, e.Level)
	assert.Equal(t, "test", e.Message)
	assert.Len(t, e.Data, 0)
}

func TestAPILogInfo(t *testing.T) {
	r := New("script", `$log.info("test");`)
	logger, hook := logtest.NewNullLogger()
	r.logger = logger

	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))

	e := hook.LastEntry()
	assert.NotNil(t, e)
	assert.Equal(t, log.InfoLevel, e.Level)
	assert.Equal(t, "test", e.Message)
	assert.Len(t, e.Data, 0)
}

func TestAPILogWarn(t *testing.T) {
	r := New("script", `$log.warn("test");`)
	logger, hook := logtest.NewNullLogger()
	r.logger = logger

	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))

	e := hook.LastEntry()
	assert.NotNil(t, e)
	assert.Equal(t, log.WarnLevel, e.Level)
	assert.Equal(t, "test", e.Message)
	assert.Len(t, e.Data, 0)
}

func TestAPILogError(t *testing.T) {
	r := New("script", `$log.error("test");`)
	logger, hook := logtest.NewNullLogger()
	r.logger = logger

	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))

	e := hook.LastEntry()
	assert.NotNil(t, e)
	assert.Equal(t, log.ErrorLevel, e.Level)
	assert.Equal(t, "test", e.Message)
	assert.Len(t, e.Data, 0)
}

func TestAPILogWithData(t *testing.T) {
	r := New("script", `$log.info("test", { a: 'hi', b: 123 });`)
	logger, hook := logtest.NewNullLogger()
	r.logger = logger

	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))

	e := hook.LastEntry()
	assert.NotNil(t, e)
	assert.Equal(t, log.InfoLevel, e.Level)
	assert.Equal(t, "test", e.Message)
	assert.Equal(t, log.Fields{"a": "hi", "b": int64(123)}, e.Data)
}

func TestAPIVUSleep1s(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `$vu.sleep(1);`)
	vu, _ := r.NewVU()

	startTime := time.Now()
	err := vu.RunOnce(context.Background())
	duration := time.Since(startTime)

	assert.NoError(t, err)

	// Allow 50ms margin for call overhead
	target := 1 * time.Second
	if duration < target || duration > target+(50*time.Millisecond) {
		t.Fatalf("Incorrect sleep duration: %s", duration)
	}
}

func TestAPIVUSleep01s(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `$vu.sleep(0.1);`)
	vu, _ := r.NewVU()

	startTime := time.Now()
	err := vu.RunOnce(context.Background())
	duration := time.Since(startTime)

	assert.NoError(t, err)

	// Allow 50ms margin for call overhead
	target := 100 * time.Millisecond
	if duration < target || duration > target+(50*time.Millisecond) {
		t.Fatalf("Incorrect sleep duration: %s", duration)
	}
}

func TestAPIVUID(t *testing.T) {
	r := New("script", `if ($vu.id() !== 100) { throw new Error("invalid ID"); }`)
	vu, _ := r.NewVU()
	vu.Reconfigure(100)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIVUIteration(t *testing.T) {
	r := New("script", `if ($vu.iteration() !== 1) { throw new Error("invalid iteration"); }`)
	vu, _ := r.NewVU()
	vu.Reconfigure(100)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPITestEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "hi")
	r := New("script", `if ($test.env("TEST_VAR") !== "hi") { throw new Error("assertion failed"); }`)
	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPITestEnvUndefined(t *testing.T) {
	os.Unsetenv("NOT_SET_VAR") // Just in case...
	r := New("script", `if ($test.env("NOT_SET_VAR") !== undefined) { throw new Error("assertion failed"); }`)
	vu, _ := r.NewVU()
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPITestAbort(t *testing.T) {
	r := New("script", `$test.abort();`)
	vu, _ := r.NewVU()
	assert.Panics(t, func() { vu.RunOnce(context.Background()) })
}

// func TestAPIHTTPSetMaxConnsPerHost(t *testing.T) {
// 	r := New("script", `$http.setMaxConnsPerHost(100);`)
// 	vu, _ := r.NewVU()
// 	assert.NoError(t, vu.RunOnce(context.Background()))
// 	assert.Equal(t, 100, vu.(*VU).Client.MaxConnsPerHost)
// }

// func TestAPIHTTPSetMaxConnsPerHostOverflow(t *testing.T) {
// 	r := New("script", `$http.setMaxConnsPerHost(`+strconv.FormatInt(math.MaxInt64, 10)+`);`)
// 	vu, _ := r.NewVU()
// 	assert.NoError(t, vu.RunOnce(context.Background()))
// 	assert.Equal(t, math.MaxInt32, vu.(*VU).Client.MaxConnsPerHost)
// }

// func TestAPIHTTPSetMaxConnsPerHostZero(t *testing.T) {
// 	r := New("script", `$http.setMaxConnsPerHost(0);`)
// 	vu, _ := r.NewVU()
// 	assert.Error(t, vu.RunOnce(context.Background()))
// }

// func TestAPIHTTPSetMaxConnsPerHostNegative(t *testing.T) {
// 	r := New("script", `$http.setMaxConnsPerHost(-1);`)
// 	vu, _ := r.NewVU()
// 	assert.Error(t, vu.RunOnce(context.Background()))
// }

// func TestAPIHTTPSetMaxConnsPerHostInvalid(t *testing.T) {
// 	r := New("script", `$http.setMaxConnsPerHost("qwerty");`)
// 	vu, _ := r.NewVU()
// 	assert.Error(t, vu.RunOnce(context.Background()))
// }

func TestAPIHTTPRequestReportsStats(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", "$http.get('http://httpbin.org/get');")
	vu, err := r.NewVU()
	assert.NoError(t, err)
	err = vu.RunOnce(context.Background())
	assert.NoError(t, err)

	mRequestsFound := false
	for _, p := range vu.(*VU).Collector.Batch {
		switch p.Stat {
		case &mRequests:
			mRequestsFound = true
			assert.Contains(t, p.Tags, "url")
			assert.Contains(t, p.Tags, "method")
			assert.Contains(t, p.Tags, "status")
			assert.Contains(t, p.Values, "duration")
		case &mErrors:
			assert.Fail(t, "Errors found")
		}
	}
	assert.True(t, mRequestsFound)
}

func TestAPIHTTPRequestErrorReportsStats(t *testing.T) {
	r := New("script", "$http.get('http://255.255.255.255/');")
	vu, err := r.NewVU()
	assert.NoError(t, err)
	err = vu.RunOnce(context.Background())
	assert.Error(t, err)

	mRequestsFound := false
	mErrorsFound := false
	for _, p := range vu.(*VU).Collector.Batch {
		switch p.Stat {
		case &mRequests:
			mRequestsFound = true
			assert.Contains(t, p.Tags, "url")
			assert.Contains(t, p.Tags, "method")
			assert.Contains(t, p.Tags, "status")
			assert.Contains(t, p.Values, "duration")
		case &mErrors:
			mErrorsFound = true
			assert.Contains(t, p.Tags, "url")
			assert.Contains(t, p.Tags, "method")
			assert.Contains(t, p.Tags, "status")
			assert.Contains(t, p.Values, "value")
		}
	}
	assert.True(t, mRequestsFound)
	assert.True(t, mErrorsFound)
}

func TestAPIHTTPRequestQuietReportsNoStats(t *testing.T) {
	r := New("script", "$http.get('http://255.255.255.255/', null, { quiet: true });")
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.Error(t, vu.RunOnce(context.Background()))
	assert.Len(t, vu.(*VU).Collector.Batch, 0)
}

func TestAPIHTTPRequestGET(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.get("http://httpbin.org/get")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestGETArgs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	data = $http.get("http://httpbin.org/get", {a: 'b', b: 2}).json()
	if (data.args.a !== 'b') {
		throw new Error("invalid args.a: " + data.args.a);
	}
	if (data.args.b !== '2') {
		throw new Error("invalid args.b: " + data.args.b);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestGETHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	data = $http.get("http://httpbin.org/get", null, { headers: { 'X-Test': 'hi' } }).json()
	if (data.headers['X-Test'] !== 'hi') {
		throw new Error("invalid X-Test header: " + data.headers['X-Test'])
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestGETRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.get("http://httpbin.org/redirect/6");
	if (res.status !== 302) {
		throw new Error("invalid response code: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestGETRedirectFollow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.get("http://httpbin.org/redirect/6", null, { follow: true });
	if (res.status !== 200) {
		throw new Error("invalid response code: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestGETRedirectFollowTooMany(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	$http.get("http://httpbin.org/redirect/15", null, { follow: true });
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.Error(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.head("http://httpbin.org/get")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	if (res.body !== "") {
		throw new Error("body not empty")
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestHEADWithArgsDoesntStickThemInTheBodyAndFail(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.head("http://httpbin.org/get", { a: 'b' })
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	if (res.body !== "") {
		throw new Error("body not empty")
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestPOST(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.post("http://httpbin.org/post")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestPOSTArgs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	data = $http.post("http://httpbin.org/post", { a: 'b' }).json()
	if (data.form.a !== 'b') {
		throw new Error("invalid form.a: " + data.form.a);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestPOSTBody(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	data = $http.post("http://httpbin.org/post", 'a=b').json()
	if (data.data !== 'a=b') {
		throw new Error("invalid data: " + data.data);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestPUT(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.put("http://httpbin.org/put")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestPATCH(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.patch("http://httpbin.org/patch")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestDELETE(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.delete("http://httpbin.org/delete")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}

func TestAPIHTTPRequestOPTIONS(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	r := New("script", `
	res = $http.options("http://httpbin.org/")
	if (res.status !== 200) {
		throw new Error("invalid status: " + res.status);
	}
	if (res.body !== "") {
		throw new Error("non-empty body: " + res.body);
	}
	`)
	vu, err := r.NewVU()
	assert.NoError(t, err)
	assert.NoError(t, vu.RunOnce(context.Background()))
}
