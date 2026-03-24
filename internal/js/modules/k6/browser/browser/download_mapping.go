package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func mapDownloadEvent(vu moduleVU, event common.PageEvent) mapping {
	return mapDownload(vu, event.Download)
}

// mapDownload to the JS module.
func mapDownload(vu moduleVU, dl *common.Download) mapping {
	if dl == nil {
		return nil
	}
	return mapping{
		"url":               dl.URL,
		"suggestedFilename": dl.SuggestedFilename,
		"path": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return dl.Path()
			})
		},
		"failure": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return dl.Failure(), nil
			})
		},
		"saveAs": func(path string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, dl.SaveAs(path) //nolint:wrapcheck
			})
		},
		"cancel": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, dl.Cancel() //nolint:wrapcheck
			})
		},
		"page": func() mapping {
			return mapPage(vu, dl.Page())
		},
	}
}
