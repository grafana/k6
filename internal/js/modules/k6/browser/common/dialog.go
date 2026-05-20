package common

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
)

// Dialog represents a JavaScript dialog (alert, confirm, prompt).
type Dialog struct {
	ctx     context.Context
	session session
	handled bool
}

func newDialog(ctx context.Context, s session) *Dialog {
	return &Dialog{
		ctx:     ctx,
		session: s,
	}
}

// Accept accepts the dialog.
func (d *Dialog) Accept() error {
	if d.handled {
		return nil
	}
	err := cdppage.HandleJavaScriptDialog(true).Do(cdp.WithExecutor(d.ctx, d.session))
	if err == nil {
		d.handled = true
	}
	return err
}

// Dismiss dismisses the dialog.
func (d *Dialog) Dismiss() error {
	if d.handled {
		return nil
	}
	err := cdppage.HandleJavaScriptDialog(false).Do(cdp.WithExecutor(d.ctx, d.session))
	if err == nil {
		d.handled = true
	}
	return err
}
