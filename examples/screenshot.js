import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', { headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');
    page.screenshot({ path: `example-chromium.png` });
	page.close();
    browser.close();
}
