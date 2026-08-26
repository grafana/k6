> [!WARNING]
> **⚠️ This repository is archived and no longer maintained by Grafana Labs.**
> 
> Grafana Labs is no longer actively maintaining this project. The repository is now read-only, and no further updates, bug fixes, or feature requests will be addressed.
> 
> **You are welcome to fork this repository** if you would like to continue development or maintain your own version.

# xk6-disruptor

</br>
</br>

<p align="center">
  <img src="assets/logo.png" alt="xk6-disruptor" width="300"></a>
  <br>
  "Like Unit Testing, but for <strong>Reliability</strong>"
  <br>
</p>
<p align="center">  
</div>


xk6-disruptor is an extension adds fault injection capabilities to [Grafana k6](https://github.com/grafana/k6). It implements the ideas of the Chaos Engineering discipline and enables Grafana k6 users to test their system's reliability under turbulent conditions.

<blockquote>
⚠️ <strong>Important</strong>
xk6-disruptor is in the alpha stage, undergoing active development. We do not guarantee API compatibility between releases - your k6 scripts may need to be updated on each release until this extension reaches v1.0 release.
</blockquote>

## Why xk6-disruptor?

xk6-disruptor is purposely designed and built to provide the best experience for developers trying to make their systems more reliable:

- Everything as code.
  - No need to learn a new DSL.
  - Developers can use their usual development IDE
  - Facilitate test reuse and sharing

- Fast to adopt with no day-two surprises.
  - No need to deploy and maintain a fleet of agents or operators.
- Easy to extend and integrate with other [types of tests](https://k6.io/docs/test-types/introduction/).
  - No need to try to glue multiple tools together to get the job done.

Also, this project has been built to be a good citizen in the Grafana k6 ecosystem by:

- Working well with other extensions.
- Working well with k6's core concepts and features.

You can check this out in the following example:

```js
export default function () {
    // Create a new pod disruptor with a selector 
    // that matches pods from the "default" namespace with the label "app=my-app"
    const disruptor = new PodDisruptor({
        namespace: "default",
        select: { labels: { app: "my-app" } },
    });

    // Disrupt the targets by injecting HTTP faults into them for 30 seconds
    const fault = {
        averageDelay: 500,
        errorRate: 0.1,
        errorCode: 500
    }
    disruptor.injectHTTPFaults(fault, "30s")
}
```

## Features

The project, at this time, is intended to test systems running in Kubernetes. Other platforms are not supported at this time.

It offers an [API](https://k6.io/docs/javascript-api/xk6-disruptor/api) for creating disruptors that target one specific type of the component (e.g., Pods) and is capable of injecting different kinds of [faults](https://k6.io/docs/javascript-api/xk6-disruptor/api/faults), such as errors in HTTP requests served by that component. 
Currently, disruptors exist for [Pods](https://k6.io/docs/javascript-api/xk6-disruptor/api/poddisruptor) and [Services](https://k6.io/docs/javascript-api/xk6-disruptor/api/servicedisruptor), but others will be introduced in the future as well as additional types of faults for the existing disruptors.

## Use cases

The main use case for xk6-disruptor is to test the resiliency of an application of diverse types of disruptions by reproducing their effects without reproducing their root causes. For example, inject delays in the HTTP requests an application makes to a service without having to stress or interfere with the infrastructure (network, nodes) on which the service runs or affect other workloads in unexpected ways.

In this way, xk6-disruptor make reliability tests repeatable and predictable while limiting their blast radius. These are essential characteristics to incorporate these tests in the test suits of applications deployed on shared infrastructures such as staging environments.

## Learn more

Check the [get started guide](https://k6.io/docs/javascript-api/xk6-disruptor/get-started) for instructions on how to install and use `xk6-disruptor`.

The [examples](https://k6.io/docs/javascript-api/xk6-disruptor/examples/) section in the documentation presents examples of using xk6-disruptor for injecting faults in different scenarios.

If you encounter any bugs or unexpected behavior, please search the [currently open GitHub issues](https://github.com/grafana/xk6-disruptor/issues) first, and create a new one if it doesn't exist yet.

The [Roadmap](/ROADMAP.md) presents the project's goals for the coming months regarding new functionalities and enhancements.

If you are interested in contributing with the development of this project, check the [contributing guide](/docs/01-development/01-contributing.md)



