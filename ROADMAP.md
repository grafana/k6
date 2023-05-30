# k6 open-source project's roadmap

Our team is dedicated to continuously improving k6 and providing the best user experience possible. We're excited to share our roadmap with the k6 community, outlining the upcoming features and improvements we have planned.

This roadmap covers user-oriented features, UX improvements, JavaScript support, and k6 internals that our team will focus on. Remember that timeframes and priorities may shift, but we believe it's important to share our vision and allow users to plan accordingly.

We hope this updated roadmap provides a clear overview of our plans for k6's future development. As always, we welcome feedback, corrections, and suggestions to make this roadmap more comprehensive, accessible, and valuable for the k6 community.

## Short-term goals

These are goals that are tentatively achievable by **Q2 2023** (April-June).

### gRPC streaming support

Although k6 already supports gRPC, we've been actively working towards adding streaming capabilities. The implementation is mostly complete, and we expect to release it as an experimental feature soon.

*see*: [#2020](https://github.com/grafana/k6/issues/2020)

### WebCrypto API

The current k6 crypto module has limitations and doesn't support advanced use cases involving private-key-infrastructure. Our goal is to provide support for encryption/decryption operations using symmetric and asymmetric keys. A partial implementation of the WebCrypto API is available as the `k6/experimental/webcrypto` module, and we're working on adding the remaining features.

*see*: the [xk6-webcrypto](https://github.com/grafana/xk6-webcrypto) repository which holds the implementation of the `k6/experimental/webcrypto` module for more details.

## Mid-term goals

These are goals tentatively achievable by **Q1 2024**.

### ECMAScript modules support

Although k6 supports ECMAScript modules, we currently rely on Babel for code transpilation, which has performance and design drawbacks. We're working on implementing native ES module support in Goja, our JavaScript runtime. The project is ongoing, with no estimated delivery date.

*see*: [#2258](https://github.com/grafana/k6/issues/2258)

### Large files handling

We're working to improve k6's handling of large files, ensuring efficient and reliable test execution for users working with big data sets or complex test scenarios. Our current file-handling approach can lead to significant memory consumption in some cases. We're addressing this issue, and specific improvements will be incrementally released in upcoming versions.

*see*: [#2974](https://github.com/grafana/k6/issues/2974)

### Improve `async/await` syntax support

We aim to enhance the support of async functionality within k6, making it easier for users to write and execute asynchronous code. We're improving compatibility with our existing APIs and expect users to see improvements in upcoming versions.

*see*: [#3014](https://github.com/grafana/k6/issues/3014), [#2967](https://github.com/grafana/k6/issues/2967), [#2869](https://github.com/grafana/k6/issues/2869)

### Command-line output improvements

We've received consistent feedback about potential improvements to the k6 terminal summary displayed at the end of a test run. We aim to enhance the CLI output to provide better readability and more insightful information, helping users understand their test results and performance more efficiently.

### HDR & sparse histograms

We're exploring ways to integrate the use of HDR and sparse histograms in k6 to improve our metrics' accuracy and overall performance of k6. HDR histograms provide a fixed memory footprint and constant performance, regardless of the number of recorded values. Sparse histograms are a variation of HDR histograms that can be used to reduce the memory footprint of a histogram at the cost of some accuracy. We're still in the early stages of development, with no timeline set timeline for completion.

*see*: [#763](https://github.com/grafana/k6/issues/763)

## Long-term goals

These are goals we anticipate will be delivered more **than a year from now**. We have yet to set a specific timeline for these goals.

### HTTP API redesign

The k6 HTTP module, while widely used, has limitations, design flaws, and inconsistencies. We've started a project to create a new HTTP module to improve usability and make it more intuitive and user-friendly. The project is currently in the research and analysis phase, with no timeline for completion.

*see*: [#2461](https://github.com/grafana/k6/issues/2461)

### Native distributed execution

We aim to enhance k6 to natively support distributed load testing across multiple machines, making it easier for users to scale their tests. Although some research and initial design work have been done, we're still in the early stages of development and still need an estimated delivery date.

*see*: [#140](https://github.com/grafana/k6/issues/140)

### Test suites

We aim to implement a test suite feature to enable better organization and management of test cases, making it more convenient for users to maintain and execute multiple tests. The end goal from a user perspective would be to be able to run multiple scripts, one after the other, in a user-defined sequence to form what we refer to as a "test suite". This would allow, for instance, to have scenarios wait for other scenarios to complete before they start their execution.

*see*: [#1342](https://github.com/grafana/k6/issues/1342)

