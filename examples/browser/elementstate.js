import { browser } from 'k6/browser';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = await browser.newContext();
  const page = await context.newPage();

  // Inject page content
  await page.setContent(`
    <div class="visible">Hello world</div>
    <div style="display:none" class="hidden"></div>
    <div class="editable" editable>Edit me</div>
    <input type="checkbox" enabled class="enabled">
    <input type="checkbox" disabled class="disabled">
    <input type="checkbox" checked class="checked">
    <input type="checkbox" class="unchecked">
  `);

  // Check state
  await check(page, {
    'is visible': async p => {
      const e = await p.$('.visible');
      return await e.isVisible();
    },
    'is hidden': async p => {
      const e = await p.$('.hidden');
      return await e.isHidden()
    },
    'is editable': async p => {
      const e = await p.$('.editable');
      return await e.isEditable();
    },
    'is enabled': async p => {
      const e = await p.$('.enabled');
      return await e.isEnabled();
    },
    'is disabled': async p => {
      const e = await p.$('.disabled');
      return await e.isDisabled();
    },
    'is checked': async p => {
      const e = await p.$('.checked');
      return await e.isChecked();
    },
    'is unchecked': async p => {
      const e = await p.$('.unchecked');
      return !await e.isChecked();
    }
  });

  // Change state and check again
  await check(page, {
    'is unchecked checked': async p => {
      const e = await p.$(".unchecked");
      await e.setChecked(true);
      return e.isChecked();
    },
    'is checked unchecked': async p => {
      const e = await p.$(".checked");
      await e.setChecked(false);
      return !await e.isChecked();
    }
  });

  await page.close();
}
