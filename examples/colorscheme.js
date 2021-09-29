import launcher from "k6/x/browser";
import { sleep } from "k6";

export default function() {
    const browser = launcher.launch('chromium', {
        colorScheme: 'dark', // Valid values are "light", "dark" or "no-preference"
        headless: false
    });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    sleep(5);

	page.close();
    browser.close();
}