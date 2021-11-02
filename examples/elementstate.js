import launcher from 'k6/x/browser';
import { check } from 'k6';

export default function() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  // Inject page content
  page.setContent(`
    <div class="visible">Hello world</div>
    <div style="display:none" class="hidden"></div>
    <div class="editable" editable>Edit me</div>
    <input type="checkbox" enabled class="enabled">
    <input type="checkbox" disabled class="disabled">
    <input type="checkbox" checked class="checked">
    <input type="checkbox" class="unchecked">
  `);

  // Check state
  check(page, {
    'visible': p => p.$('.visible').isVisible(),
    'hidden': p => p.$('.hidden').isHidden(),
    'editable': p => p.$('.editable').isEditable(),
    'enabled': p => p.$('.enabled').isEnabled(),
    'disabled': p => p.$('.disabled').isDisabled(),
    'checked': p => p.$('.checked').isChecked(),
    'unchecked': p => p.$('.unchecked').isChecked() === false,
  });

  page.close();
  browser.close();
}
