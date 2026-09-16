package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Keep browser instances sequential so CPU contention does not distort deadline assertions.
//
//nolint:paralleltest,tparallel
func TestSelectorDeadlineAndHandleLifetime(t *testing.T) {
	t.Parallel()
	for _, api := range []string{"locator", "page", "frame", "element"} {
		t.Run(api, func(t *testing.T) {
			tb := newTestBrowser(t)
			tb.vu.ActivateVU()
			tb.vu.StartIteration(t)
			defer tb.vu.EndIteration(t)
			_, err := tb.vu.RunAsync(t, `
    const page=await browser.newPage();
    try {
     await page.setContent('<button id="ready">ready</button>');
     const api=%q;
     const target=api==='element'?await page.$('body'):api==='frame'?page.mainFrame():page;
     if(api==='locator') await page.locator('#ready').waitFor({timeout:500});
     else {
      const handle=await target.waitForSelector('#ready',{timeout:500});
      if(await handle.textContent()!=='ready')throw Error('returned handle unusable');
      await handle.dispose();
     }
     await page.evaluate(()=>{setTimeout(()=>{const end=Date.now()+1000;while(Date.now()<end){}},100)});
     const start=Date.now();let rejected=false;
     try {
      if(api==='locator')await page.locator('#absent').waitFor({timeout:500});
      else await target.waitForSelector('#absent',{timeout:500});
     }catch{rejected=true}
     if(!rejected)throw Error('missing selector unexpectedly resolved');
     if(Date.now()-start>=900)throw Error('deadline waited for blocked renderer');
    }finally{await page.close()}
   `, api)
			require.NoError(t, err)
		})
	}
}
