// Run with:
// k6 run --features async-metric-context examples/browser/async_metric_context.js

import { browser } from 'k6/browser';
import { check, group } from 'k6';
import exec from 'k6/execution';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      iterations: 1,
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    'browser_http_req_duration{group:::navigation,owner:browser-navigation}': ['count > 0'],
    checks: ['rate == 1'],
  },
};

function contextMatches(owner, groupName) {
  return exec.vu.metrics.tags.owner === owner
    && exec.vu.metrics.tags.group === groupName
    && exec.vu.metrics.metadata.trace === owner;
}

export default async function () {
  const page = await browser.newPage();
  exec.vu.metrics.tags.owner = 'root';
  exec.vu.metrics.metadata.trace = 'root';

  let observedRequest = false;
  try {
    await group('navigation', async () => {
      exec.vu.metrics.tags.owner = 'browser-navigation';
      exec.vu.metrics.metadata.trace = 'browser-navigation';

      page.on('request', request => {
        if (!request.url().startsWith('https://quickpizza.grafana.com/')) {
          return;
        }
        observedRequest = true;
        check(null, {
          'request callback kept its registration context': () => (
            contextMatches('browser-navigation', '::navigation')
          ),
        });
      });

      await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'domcontentloaded' });
      await page.title();
      page.url();
    });

    check(null, {
      'navigation emitted a request event': () => observedRequest,
      'the context outside the group was restored': () => contextMatches('root', ''),
    });
  } finally {
    await page.close();
  }
}
