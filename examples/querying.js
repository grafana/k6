import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', { headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    // Find element using CSS selector
    let ip = page.$('.ip-address p').textContent();
    console.log("CSS selector: ", ip);

    // Find element using XPath expression
    ip = page.$("//div[@class='ip-address']/p").textContent();
    console.log("Xpath expression: ", ip);

    // Find element using Text search
    //ip = page.$("My IP Address").textContent();
    //console.log("Text search: ", ip);

	page.close();
    browser.close();
}
