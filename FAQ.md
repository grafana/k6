## FAQ

- **Is this production ready?**<br>
    No, not yet. We're focused on making the extension stable and reliable, as that's our top priority, before adding more features.

- **Is this extension supported in k6 Cloud?**<br>
    No, not yet. Once the codebase is deemed production ready we'll add support for browser-based testing in k6 Cloud.

- **It doesn't work with my Chromium/Chrome version, why?**<br>
    CDP evolves and there are differences between different versions of Chromium, sometimes quite subtle. The codebase is continuously tested with the two latest major releases of Google Chrome.

- **Are Firefox or WebKit-based browsers supported?**<br>
    Not yet. There are differences in CDP coverage between Chromium, Firefox, and WebKit-based browsers. xk6-browser is initially only targetting Chromium-based browsers.

- **Are all features of Playwright supported?**<br>
    No. Playwright's API is pretty large and some of the functionality only makes sense if it's implemented using async operations: event listening, request interception, waiting for events, etc. This requires the existence of an event loop per VU in k6, which was only [recently added](https://github.com/grafana/k6/issues/882). Most of the current xk6-browser API is synchronous and thus lacks some of the functionality that requires asynchronicity, but we're gradually migrating existing methods to return a `Promise`, and adding new ones that will follow the same API.

    Expect breaking changes during this transition. We'll point them out in the release notes as well as proposed migration plan.

    Note that `async`/`await` is still under development and is not supported in k6 scripts. If you wish to use this syntax you'll have to transform your script beforehand with an updated Babel version. See the [k6-template-es6 project](https://github.com/grafana/k6-template-es6) and [this comment](https://github.com/grafana/k6/issues/779#issuecomment-964027280) for details.