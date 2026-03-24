package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestPageOnDownload(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())

	tb.withHandler("/download-page", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
    <a id="download-link" href="/file" download="test-file.txt">Download</a>
</body>
</html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/file", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=test-file.txt")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, err := w.Write([]byte("hello download"))
		require.NoError(t, err)
	})

	p := tb.NewPage(&common.BrowserContextOptions{
		AcceptDownloads: true,
	})

	_, err := p.Goto(tb.url("/download-page"), &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad,
		Timeout:   common.DefaultTimeout,
	})
	require.NoError(t, err)

	// Register download handler before clicking.
	done := make(chan struct{})
	var gotDownload *common.Download

	err = p.On(common.PageEventDownload, func(ev common.PageEvent) error {
		gotDownload = ev.Download
		close(done)
		return nil
	})
	require.NoError(t, err)

	clickOpts := common.NewFrameClickOptions(p.Timeout())
	clickOpts.Force = true
	clickOpts.NoWaitAfter = true
	err = p.Click("#download-link", clickOpts)
	require.NoError(t, err)

	<-done

	require.NotNil(t, gotDownload)
	assert.Contains(t, gotDownload.URL(), "/file")
	assert.Equal(t, "test-file.txt", gotDownload.SuggestedFilename())
}

func TestPageOnDownloadMapping(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())

	tb.withHandler("/download-mapping-page", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
    <a id="download-link" href="/file" download="test-file.txt">Download</a>
</body>
</html>`)
		require.NoError(t, err)
	})
	tb.withHandler("/file", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=test-file.txt")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, err := w.Write([]byte("hello download"))
		require.NoError(t, err)
	})

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext({acceptDownloads: true});
		const page = await context.newPage();

		await page.goto('%s', {waitUntil: 'domcontentloaded'});

		const downloadPromise = page.waitForEvent('download');
		await page.evaluate(() => document.getElementById('download-link').click());
		const download = await downloadPromise;

		const result = {
			url: download.url(),
			suggestedFilename: download.suggestedFilename(),
		};

		await page.close();

		return JSON.stringify(result);
	`, tb.url("/download-mapping-page"))
	require.NoError(t, err)

	got := k6test.ToPromise(t, gv)

	var result struct {
		URL               string `json:"url"`
		SuggestedFilename string `json:"suggestedFilename"`
	}
	err = json.Unmarshal([]byte(got.Result().String()), &result)
	require.NoError(t, err)

	assert.Contains(t, result.URL, "/file")
	assert.Equal(t, "test-file.txt", result.SuggestedFilename)
}

func TestPageOnDownloadCancel(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())

	tb.withHandler("/download-cancel-page", func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
    <a id="download-link" href="/slow-file" download="big-file.bin">Download</a>
</body>
</html>`)
		require.NoError(t, err)
	})
	// Serve a slow download that gives us time to cancel.
	tb.withHandler("/slow-file", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=big-file.bin")
		w.Header().Set("Content-Type", "application/octet-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for i := 0; i < 100; i++ {
			_, _ = w.Write(make([]byte, 1024))
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	})

	p := tb.NewPage(&common.BrowserContextOptions{
		AcceptDownloads: true,
	})

	_, err := p.Goto(tb.url("/download-cancel-page"), &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad,
		Timeout:   common.DefaultTimeout,
	})
	require.NoError(t, err)

	// Register download handler before clicking.
	done := make(chan struct{})
	var gotDownload *common.Download

	err = p.On(common.PageEventDownload, func(ev common.PageEvent) error {
		gotDownload = ev.Download
		close(done)
		return nil
	})
	require.NoError(t, err)

	clickOpts := common.NewFrameClickOptions(p.Timeout())
	clickOpts.Force = true
	clickOpts.NoWaitAfter = true
	err = p.Click("#download-link", clickOpts)
	require.NoError(t, err)

	<-done

	require.NotNil(t, gotDownload)

	// Cancel the in-progress download via CDP.
	require.NoError(t, gotDownload.Cancel())

	// The download should report failure as canceled.
	assert.Equal(t, "canceled", gotDownload.Failure())
}
