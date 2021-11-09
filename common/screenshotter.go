/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
	k6common "go.k6.io/k6/js/common"
)

type screenshotter struct {
	ctx context.Context
}

func newScreenshotter(ctx context.Context) *screenshotter {
	return &screenshotter{ctx}
}

func (s *screenshotter) fullPageSize(p *Page) (*Size, error) {
	rt := k6common.GetRuntime(s.ctx)
	result, err := p.frameManager.mainFrame.mainExecutionContext.evaluate(s.ctx, true, true, rt.ToValue(`
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
	r, ok := result.(goja.Value)
	if !ok {
		return nil, fmt.Errorf("cannot convert result to goja value")
	}
	o := r.ToObject(rt)
	return &Size{
		Width:  o.Get("width").ToFloat(),
		Height: o.Get("height").ToFloat(),
	}, nil
}

func (s *screenshotter) originalViewportSize(p *Page) (*Size, *Size, error) {
	rt := k6common.GetRuntime(s.ctx)
	originalViewportSize := p.viewportSize()
	viewportSize := originalViewportSize
	if viewportSize.Width != 0 || viewportSize.Height != 0 {
		return &viewportSize, &originalViewportSize, nil
	}
	result, err := p.frameManager.mainFrame.mainExecutionContext.evaluate(s.ctx, true, true, rt.ToValue(`
	() => (
		{ width: window.innerWidth, height: window.innerHeight }
	)`))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot evaluate page function: %w", err)
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

func (s *screenshotter) screenshot(session *Session, documentRect *Rect, viewportRect *Rect, format ImageFormat, omitBackground bool, quality int64, path string) (*[]byte, error) {
	var (
		buf  []byte
		clip *cdppage.Viewport
	)
	capture := cdppage.CaptureScreenshot()

	shouldSetDefaultBackground := omitBackground && format == "png"
	if shouldSetDefaultBackground {
		action := emulation.SetDefaultBackgroundColorOverride().
			WithColor(&cdp.RGBA{R: 0, G: 0, B: 0, A: 0})
		if err := action.Do(cdp.WithExecutor(s.ctx, session)); err != nil {
			return nil, fmt.Errorf("cannot set screenshot background transparency: %w", err)
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

	// Add clip region
	_, visualViewport, _, _, _, _, err := cdppage.GetLayoutMetrics().Do(cdp.WithExecutor(s.ctx, session))
	if err != nil {
		return nil, fmt.Errorf("cannot get layout metrics for screenshot: %w", err)
	}

	if documentRect == nil {
		s := Size{
			Width:  viewportRect.Width / visualViewport.Scale,
			Height: viewportRect.Height / visualViewport.Scale,
		}.enclosingIntSize()
		documentRect = &Rect{
			X:      visualViewport.PageX + viewportRect.X,
			Y:      visualViewport.PageY + viewportRect.Y,
			Width:  s.Width,
			Height: s.Height,
		}
	}

	scale := 1.0
	if viewportRect != nil {
		scale = visualViewport.Scale
	}
	clip = &cdppage.Viewport{
		X:      documentRect.X,
		Y:      documentRect.Y,
		Width:  documentRect.Width,
		Height: documentRect.Height,
		Scale:  scale,
	}
	if clip.Width > 0 && clip.Height > 0 {
		capture = capture.WithClip(clip)
	}

	// Capture screenshot
	buf, err = capture.Do(cdp.WithExecutor(s.ctx, session))
	if err != nil {
		return nil, fmt.Errorf("cannot capture screenshot: %w", err)
	}

	if shouldSetDefaultBackground {
		action := emulation.SetDefaultBackgroundColorOverride()
		if err := action.Do(cdp.WithExecutor(s.ctx, session)); err != nil {
			return nil, fmt.Errorf("cannot reset screenshot background color: %w", err)
		}
	}

	// Save screenshot capture to file
	// TODO: we should not write to disk here but put it on some queue for async disk writes
	if path != "" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0775); err != nil {
			return nil, fmt.Errorf("cannot create directory for screenshot: %w", err)
		}
		if err := ioutil.WriteFile(path, buf, 0664); err != nil {
			return nil, fmt.Errorf("cannot save screenshot to file: %w", err)
		}
	}
	return &buf, nil
}

func (s *screenshotter) screenshotElement(h *ElementHandle, opts *ElementHandleScreenshotOptions) (*[]byte, error) {
	format := opts.Format
	viewportSize, originalViewportSize, err := s.originalViewportSize(h.frame.page)
	if err != nil {
		return nil, fmt.Errorf("failed to get original viewport size: %w", err)
	}

	err = h.waitAndScrollIntoViewIfNeeded(h.ctx, false, true, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to scroll element into view: %w", err)
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
			return nil, fmt.Errorf("cannot set viewport size: %w", err)
		}
		err = h.waitAndScrollIntoViewIfNeeded(h.ctx, false, true, opts.Timeout)
		if err != nil {
			return nil, fmt.Errorf("cannot scroll element into view: %w", err)
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
	rt := k6common.GetRuntime(s.ctx)
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
			return nil, fmt.Errorf("cannot restore viewport: %w", err)
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
		return nil, fmt.Errorf("failed to get original viewport size: %w", err)
	}

	if opts.FullPage {
		fullPageSize, err := s.fullPageSize(p)
		if err != nil {
			return nil, fmt.Errorf("cannot get full page size: %w", err)
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
				return nil, fmt.Errorf("cannot set viewport size: %w", err)
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
				return nil, fmt.Errorf("cannot trim clip to size: %w", err)
			}
		}

		buf, err := s.screenshot(p.session, documentRect, nil, format, opts.OmitBackground, opts.Quality, opts.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot screenshot: %w", err)
		}
		if overriddenViewportSize != nil {
			if err := s.restoreViewport(p, originalViewportSize); err != nil {
				return nil, fmt.Errorf("cannot restore viewport: %w", err)
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
			return nil, fmt.Errorf("cannot trim clip to size: %w", err)
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
