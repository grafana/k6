// practically none of this work on windows
//go:build !windows

package tests

import (
	"encoding/json"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageListSelectors(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	// For each entry, resolve both the raw selector and the locator code, and
	// record the data-check marker of the element they find.
	got := tb.vu.RunPromise(t, `
		const page = await browser.newPage();
		await page.goto('%s');

		const resolve = async (loc) => {
			const count = await loc.count();
			return { count, check: count ? await loc.first().getAttribute('data-check') : '' };
		};
		const result = {};
		for (const kind of ['interactive', 'content', 'all']) {
			result[kind] = [];
			for (const e of await page.listSelectors({ kind })) {
				const viaSelector = await resolve(page.locator(e.selector));
				const viaLocator = await resolve(new Function('page', 'return ' + e.locator)(page));
				result[kind].push({ ...e, viaSelector, viaLocator });
			}
		}
		result.defaultKind = (await page.listSelectors()).map((e) => e.selector);
		try {
			await page.listSelectors({ kind: 'nope' });
		} catch (e) {
			result.badKindError = String(e);
		}
		return JSON.stringify(result);
	`, tb.staticURL("list_selectors.html"))
	require.Equal(t, sobek.PromiseStateFulfilled, got.State(), "%v", got.Result())

	type resolved struct {
		Count int    `json:"count"`
		Check string `json:"check"`
	}
	type entry struct {
		Kind        string   `json:"kind"`
		Role        string   `json:"role"`
		Name        string   `json:"name"`
		Checked     any      `json:"checked"`
		Locator     string   `json:"locator"`
		Selector    string   `json:"selector"`
		ViaSelector resolved `json:"viaSelector"`
		ViaLocator  resolved `json:"viaLocator"`
	}
	var result struct {
		Interactive  []entry  `json:"interactive"`
		Content      []entry  `json:"content"`
		All          []entry  `json:"all"`
		DefaultKind  []string `json:"defaultKind"`
		BadKindError string   `json:"badKindError"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.Result().String()), &result))

	marks := func(entries []entry) []string {
		var marks []string
		for _, e := range entries {
			assert.Equal(t, 1, e.ViaSelector.Count, "selector %s", e.Selector)
			assert.Equal(t, e.ViaSelector, e.ViaLocator, "locator %s", e.Locator)
			marks = append(marks, e.ViaSelector.Check)
		}
		return marks
	}
	interactive := []string{
		"email", "search", "remember", "subscribe", "select-all", "size", "notes",
		"sign-in", "help", "add-margherita", "add-pepperoni", "promo", "editor",
	}
	content := []string{"heading", "paragraph", "logo", "cart-total", "status", "margherita", "pepperoni", "cell"}
	assert.Equal(t, interactive, marks(result.Interactive))
	assert.Equal(t, content, marks(result.Content))
	assert.Equal(t, []string{
		"heading", "paragraph", "logo", "email", "search", "remember", "subscribe", "select-all",
		"size", "notes", "sign-in", "help", "cart-total", "status", "margherita", "add-margherita",
		"pepperoni", "add-pepperoni", "promo", "editor", "cell",
	}, marks(result.All))

	byCheck := make(map[string]entry)
	for _, e := range result.All {
		byCheck[e.ViaSelector.Check] = e
	}
	for _, check := range content {
		assert.Equal(t, "content", byCheck[check].Kind, check)
	}
	for _, check := range interactive {
		assert.Equal(t, "interactive", byCheck[check].Kind, check)
	}

	signIn := byCheck["sign-in"]
	assert.Equal(t, "button", signIn.Role)
	assert.Equal(t, "Sign in", signIn.Name)
	assert.Equal(t, "page.getByRole('button', { name: 'Sign in' })", signIn.Locator)
	assert.Equal(t, `internal:role=button[name="Sign in"i]`, signIn.Selector)
	assert.Nil(t, signIn.Checked)
	assert.Equal(t, "page.getByRole('link', { name: 'Help' })", byCheck["help"].Locator)
	assert.Equal(t, "page.getByRole('textbox', { name: 'Email' })", byCheck["email"].Locator)
	assert.Equal(t, "page.getByRole('heading', { name: 'Pizza shop' })", byCheck["heading"].Locator)
	assert.Equal(t, "page.getByTestId('cart-total')", byCheck["cart-total"].Locator)

	assert.Equal(t, false, byCheck["remember"].Checked)
	assert.Equal(t, true, byCheck["subscribe"].Checked)
	assert.Equal(t, "mixed", byCheck["select-all"].Checked)
	assert.Equal(t, "Remember me", byCheck["remember"].Name)

	assert.Empty(t, byCheck["promo"].Role)
	assert.Empty(t, byCheck["promo"].Name)

	var interactiveSelectors []string
	for _, e := range result.Interactive {
		interactiveSelectors = append(interactiveSelectors, e.Selector)
	}
	assert.Equal(t, interactiveSelectors, result.DefaultKind, "the default kind is interactive")
	assert.Contains(t, result.BadKindError, "kind")
}

func TestListSelectorsFrames(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	got := tb.vu.RunPromise(t, `
		const page = await browser.newPage();
		await page.goto('%s');
		await page.frameLocator('iframe[title="Payment"]').frameLocator('iframe[title="Nested"]').getByRole('button').waitFor();

		const resolve = async (loc) => {
			const count = await loc.count();
			return { count, check: count ? await loc.first().getAttribute('data-check') : '' };
		};
		const withResolved = async (entries) => {
			const out = [];
			for (const e of entries) {
				const viaSelector = await resolve(page.locator(e.selector));
				const viaLocator = await resolve(new Function('page', 'return ' + e.locator)(page));
				out.push({ ...e, viaSelector, viaLocator });
			}
			return out;
		};

		const result = {};
		result.page = await withResolved(await page.listSelectors());
		const payment = page.frames().find((f) => f.name() === '__privateFrame5');
		result.payment = await withResolved(await payment.listSelectors());
		result.paymentURL = payment.url();
		const hidden = page.frames().find((f) => f.name() === 'hidden');
		result.hidden = await hidden.listSelectors();
		await page.evaluate(() => document.querySelector('iframe[name="hidden"]').remove());
		try {
			await hidden.listSelectors();
		} catch (e) {
			result.detachedError = String(e);
		}
		return JSON.stringify(result);
	`, tb.staticURL("list_selectors_frames.html"))
	require.Equal(t, sobek.PromiseStateFulfilled, got.State(), "%v", got.Result())

	type resolved struct {
		Count int    `json:"count"`
		Check string `json:"check"`
	}
	type entry struct {
		Locator     string   `json:"locator"`
		Selector    string   `json:"selector"`
		ViaSelector resolved `json:"viaSelector"`
		ViaLocator  resolved `json:"viaLocator"`
	}
	var result struct {
		Page          []entry `json:"page"`
		Payment       []entry `json:"payment"`
		PaymentURL    string  `json:"paymentURL"`
		Hidden        []any   `json:"hidden"`
		DetachedError string  `json:"detachedError"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.Result().String()), &result))

	marks := func(entries []entry) []string {
		var marks []string
		for _, e := range entries {
			assert.Equal(t, 1, e.ViaSelector.Count, "selector %s", e.Selector)
			assert.Equal(t, e.ViaSelector, e.ViaLocator, "locator %s", e.Locator)
			marks = append(marks, e.ViaSelector.Check)
		}
		return marks
	}
	// A frame's entries go where its iframe is, and the hidden iframe is skipped.
	require.Equal(t, []string{"before", "card", "pay", "after"}, marks(result.Page))

	// The iframe is found by its title, not by its generated-looking name.
	card, pay := result.Page[1], result.Page[2]
	assert.Equal(t, `page.frameLocator('iframe[title="Payment"]').getByRole('textbox', { name: 'Card number' })`, card.Locator)
	assert.Equal(t, `iframe[title="Payment"] >> internal:control=enter-frame >> internal:role=textbox[name="Card number"i]`, card.Selector)
	assert.Equal(t,
		`page.frameLocator('iframe[title="Payment"]').frameLocator('iframe[title="Nested"]').getByRole('button', { name: 'Pay' })`,
		pay.Locator)
	assert.Equal(t,
		`iframe[title="Payment"] >> internal:control=enter-frame >> iframe[title="Nested"] >> internal:control=enter-frame >> internal:role=button[name="Pay"i]`,
		pay.Selector)

	assert.Contains(t, result.PaymentURL, "localhost", "the payment frame should come from another site")

	// frame.listSelectors() lists that frame, with locators starting from the page.
	assert.Equal(t, []string{"card", "pay"}, marks(result.Payment))
	assert.Equal(t, []entry{card, pay}, result.Payment)

	assert.Empty(t, result.Hidden, "elements in a hidden iframe aren't visible")
	assert.NotEmpty(t, result.DetachedError, "listing a detached frame should fail")
}

func TestLocatorListSelectors(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)

	got := tb.vu.RunPromise(t, `
		const page = await browser.newPage();
		await page.goto('%s');
		await page.frameLocator('iframe[title="Promo"]').getByRole('link').waitFor();

		const resolve = async (loc) => {
			const count = await loc.count();
			return { count, check: count ? await loc.first().getAttribute('data-check') : '' };
		};
		const withResolved = async (entries) => {
			const out = [];
			for (const e of entries) {
				const viaSelector = await resolve(page.locator(e.selector));
				const viaLocator = await resolve(new Function('page', 'return ' + e.locator)(page));
				out.push({ ...e, viaSelector, viaLocator });
			}
			return out;
		};

		const result = {};
		result.dialog = await withResolved(await page.getByRole('dialog').listSelectors());
		result.iframe = await withResolved(await page.locator('iframe[title="Promo"]').listSelectors());
		result.inFrameLocator = await withResolved(await page.frameLocator('iframe[title="Promo"]').locator('body').listSelectors());
		const promo = page.frames().find((f) => f.url() === 'about:srcdoc');
		result.inFrame = await withResolved(await promo.locator('body').listSelectors());
		page.setDefaultTimeout(500);
		for (const [key, selector] of [['strictError', 'section'], ['missingError', '#missing']]) {
			try {
				await page.locator(selector).listSelectors();
			} catch (e) {
				result[key] = String(e);
			}
		}
		return JSON.stringify(result);
	`, tb.staticURL("list_selectors_scoped.html"))
	require.Equal(t, sobek.PromiseStateFulfilled, got.State(), "%v", got.Result())

	type resolved struct {
		Count int    `json:"count"`
		Check string `json:"check"`
	}
	type entry struct {
		Locator     string   `json:"locator"`
		Selector    string   `json:"selector"`
		ViaSelector resolved `json:"viaSelector"`
		ViaLocator  resolved `json:"viaLocator"`
	}
	var result struct {
		Dialog         []entry `json:"dialog"`
		Iframe         []entry `json:"iframe"`
		InFrameLocator []entry `json:"inFrameLocator"`
		InFrame        []entry `json:"inFrame"`
		StrictError    string  `json:"strictError"`
		MissingError   string  `json:"missingError"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.Result().String()), &result))

	marks := func(entries []entry) []string {
		var marks []string
		for _, e := range entries {
			assert.Equal(t, 1, e.ViaSelector.Count, "selector %s", e.Selector)
			assert.Equal(t, e.ViaSelector, e.ViaLocator, "locator %s", e.Locator)
			marks = append(marks, e.ViaSelector.Check)
		}
		return marks
	}

	// Only what's inside the dialog, with the dialog's own locator in front.
	require.Equal(t, []string{"close", "checkout", "promo"}, marks(result.Dialog))
	assert.Equal(t, "page.getByRole('dialog').getByRole('button', { name: 'Close' })", result.Dialog[0].Locator)
	assert.Equal(t, `internal:role=dialog >> internal:role=button[name="Close"i]`, result.Dialog[0].Selector)
	assert.Equal(t,
		`page.getByRole('dialog').frameLocator('iframe[title="Promo"]').getByRole('link', { name: 'Promo' })`,
		result.Dialog[2].Locator)

	// A locator that points at an iframe lists the iframe's content.
	require.Equal(t, []string{"promo"}, marks(result.Iframe))
	assert.Equal(t, `page.frameLocator('iframe[title="Promo"]').getByRole('link', { name: 'Promo' })`, result.Iframe[0].Locator)

	// Inside a frame locator, and from a child frame, entries start from the page.
	require.Equal(t, []string{"promo"}, marks(result.InFrameLocator))
	assert.Equal(t,
		`page.frameLocator('iframe[title="Promo"]').locator('body').getByRole('link', { name: 'Promo' })`,
		result.InFrameLocator[0].Locator)
	assert.Equal(t,
		`iframe[title="Promo"] >> internal:control=enter-frame >> body >> internal:role=link[name="Promo"i]`,
		result.InFrameLocator[0].Selector)
	assert.Equal(t, result.InFrameLocator, result.InFrame)

	assert.Contains(t, result.StrictError, "strict")
	assert.NotEmpty(t, result.MissingError, "a locator that matches nothing should fail")
}
