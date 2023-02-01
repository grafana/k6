package common

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/dop251/goja"
)

type screenshotter struct {
	ctx context.Context
}

func newScreenshotter(ctx context.Context) *screenshotter {
	return &screenshotter{ctx}
}

func (s *screenshotter) fullPageSize(p *Page) (*Size, error) {
	rt := p.vu.Runtime()
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := p.frameManager.MainFrame().evaluate(s.ctx, mainWorld, opts, rt.ToValue(`
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
        }`))
	if err != nil {
		return nil, err
	}
	v, ok := result.(goja.Value)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", result)
	}
	o := v.ToObject(rt)

	return &Size{
		Width:  o.Get("width").ToFloat(),
		Height: o.Get("height").ToFloat(),
	}, nil
}

func (s *screenshotter) originalViewportSize(p *Page) (*Size, *Size, error) {
	rt := p.vu.Runtime()
	originalViewportSize := p.viewportSize()
	viewportSize := originalViewportSize
	if viewportSize.Width != 0 || viewportSize.Height != 0 {
		return &viewportSize, &originalViewportSize, nil
	}

	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := p.frameManager.MainFrame().evaluate(s.ctx, mainWorld, opts, rt.ToValue(`
	() => (
		{ width: window.innerWidth, height: window.innerHeight }
	)`))
	if err != nil {
		return nil, nil, fmt.Errorf("getting viewport dimensions: %w", err)
	}
	r, ok := result.(goja.Object)
	if !ok {
		return nil, nil, fmt.Errorf("cannot convert to goja object: %w", err)
	}
	viewportSize.Width = r.Get("width").ToFloat()
	viewportSize.Height = r.Get("height").ToFloat()
	return &viewportSize, &originalViewportSize, nil
}

func (s *screenshotter) restoreViewport(p *Page, originalViewport *Size) error {
	if originalViewport != nil {
		return p.setViewportSize(originalViewport)
	}
	return p.resetViewport()
}

//nolint:funlen,cyclop
func (s *screenshotter) screenshot(
	sess session, doc, viewport *Rect, format ImageFormat, omitBackground bool, quality int64, path string,
) (*[]byte, error) {
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
	// nolint:exhaustive
	switch format {
	case ImageFormatJPEG:
		capture.WithFormat(cdppage.CaptureScreenshotFormatJpeg)
	default:
		capture.WithFormat(cdppage.CaptureScreenshotFormatPng)
	}

	// Add clip region
	//nolint:dogsled
	_, visualViewport, _, _, _, _, err := cdppage.GetLayoutMetrics().Do(cdp.WithExecutor(s.ctx, sess))
	if err != nil {
		return nil, fmt.Errorf("getting layout metrics for screenshot: %w", err)
	}

	if doc == nil {
		s := Size{
			Width:  viewport.Width / visualViewport.Scale,
			Height: viewport.Height / visualViewport.Scale,
		}.enclosingIntSize()
		doc = &Rect{
			X:      visualViewport.PageX + viewport.X,
			Y:      visualViewport.PageY + viewport.Y,
			Width:  s.Width,
			Height: s.Height,
		}
	}

	scale := 1.0
	if viewport != nil {
		scale = visualViewport.Scale
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
	// TODO: we should not write to disk here but put it on some queue for async disk writes
	if path != "" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating screenshot directory %q: %w", dir, err)
		}
		if err := ioutil.WriteFile(path, buf, 0o644); err != nil {
			return nil, fmt.Errorf("saving screenshot to %q: %w", path, err)
		}
	}

	return &buf, nil
}

func (s *screenshotter) screenshotElement(h *ElementHandle, opts *ElementHandleScreenshotOptions) (*[]byte, error) {
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
	if !fitsViewport {
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

	documentRect := bbox
	rt := h.execCtx.vu.Runtime()
	scrollOffset := h.Evaluate(rt.ToValue(`() => { return {x: window.scrollX, y: window.scrollY};}`))
	switch s := scrollOffset.(type) {
	case goja.Value:
		documentRect.X += s.ToObject(rt).Get("x").ToFloat()
		documentRect.Y += s.ToObject(rt).Get("y").ToFloat()
	}

	buf, err := s.screenshot(h.frame.page.session, documentRect.enclosingIntRect(), nil, format, opts.OmitBackground, opts.Quality, opts.Path)
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

func (s *screenshotter) screenshotPage(p *Page, opts *PageScreenshotOptions) (*[]byte, error) {
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

	if opts.FullPage {
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
