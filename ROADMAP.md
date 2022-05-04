xk6-browser roadmap
===================

xk6-browser is a [k6](https://k6.io/) extension that will become part of the k6 core once it reaches its stability goals. The project adds browser automation support to k6, expanding the testing capabilities of the k6 ecosystem to include real-world user simulation in addition to API/performance testing. This allows web developers to test their applications fully end-to-end in a way that previously wasn't possible with k6 alone.

We consider browser automation to be an important part of web-application testing, and we have big goals in mind for xk6-browser. In the spirit of transparency, we'd like to share our roadmap for the project with the k6 community.
We hope that users can plan ahead, trusting the k6 and Grafana's commitment to its success.

With that in mind, we'll detail some of our short, mid and long-term goals. Most of these will be worked on concurrently, and reaching them will be a gradual process. The timeframes are also not set in stone, but rather serve as tentative targets that the team is aiming for.


Short-term goals
----------------

These are goals achievable within 3-6 months, tentatively done by Q3 2022.

- **Stabilize the existing functionality and fix any major, high-impact, and blocking bugs.**<br>
  Tool stability is something we take very seriously and users shouldn't run into any major showstopping issues.

  *How will we achieve this?*<br>
  By manually testing the application in different scenarios, adding more automated tests, and addressing reports from internal and external users.
  Any issues found that have a considerable user impact will be given highest priority and fixed ASAP.

  *Definition of Done*<br>
  This is difficult to gauge, but at some point we should stop running into any major issues on a frequent basis and should consider this goal as reached.


- **Limited Alpha deployment and testing of the extension in k6 Cloud.**<br>

  *How will we achieve this?*<br>
  Soon we will create a small-scale deployment for internal use and specific customers, meant to test the basic integration of the extension in k6 Cloud.

  *Definition of Done*<br>
  When the extension is usable in k6 Cloud; i.e. scripts can run and test results are shown. Frontend changes are not required at this stage.


- **Update [API documentation](https://k6.io/docs/javascript-api/xk6-browser/).**<br>
  The current documentation is not up-to-date, which makes using xk6-browser outside of the examples quite difficult.

  *How will we achieve this?*<br>
  We are prioritizing documentation work in all upcoming development cycles.

  *Definition of Done*<br>
  When the documentation reflects the state of the current API.


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
