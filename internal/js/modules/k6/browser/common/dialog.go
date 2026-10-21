package common

import (
	"context"
	"errors"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
)

// ErrDialogAlreadyHandled is returned when Accept or Dismiss is called on a
// dialog that has already been claimed, accepted, or dismissed.
var ErrDialogAlreadyHandled = errors.New("dialog has already been accepted or dismissed")

type Dialog struct {
	ctx          context.Context
	session      session
	page         *Page
	mu           sync.Mutex
	claimed      bool
	handled      bool
	dialogType   string
	message      string
	defaultValue string
}

func newDialog(ctx context.Context, s session, p *Page, event *cdppage.EventJavascriptDialogOpening) *Dialog {
	return &Dialog{
		ctx:          ctx,
		session:      s,
		page:         p,
		dialogType:   event.Type.String(),
		message:      event.Message,
		defaultValue: event.DefaultPrompt,
	}
}

func (d *Dialog) Type() string         { return d.dialogType }
func (d *Dialog) Message() string      { return d.message }
func (d *Dialog) DefaultValue() string { return d.defaultValue }
func (d *Dialog) Page() *Page          { return d.page }

// Handled reports whether the dialog has already been accepted or dismissed.
func (d *Dialog) Handled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.handled
}

// Claim marks the dialog as claimed; it reports false if already claimed.
func (d *Dialog) Claim() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.claimed {
		return false
	}
	d.claimed = true
	return true
}

// Claimed reports whether a caller has already claimed the dialog via Claim.
func (d *Dialog) Claimed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.claimed
}

func (d *Dialog) Accept(promptText ...string) error {
	return d.handle(true, promptText)
}

func (d *Dialog) Dismiss() error {
	return d.handle(false, nil)
}

func (d *Dialog) handle(accept bool, promptText []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handled {
		return ErrDialogAlreadyHandled
	}
	action := cdppage.HandleJavaScriptDialog(accept)
	if len(promptText) > 0 {
		action = action.WithPromptText(promptText[0])
	}
	err := action.Do(cdp.WithExecutor(d.ctx, d.session))
	if err == nil {
		d.handled = true
	}
	return err
}
