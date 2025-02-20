package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

type PageEmulateMediaOptions struct {
	ColorScheme   ColorScheme   `json:"colorScheme"`
	Media         MediaType     `json:"media"`
	ReducedMotion ReducedMotion `json:"reducedMotion"`
}

type PageReloadOptions struct {
	WaitUntil LifecycleEvent `json:"waitUntil" js:"waitUntil"`
	Timeout   time.Duration  `json:"timeout"`
}

type PageScreenshotOptions struct {
	Clip           *page.Viewport `json:"clip"`
	Path           string         `json:"path"`
	Format         ImageFormat    `json:"format"`
	FullPage       bool           `json:"fullPage"`
	OmitBackground bool           `json:"omitBackground"`
	Quality        int64          `json:"quality"`
}

func NewPageEmulateMediaOptions(from *Page) *PageEmulateMediaOptions {
	return &PageEmulateMediaOptions{
		ColorScheme:   from.colorScheme,
		Media:         from.mediaType,
		ReducedMotion: from.reducedMotion,
	}
}

// Parse parses the page emulate media options.
func (o *PageEmulateMediaOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "colorScheme":
				o.ColorScheme = ColorScheme(opts.Get(k).String())
			case "media":
				o.Media = MediaType(opts.Get(k).String())
			case "reducedMotion":
				o.ReducedMotion = ReducedMotion(opts.Get(k).String())
			}
		}
	}
	return nil
}

func NewPageReloadOptions(defaultWaitUntil LifecycleEvent, defaultTimeout time.Duration) *PageReloadOptions {
	return &PageReloadOptions{
		WaitUntil: defaultWaitUntil,
		Timeout:   defaultTimeout,
	}
}

// Parse parses the page reload options.
func (o *PageReloadOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if l, ok := lifecycleEventToID[lifeCycle]; ok {
					o.WaitUntil = l
				} else {
					return fmt.Errorf("%q is not a valid lifecycle", lifeCycle)
				}
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func NewPageScreenshotOptions() *PageScreenshotOptions {
	return &PageScreenshotOptions{
		Clip:           nil,
		Path:           "",
		Format:         ImageFormatPNG,
		FullPage:       false,
		OmitBackground: false,
		Quality:        100,
	}
}

// Parse parses the page screenshot options.
func (o *PageScreenshotOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)
	formatSpecified := false
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "clip":
			var c map[string]float64
			if rt.ExportTo(obj.Get(k), &c) != nil {
				o.Clip = &page.Viewport{
					X:      c["x"],
					Y:      c["y"],
					Width:  c["width"],
					Height: c["height"],
					Scale:  1,
				}
			}
		case "fullPage":
			o.FullPage = obj.Get(k).ToBoolean()
		case "omitBackground":
			o.OmitBackground = obj.Get(k).ToBoolean()
		case "path":
			o.Path = obj.Get(k).String()
		case "quality":
			o.Quality = obj.Get(k).ToInteger()
		case "type":
			if f, ok := imageFormatToID[obj.Get(k).String()]; ok {
				o.Format = f
				formatSpecified = true
			}
		}
	}

	// Infer file format by path if format not explicitly specified (default is PNG)
	if o.Path != "" && !formatSpecified {
		if strings.HasSuffix(o.Path, ".jpg") || strings.HasSuffix(o.Path, ".jpeg") {
			o.Format = ImageFormatJPEG
		}
	}

	return nil
}
