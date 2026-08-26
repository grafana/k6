# xk6-disruptor roadmap

xk6-disruptor is a young project still in an early development phase. The initial releases have been focused on providing a MVP (Minimal Viable Product) that show-cases its potential for a relevant use case: Fault injection in the most common protocols used in microservice applications (HTTP and gRPC)

However, the vision of the project is more ambitions. We envision to have a tool that makes reliability testing accessible to application developers, testers, QA engineers, Devops, SREs and any one concerned with reliability, by offering a simple way to test applications under multiple disruptive conditions.

Our roadmap includes therefore objectives to extend the xk6-disruptor to new use cases, but also to improve existing functionalities making them easier to use and more robust.

## Short-term

These are goals we expect to achieve in the next 6 months (Q3/2023-Q4/2023).
 
 1. Improve disruptor reliability

     The xk6-disruptor [works by installing an agent in the targets of the fault injection](https://k6.io/docs/javascript-api/xk6-disruptor/explanations/how-xk6-disruptor-works/). In order to ensure the execution of a test does not disrupt the target application beyond the parameters defined by the fault injection and neither does it have side effects, it is critical to ensure the agent can recover from situations such as the cancellation of the test.

     Follow-up issues:
     - [Handle interruption in the xk6-disruptor agent and clean-up before exiting](https://github.com/grafana/xk6-disruptor/issues/115)
     - [xk6-disruptor-agent does not terminate if test is cancelled](https://github.com/grafana/xk6-disruptor/issues/82)

2. Expand the catalog of faults for Pod and Service disruptions

   - Connection dropping

        Applications generally maintain a pool of open connections for accessing other services they depend on (for example a database). Different situations, such as resource exhaustion, can force these connections to be disconnected by the service end. The connection-dropping fault will simulate this situation forcing a fraction of connections served by a Pod to disconnect, allowing developers to test the application's ability of detect and recover from these disconnections.

   - Random pod kill

        Even when Kubernetes takes care of automatically restarting failed Pods, there are situations on which the recovery from a Pod crash may introduce failures in the application. Some examples may be the recovery of a member of a stateful set that requires re-synchronization with other members of the set. The random pod kill will forcefully terminate a fraction of the pods selected by a selector.

3. Expand the catalog of disruptors

    - NodeDisruptor

      A NodeDisruptor targets a set of nodes, selected or excluded by labels, annotations, and state, and supports the injection of node-level faults, such as resource exhaustion and network disruption.

      Follow-up issues:
      - [Implement NodeDisruptor](https://github.com/grafana/xk6-disruptor/issues/156)

## Mid-term

These are goals we expect to achieve in 6-12 months (Q1/2024-Q2/2024).

1. Improve API

    The main tenet of xk6-disruptor is to offer the best developer experience. In this regard, we will continue improving the API, providing more simplicity and convenience (making the general use cases easier) and extensibility (making complex use cases possible).

    - Add more criteria for Pod selection

        Presently the pod selector only supports labels. It would be convenient to also select or exclude Pods based on annotations or their state (for example, exclude pods that are in Terminated state). The [selector API](https://github.com/grafana/xk6-disruptor/blob/main/docs/01-development/design-docs/001-selector-api.md) defines some of these criteria and defines a syntax for incorporating in the definition of a selector.

      Follow-up issues:
      - [https://github.com/grafana/xk6-disruptor/issues/73](https://github.com/grafana/xk6-disruptor/issues/73)

    - Add criteria for selecting which requests to be affected by fault injection

        Presently all requests served by a target will be equally affected by the fault injection. However, in most applications, different logical operations (for example, HTTP endpoints) behave differently, as different databases may back them or have different dependencies to other services. Therefore, in order to reproduce real usage conditions, it is necessary to provide the ability to select which requests will be affected by the fault injection based on elements such as the target URL pattern.

      Follow-up issues:
      - [https://github.com/grafana/xk6-disruptor/issues/73](https://github.com/grafana/xk6-disruptor/issues/73)

2. Add fault injection capabilities for other protocols

    Presently the disruptor only supports fault injection for HTTP and gRPC protocols, the two more common protocols for communication between microservices. However, these microservices rely on infrastructure services such as cache servers, databases, message queues, and event streaming servers to operate. Therefore we will explore the implementation of protocol-level fault injection for the other most common protocols used in modern applications.

    Follow-up issues:
    -   [Add fault injection capabilities for Kafka protocol](https://github.com/grafana/xk6-disruptor/issues/151)
    -   [Add fault injection capabilities for Redis protocol](https://github.com/grafana/xk6-disruptor/issues/152)
    -   [Add fault injection capabilities for MySQL protocol](https://github.com/grafana/xk6-disruptor/issues/153)

3. Expand the catalog of disruptors

   - External dependency disruptor

      It is a common use case to test the effect of known patterns of behavior in external dependencies (services that are not under the control of the organization). Using the xk6-disruptor, this could be accomplished by implementing a Dependency Disruptor, which instead of disrupting a service (or a group of pods), disrupts the requests these pods make to other services.

      Follow-up issues:
     - [Dependency disruptor](https://github.com/grafana/xk6-disruptor/issues/53)


## Non-goals

As with any other project, it is important for xk6-disruptor to have a clear boundary of what problems we want to solve and what problems we consider are outside or vision:

1. Fuzzing

   Testing corrupted or invalid responses is an important part of reliability testing.  However, generating realistic responses that are still invalid may require some application-specific logic that must be implemented somehow in the disruptor agent. We consider the complexity introduced by this requirement to outweigh the benefits.
