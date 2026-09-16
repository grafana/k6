package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func TestWaitForSelectorDuringDocumentRootReplacement(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                string
		removeRoot, restoreRoot, overridden bool
	}{
		{name: "normal root"},
		{name: "restored root", removeRoot: true, restoreRoot: true},
		{name: "restored root with overridden globals", removeRoot: true, restoreRoot: true, overridden: true},
		{name: "absent root", removeRoot: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t)
			page := tb.NewPage(nil)
			require.NoError(t, page.SetContent(`<button id="ready">Ready</button>`, nil))
			_, err := page.Evaluate(`(removeRoot, restoreRoot, overridden) => {
    if (overridden) {
     window.Map = () => { throw new Error('page Map used'); };
     window.Set = () => { throw new Error('page Set used'); };
    }
    window.originalRoot = document.documentElement;
    if (removeRoot) document.documentElement.remove();
    if (restoreRoot) setTimeout(() => document.appendChild(window.originalRoot), 600);
   }`, tc.removeRoot, tc.restoreRoot, tc.overridden)
			require.NoError(t, err)
			_, err = page.WaitForSelector("#ready", common.NewFrameWaitForSelectorOptions(2*time.Second))
			if tc.removeRoot && !tc.restoreRoot {
				require.ErrorContains(t, err, "timed out")
			} else {
				require.NoError(t, err)
			}
			unchanged, err := page.Evaluate(`(absent) =>
    document.documentElement === (absent ? null : window.originalRoot) &&
    document.querySelectorAll('iframe').length === 0`, tc.removeRoot && !tc.restoreRoot)
			require.NoError(t, err)
			require.Equal(t, true, unchanged)
		})
	}
}
