xk6-browser roadmap
===================

xk6-browser is a [k6](https://k6.io/) extension that will become part of the k6 core once it reaches its stability goals. The project adds browser automation support to k6, expanding the testing capabilities of the k6 ecosystem to include real-world user simulation in addition to API/performance testing. This allows web developers to test their applications fully end-to-end in a way that previously wasn't possible with k6 alone.

We consider browser automation to be an important part of web-application testing, and we have big goals in mind for xk6-browser. In the spirit of transparency, we'd like to share our roadmap for the project with the k6 community. We hope that users can plan ahead, trusting the k6 and Grafana's commitment to its success. With that in mind, we'll detail some of our important status updates, our short, mid and long-term goals. Most of these will be worked on concurrently, and reaching them will be a gradual process. The timeframes are also not set in stone, but rather serve as tentative targets that the team is aiming for.

Status updates
----------------

- **Is this production ready?**<br>
   xk6-browser is ready to be used in production. However, be warned that our API is still undergoing a few changes so expect a few breaking changes and bugs 🐞.

- **Is this extension supported in k6 Cloud?**<br>
    No, not yet. We take the security of our customer data very seriously and currently, we are analyzing the implications of running browser instances in the cloud.

- **It doesn't work with my Chromium/Chrome version, why?**<br>
    CDP evolves and there are differences between different versions of Chromium, sometimes quite subtle. The codebase is continuously tested with the two latest major releases of Google Chrome.

- **Are Firefox or WebKit-based browsers supported?**<br>
    Not yet. There are differences in CDP coverage between Chromium, Firefox, and WebKit-based browsers. xk6-browser is initially only targetting Chromium-based browsers.

- **Are all features of Playwright supported?**<br>
    No. Playwright's API is pretty large and some of the functionality only makes sense if it's implemented using async operations: event listening, request interception, waiting for events, etc. This requires the existence of an event loop per VU in k6, which was only [recently added](https://github.com/grafana/k6/issues/882). Most of the current xk6-browser API is synchronous and thus lacks some of the functionality that requires asynchronicity, but we're gradually migrating existing methods to return a `Promise`, and adding new ones that will follow the same API.

    Expect breaking changes during this transition. We'll point them out in the release notes as well as proposed migration plan.

    Note that `async`/`await` is still under development and is not supported in k6 scripts. If you wish to use this syntax you'll have to transform your script beforehand with an updated Babel version. See the [k6-template-es6 project](https://github.com/grafana/k6-template-es6) and [this comment](https://github.com/grafana/k6/issues/779#issuecomment-964027280) for details.

Short-term goals
----------------

These are goals achievable within 3-6 months, tentatively done by Q2 2023.

- **Make xk6-browser a part of standard distribution of k6.**<br>

  *How will we achieve this?*<br>
  xk6-browser will become an [experimental module](https://k6.io/docs/javascript-api/k6-experimental/) of k6, and thus available to use in load tests that utilize standard k6 release builds.

  *Definition of Done*<br>
  A load test script that imports `k6/x/browser` can be executed using the latest release build of k6. The browser-level APIs provided by xk6-browser are available for use within this script.

- **Public beta in k6 Cloud.**

  *How will we achieve this?*<br>
  Cloud builds will start using k6 releases that include xk6-browser as an experimental module.

  *Definition of Done*<br>
  Load test executed in Cloud can use xk6-browser API by importing `k6/x/browser`.

Mid-term goals
--------------

These are goals achievable within 6-12 months, tentatively done by mid 2023.

- **Transition our API to be async/`Promise` based.**<br>
  Currently (April 2022), most of our API is synchronous. This is due to the historical fact that k6 didn't support async behavior because of a missing per-VU event loop.
[This event loop is now available](https://github.com/grafana/k6/pull/2228).
  Async APIs are important for a browser-testing tool, since most browser behavior and [CDP](https://chromedevtools.github.io/devtools-protocol/) (the protocol we use to communicate with the browser) is event-based. We need to expose an async API to implement this missing functionality and reach feature parity with tools like Playwright or Puppeteer.

  *How will we achieve this?*<br>
  By gradually transitioning the current API to an async implementation, and implementing new features as async.

  *Definition of Done*<br>
  When most of the API can be used asynchronously.


- **Beta availability of the extension in k6 Cloud for all users.**<br>

  *How will we achieve this?*<br>
  The deployment should be optimized and the extension thoroughly tested before making it available to all users. Frontend changes should be done at this point, and usage costs (CPU, RAM, storage) and pricing details should be determined, followed by public announcements of the availability. Features such as screen capture, video recording, downloading, and file uploading should be available.

  *Definition of Done*<br>
  When all users are eligible to upgrade to a plan that includes browser testing.


- **Merge the extension into the main k6 repository.**<br>
  k6 and Grafana consider browser testing to be another strategy under the testing umbrella, along with performance, contract, and other types of testing that modern web applications benefit from. As such, the scope of the k6 tool will expand to include functional/E2E testing using browser automation, with the ultimate goal of merging the xk6-browser extension into the main k6 repository as a core JS module.

  *How will we achieve this?*<br>
  Once the extension and API are relatively stable, and it's well-tested in k6 Cloud, we will create a PR to the main k6 repository with the extension code. Then we will deprecate the standalone xk6-browser repository, and point users to use the main k6 binary instead.

  This will be a minor breaking change, as scripts will have to import `k6/browser` instead of `k6/x/browser`, and CI jobs will need to change, but the functionality should remain the same.

  *Definition of Done*<br>
  Once the extension has been merged into the k6 repository.


- **Increase test code coverage; refactor problematic areas of the codebase; fix "flaky" tests, linter issues, etc.**<br>
  We'll constantly focus on these mid and long-term goals. See issues [#228](https://github.com/grafana/xk6-browser/issues/228), [#241](https://github.com/grafana/xk6-browser/issues/241) and [#58](https://github.com/grafana/xk6-browser/issues/58).

  *How will we achieve this?*<br>
  By gradually adding more unit and functional tests and fixing issues when the opportunity arises. We won't have long stints where we focus exclusively on this goal, but it's something we'll dedicate as much time as possible to.

  *Definition of Done*<br>
  The goal is not to reach 100% code coverage, but, realistically speaking, 80 or 90% should be achievable. Our CI test runs should be stable and flake-free, and we should have no linter issues.


Long-term goals
---------------

These are goals achievable after a year, and don't have a clear date of delivery yet.

- **Add support for Firefox and other WebKit-based browsers.**<br>
  Currently (April 2022), our main focus is supporting Chromium-based browsers. We should expand support to include other browsers as well. The main challenges here will be around CDP and the behavior differences between browsers.

  *How will we achieve this?*<br>
  By testing other browsers and fixing issues as they arise.

  *Definition of Done*<br>
  When the latest Firefox and certain WebKit-based browsers are as well-supported as Chromium is.


- **Feature parity with Playwright and Puppeteer.**<br>
  Currently, our functionality is limited compared to more mature projects like Playwright and Puppeteer. We plan to expand this gradually and reach or exceed the features offered by other browser automation tools.

  *How will we achieve this?*<br>
  By prioritizing new features to add based on API importance and user feedback. After the short-term stability improvements are made, our main focus will be to add more missing features and close the current feature gap.

  *Definition of Done*<br>
  When we implement all of the functionality found in other tools that makes sense for xk6-browser. This is intentionally vague at the moment, and we'll refine it as we make progress.


- **Optimize the k6 Cloud deployment and remove the Beta status.**<br>
  At this point, browser testing should be a well-tested feature in k6 Cloud, and work should be done to optimize the deployment.

  *How will we achieve this?*<br>
  By making changes to optimize test-startup times and reduce operating costs. An example could be to use browser pools that are reused between test runs.

  *Definition of Done*<br>
  Difficult to gauge, but whenever we feel comfortable about removing the Beta label. Both browser testing and support for it in k6 Cloud should be considered feature complete, and regular maintenance and improvements should follow.
