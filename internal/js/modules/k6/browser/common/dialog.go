package common

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
)

type Dialog struct {
	ctx          context.Context
	session      session
	handled      bool
	dialogType   string
	message      string
	defaultValue string
}

func newDialog(ctx context.Context, s session, event *cdppage.EventJavascriptDialogOpening) *Dialog {
	return &Dialog{
		ctx:          ctx,
		session:      s,
		dialogType:   event.Type.String(),
		message:      event.Message,
		defaultValue: event.DefaultPrompt,
	}
}

func (d *Dialog) Type() string         { return d.dialogType }
func (d *Dialog) Message() string      { return d.message }
func (d *Dialog) DefaultValue() string { return d.defaultValue }

func (d *Dialog) Accept(promptText ...string) error {
	if d.handled {
		return nil
	}
	action := cdppage.HandleJavaScriptDialog(true)
	if len(promptText) > 0 {
		action = action.WithPromptText(promptText[0])
	}
	err := action.Do(cdp.WithExecutor(d.ctx, d.session))
	if err == nil {
		d.handled = true
	}
	return err
}

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
