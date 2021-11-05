import launcher from "k6/x/browser";
import { check } from 'k6';

export default function () {
        const browser = launcher.launch('chromium', {
                headless: __ENV.XK6_HEADLESS ? true : false,
        });
        const context = browser.newContext();
        const page = context.newPage();

        page.goto('https://googlechromelabs.github.io/dark-mode-toggle/demo/', {
                waitUntil: 'load',
        });
        let el = page.$('#dark-mode-toggle-3')
        check(el, {
                "GetAttribute('mode')": e => e.getAttribute('mode') == 'light',
        });

        page.close();
        browser.close();
}