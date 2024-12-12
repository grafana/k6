import { browser } from 'k6/x/browser';

export const options = {
  scenarios: {
    browser: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  },
}

export default async function() {
  const context = browser.newContext()

  // We want to wait for two page creations before carrying on.
  var counter = 0
  const promise = context.waitForEvent("page", { predicate: page => {
    if (++counter == 2) {
      return true
    }
    return false
  } })
  
  // Now we create two pages.
  const page = context.newPage()
  const page2 = context.newPage()

  // We await for the page creation events to be processed and the predicate
  // to pass.
  await promise
  console.log('predicate passed')

  page.close()
  page2.close()
};
