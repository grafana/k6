import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', {
        headless: false,
		slowMo: '500ms' // slow down by 500ms
    });
    const context = browser.newContext();
    const page = context.newPage();

    // Goto front page, find login link and click it
    page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
    const elem = page.$('a[href="/my_messages.php"]');
    elem.click();

    // Enter login credentials and login
    page.$('input[name="login"]').type('admin');
    page.$('input[name="password"]').type('123');
    page.$('input[type="submit"]').click();

    // Wait for next page to load
    page.waitForLoadState('networkdidle');

	page.close();
    browser.close();
}
