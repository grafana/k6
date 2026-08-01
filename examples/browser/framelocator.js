import { browser } from 'k6/browser';
import { check } from 'k6';
import exec from 'k6/execution';

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

export default async function () {
  const page = await browser.newPage();

  try {
    // create a page with an iframe
    await page.setContent(`
      <html>
        <body>
          <h1>Main Page</h1>
          <iframe id="my-iframe" srcdoc="
            <html>
              <body>
                <h2>Inside iframe</h2>
                <button id='submit-btn'>Submit</button>
              </body>
            </html>
          "></iframe>
        </body>
      </html>
    `);

    /*
    The frameLocator() method is a shorthand for locator(selector).contentFrame().
    Both approaches are equivalent:

      // Using frameLocator():
      const button = page.frameLocator('#my-iframe').locator('#submit-btn');

      // Using locator().contentFrame():
      const frame = page.locator('#my-iframe').contentFrame();
      const button = frame.locator('#submit-btn');
    */

    const button = page.frameLocator('#my-iframe').locator('#submit-btn');
    const buttonText = await button.textContent();
    console.log(`Button text: ${buttonText}`);

    check(buttonText, {
      'found button inside iframe': (text) => text === 'Submit',
    });

    const heading = await page.frameLocator('#my-iframe').getByRole('heading').textContent();
    console.log(`Heading inside iframe: ${heading}`);

    check(heading, {
      'found heading inside iframe': (text) => text === 'Inside iframe',
    });

  } catch (error) {
    // This script is run as a smoke test in the build workflow to verify the
    // browser Docker image. exec.test.fail() is used here (instead of fail())
    // to produce a non-zero exit code on exception, failing that CI check.
    // error.message is empty for rejected promises, so fall back to error.toString()
    const message = error.message || error.toString();
    exec.test.fail(`Browser iteration failed: ${message}`);
  } finally {
    await page.close();
  }
}
