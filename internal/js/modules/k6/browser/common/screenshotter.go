package common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// ScreenshotPersister is the type that all file persisters must implement. It's job is
// to persist a file somewhere, hiding the details of where and how from the caller.
type ScreenshotPersister interface {
	Persist(ctx context.Context, path string, data io.Reader) (err error)
}

// ImageFormat represents an image file format.
type ImageFormat string

// Valid image format options.
const (
	ImageFormatJPEG ImageFormat = "jpeg"
	ImageFormatPNG  ImageFormat = "png"
)

func (f ImageFormat) String() string {
	return imageFormatToString[f]
}

var imageFormatToString = map[ImageFormat]string{ //nolint:gochecknoglobals
	ImageFormatJPEG: "jpeg",
	ImageFormatPNG:  "png",
}

var imageFormatToID = map[string]ImageFormat{ //nolint:gochecknoglobals
	"jpeg": ImageFormatJPEG,
	"png":  ImageFormatPNG,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (f ImageFormat) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(imageFormatToString[f])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (f *ImageFormat) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return fmt.Errorf("unmarshalling image format: %w", err)
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*f = imageFormatToID[j]
	return nil
}

type screenshotter struct {
	ctx       context.Context
	persister ScreenshotPersister
	logger    *log.Logger
}

func newScreenshotter(
	ctx context.Context,
	sp ScreenshotPersister,
	logger *log.Logger,
) *screenshotter {
	return &screenshotter{ctx, sp, logger}
}

func (s *screenshotter) fullPageSize(p *Page) (*Size, error) {
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := p.frameManager.MainFrame().evaluate(s.ctx, mainWorld, opts, `
        () => {
            if (!document.body || !document.documentElement) {
                return null;
            }
            return {
                width: Math.max(
                    document.body.scrollWidth, document.documentElement.scrollWidth,
                    document.body.offsetWidth, document.documentElement.offsetWidth,
                    document.body.clientWidth, document.documentElement.clientWidth
                ),
                height: Math.max(
                    document.body.scrollHeight, document.documentElement.scrollHeight,
                    document.body.offsetHeight, document.documentElement.offsetHeight,
                    document.body.clientHeight, document.documentElement.clientHeight
                ),
            };
        }`)
	if err != nil {
		return nil, err
	}
	var size Size
	if err := convert(result, &size); err != nil {
		return nil, fmt.Errorf("converting result (%v of type %t) to size: %w", result, result, err)
	}

	return &size, nil
}

func (s *screenshotter) originalViewportSize(p *Page) (*Size, *Size, error) {
	originalViewportSize := p.viewportSize()
	viewportSize := originalViewportSize
	if viewportSize.Width != 0 || viewportSize.Height != 0 {
		return &viewportSize, &originalViewportSize, nil
	}

	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := p.frameManager.MainFrame().evaluate(s.ctx, mainWorld, opts, `
	() => (
		{ width: window.innerWidth, height: window.innerHeight }
	)`)
	if err != nil {
		return nil, nil, fmt.Errorf("getting viewport dimensions: %w", err)
	}

	var returnVal Size
	if err := convert(result, &returnVal); err != nil {
		return nil, nil, fmt.Errorf("unpacking window size: %w", err)
	}

	viewportSize.Width = returnVal.Width
	viewportSize.Height = returnVal.Height

	return &viewportSize, &originalViewportSize, nil
}

func (s *screenshotter) restoreViewport(p *Page, originalViewport *Size) error {
	if originalViewport != nil {
		return p.setViewportSize(originalViewport)
	}
	return p.resetViewport()
}

func (s *screenshotter) screenshot(
	sess session, doc, viewport *Rect, format ImageFormat, omitBackground bool, quality int64, path string,
) ([]byte, error) {
	var (
		buf  []byte
		clip *cdppage.Viewport
	)
	capture := cdppage.CaptureScreenshot()

	shouldSetDefaultBackground := omitBackground && format == "png"
	if shouldSetDefaultBackground {
		action := emulation.SetDefaultBackgroundColorOverride().
			WithColor(&cdp.RGBA{R: 0, G: 0, B: 0, A: 0})
		if err := action.Do(cdp.WithExecutor(s.ctx, sess)); err != nil {
			return nil, fmt.Errorf("setting screenshot background transparency: %w", err)
		}
	}

	// Add common options
	capture.WithQuality(quality)
	switch format {
	case ImageFormatJPEG:
		capture.WithFormat(cdppage.CaptureScreenshotFormatJpeg)
	default:
		capture.WithFormat(cdppage.CaptureScreenshotFormatPng)
	}

	visualViewportScale, visualViewportPageX, visualViewportPageY, err := getViewPortDimensions(s.ctx, sess, s.logger)
	if err != nil {
		return nil, err
	}

	if doc == nil {
		s := Size{
			Width:  viewport.Width / visualViewportScale,
			Height: viewport.Height / visualViewportScale,
		}.enclosingIntSize()
		doc = &Rect{
			X:      visualViewportPageX + viewport.X,
			Y:      visualViewportPageY + viewport.Y,
			Width:  s.Width,
			Height: s.Height,
		}
	}

	scale := 1.0
	if viewport != nil {
		scale = visualViewportScale
	}
	clip = &cdppage.Viewport{
		X:      doc.X,
		Y:      doc.Y,
		Width:  doc.Width,
		Height: doc.Height,
		Scale:  scale,
	}
	if clip.Width > 0 && clip.Height > 0 {
		capture = capture.WithClip(clip)
	}

	// Capture screenshot
	buf, err = capture.Do(cdp.WithExecutor(s.ctx, sess))
	if err != nil {
		return nil, fmt.Errorf("capturing screenshot: %w", err)
	}

	if shouldSetDefaultBackground {
		action := emulation.SetDefaultBackgroundColorOverride()
		if err := action.Do(cdp.WithExecutor(s.ctx, sess)); err != nil {
			return nil, fmt.Errorf("resetting screenshot background color: %w", err)
		}
	}

	// Save screenshot capture to file
	if path != "" {
		if err := s.persister.Persist(s.ctx, path, bytes.NewBuffer(buf)); err != nil {
			return nil, fmt.Errorf("persisting screenshot: %w", err)
		}
	}

	return buf, nil
}

func getViewPortDimensions(ctx context.Context, sess session, logger *log.Logger) (float64, float64, float64, error) {
	visualViewportScale := 1.0
	visualViewportPageX, visualViewportPageY := 0.0, 0.0

	// Add clip region
	//nolint:dogsled
	_, visualViewport, _, _, cssVisualViewport, _, err := cdppage.GetLayoutMetrics().Do(cdp.WithExecutor(ctx, sess))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("getting layout metrics for screenshot: %w", err)
	}

	// we had a null pointer panic cases, when visualViewport is nil
	// instead of the erroring out, we fallback to defaults and still try to do a screenshot
	switch {
	case cssVisualViewport != nil:
		visualViewportScale = cssVisualViewport.Scale
		visualViewportPageX = cssVisualViewport.PageX
		visualViewportPageY = cssVisualViewport.PageY
	case visualViewport != nil:
		visualViewportScale = visualViewport.Scale
		visualViewportPageX = visualViewport.PageX
		visualViewportPageY = visualViewport.PageY
	default:
		logger.Warnf(
			"Screenshotter::screenshot",
			"chrome browser returned nil on page.getLayoutMetrics, falling back to defaults for visualViewport "+
				"(scale: %v, pageX: %v, pageY: %v)."+
				"This is non-standard behavior, if possible please report this issue (with reproducible script) "+
				"to the https://go.k6.io/k6/js/modules/k6/browser/issues/1502.",
			visualViewportScale, visualViewportPageX, visualViewportPageY,
		)
	}

	return visualViewportScale, visualViewportPageX, visualViewportPageY, nil
}

func (s *screenshotter) screenshotElement(h *ElementHandle, opts *ElementHandleScreenshotOptions) ([]byte, error) {
	format := opts.Format
	viewportSize, originalViewportSize, err := s.originalViewportSize(h.frame.page)
	if err != nil {
		return nil, fmt.Errorf("getting original viewport size: %w", err)
	}

	err = h.waitAndScrollIntoViewIfNeeded(h.ctx, false, true, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("scrolling element into view: %w", err)
	}

	bbox, err := h.boundingBox()
	if err != nil {
		return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
	}
	if bbox.Width <= 0 {
		return nil, fmt.Errorf("node has 0 width")
	}
	if bbox.Height <= 0 {
		return nil, fmt.Errorf("node has 0 height")
	}

	var overriddenViewportSize *Size
	fitsViewport := bbox.Width <= viewportSize.Width && bbox.Height <= viewportSize.Height
	if !fitsViewport { //nolint:nestif
		overriddenViewportSize = Size{
			Width:  math.Max(viewportSize.Width, bbox.Width),
			Height: math.Max(viewportSize.Height, bbox.Height),
		}.enclosingIntSize()
		if err := h.frame.page.setViewportSize(overriddenViewportSize); err != nil {
			return nil, fmt.Errorf("setting viewport size to %s: %w",
				overriddenViewportSize, err)
		}
		err = h.waitAndScrollIntoViewIfNeeded(h.ctx, false, true, opts.Timeout)
		if err != nil {
			return nil, fmt.Errorf("scrolling element into view: %w", err)
		}
		bbox, err = h.boundingBox()
		if err != nil {
			return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
		}
		if bbox.Width <= 0 {
			return nil, fmt.Errorf("node has 0 width")
		}
		if bbox.Height <= 0 {
			return nil, fmt.Errorf("node has 0 height")
		}
	}

	scrollOffset, err := h.Evaluate(`() => { return {x: window.scrollX, y: window.scrollY};}`)
	if err != nil {
		return nil, fmt.Errorf("evaluating scroll offset: %w", err)
	}

	var returnVal Position
	if err := convert(scrollOffset, &returnVal); err != nil {
		return nil, fmt.Errorf("unpacking scroll offset: %w", err)
	}

	documentRect := bbox
	documentRect.X += returnVal.X
	documentRect.Y += returnVal.Y

	buf, err := s.screenshot(
		h.frame.page.session,
		documentRect.enclosingIntRect(),
		nil, // viewportRect
		format,
		opts.OmitBackground,
		opts.Quality,
		opts.Path,
	)
	if err != nil {
		return nil, err
	}
	if overriddenViewportSize != nil {
		if err := s.restoreViewport(h.frame.page, originalViewportSize); err != nil {
			return nil, fmt.Errorf("restoring viewport: %w", err)
		}
	}

	return buf, nil
}

func (s *screenshotter) screenshotPage(p *Page, opts *PageScreenshotOptions) ([]byte, error) {
	format := opts.Format

	// Infer file format by path
	if opts.Path != "" && opts.Format != "png" && opts.Format != "jpeg" {
		if strings.HasSuffix(opts.Path, ".jpg") || strings.HasSuffix(opts.Path, ".jpeg") {
			format = "jpeg"
		}
	}

	viewportSize, originalViewportSize, err := s.originalViewportSize(p)
	if err != nil {
		return nil, fmt.Errorf("getting original viewport size: %w", err)
	}

	if opts.FullPage { //nolint:nestif
		fullPageSize, err := s.fullPageSize(p)
		if err != nil {
			return nil, fmt.Errorf("getting full page size: %w", err)
		}
		documentRect := &Rect{
			X:      0,
			Y:      0,
			Width:  fullPageSize.Width,
			Height: fullPageSize.Height,
		}
		var overriddenViewportSize *Size
		fitsViewport := fullPageSize.Width <= viewportSize.Width && fullPageSize.Height <= viewportSize.Height
		if !fitsViewport {
			overriddenViewportSize = fullPageSize
			if err := p.setViewportSize(overriddenViewportSize); err != nil {
				return nil, fmt.Errorf("setting viewport size to %s: %w",
					overriddenViewportSize, err)
			}
		}
		if opts.Clip != nil {
			documentRect, err = s.trimClipToSize(&Rect{
				X:      opts.Clip.X,
				Y:      opts.Clip.Y,
				Width:  opts.Clip.Width,
				Height: opts.Clip.Height,
			}, &Size{Width: documentRect.Width, Height: documentRect.Height})
			if err != nil {
				return nil, fmt.Errorf("trimming clip to size: %w", err)
			}
		}

		buf, err := s.screenshot(p.session, documentRect, nil, format, opts.OmitBackground, opts.Quality, opts.Path)
		if err != nil {
			return nil, err
		}
		if overriddenViewportSize != nil {
			if err := s.restoreViewport(p, originalViewportSize); err != nil {
				return nil, fmt.Errorf("restoring viewport to %s: %w",
					originalViewportSize, err)
			}
		}
		return buf, nil
	}

	viewportRect := &Rect{
		X:      0,
		Y:      0,
		Width:  viewportSize.Width,
		Height: viewportSize.Height,
	}
	if opts.Clip != nil {
		viewportRect, err = s.trimClipToSize(&Rect{
			X:      opts.Clip.X,
			Y:      opts.Clip.Y,
			Width:  opts.Clip.Width,
			Height: opts.Clip.Height,
		}, viewportSize)
		if err != nil {
			return nil, fmt.Errorf("trimming clip to size: %w", err)
		}
	}
	return s.screenshot(p.session, nil, viewportRect, format, opts.OmitBackground, opts.Quality, opts.Path)
}

func (s *screenshotter) trimClipToSize(clip *Rect, size *Size) (*Rect, error) {
	p1 := Position{
		X: math.Max(0, math.Min(clip.X, size.Width)),
		Y: math.Max(0, math.Min(clip.Y, size.Height)),
	}
	p2 := Position{
		X: math.Max(0, math.Min(clip.X+clip.Width, size.Width)),
		Y: math.Max(0, math.Min(clip.Y+clip.Height, size.Height)),
	}
	result := Rect{
		X:      p1.X,
		Y:      p1.Y,
		Width:  p2.X - p1.X,
		Height: p2.Y - p1.Y,
	}
	if result.Width == 0 || result.Height == 0 {
		return nil, errors.New("clip area is either empty or outside the viewport")
	}
	return &result, nil
}
