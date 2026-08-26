# Design Doc: Disruptor Target Selectors 

|                       |               | 
|-----------------------|---------------|
|**Author(s)**:         | Pablo Chacin (@pablochacin) |
|**Created**:           | 27 Jul 2022 |
|**Status**:            | Accepted |
|**Last status change**:| 14 Sep 2022 | 
|**Approver(s)**:       | Daniel González Lopes (@dgzlopes) |
|**Related**|  N/A |
|**Replaces**| N/A |
|**Superseded by** | N/A |


## Background
The xk6-disruptor extension's API consists in a series of disruptors that execute actions on a target such as a pod or node that interfere with its normal operation. For example, introduce delays in the network traffic of a Pod.

Each disruptor must be capable of selecting the targets of the disruption. In a shared test environment such a Kubernetes cluster this typically means selecting some components of an application such as the pods in a given namespace whose labels match a deployment selector.

Moreover, disruptors are not limited to Kubernetes. Disruption can be applied to physical nodes or other external resources.

## Problem
How to offer an uniform interface for selecting the targets of a disruptor, so that developers do not need to learn a new way for selecting targets for each disruptor.

## Goals
The selector mechanism should be:
* Flexible. Different use cases may require different selection strategies, such as matching for static attributes, dynamic attributes, conditions, or even excluding according to any of those criteria
* Extensible. Support additional targets and additional selection criteria for existing targets
* Make common cases easy and complex cases possible

## Proposal

Use a map of attributes as a selector for defining the target of a disruption. The attributes will vary depending on the target type. For Kubernetes objects (Pods, Nodes), it will be labels, for physical nodes it can be the hostname or ip address. Each disruptor must implement a mechanism for finding a list of targets that match the selector. For a target to match the selector, it must match all of the attributes.
Some examples of selectors

For Pods: 
```
import { PodDisruptor } from 'xk6-disruptor'
Cost podDisruptor = new PodDisruptor({ 
      selector: { labels: { app: "my-app" } }
  })
```

For K8s Nodes: 
```
import { NodeDisruptor } from 'xk6-disruptor'
Cost NodeDisruptor = new NodeDisruptor(
     { 
      selector: { labels: { "topology.kubernetes.io/zone" : "zoneA"} } 
})
```

Defining NOT-like selection criteria (e.g. pods not terminated, nodes not in the control plane) require an additional "exclude" attribute in the interface of the disruptor with the same structure of the selector, but that will exclude those potential targets that match the criteria. For example, select nodes that do not have the control plane label:

```
import { NodeDisruptor } from 'xk6-disruptor'
const podDisruptor = new NodeDisruptor(
     { 
      exclude: { labels: { "node-role.kubernetes.io/control-plane": ""} }
  })
```

Selector and exclude can be combined:

```
import { PodDisruptor } from 'xk6-disruptor'
Cost podDisruptor = new PodDisruptor({ 
      selector: { labels: { app: "my-app" } },
      exclude: { phase: "Failed" }}
  })
```

Querying kubernetes objects for a given status (e.g. Pod is ready, node is drained) is generally difficult because these conditions are stored in structures that must be navigated and not as fields whose value can be matched. Therefore specialized selector criteria must be used. For example, for checking the pod condition:

```
import { PodDisruptor } from 'xk6-disruptor'
Cost podDisruptor = new PodDisruptor({ 
      selector: { 
           labels: { app: "my-app" } },
           condition: "Ready" 
  })

```

### Advantages:
* Simplicity
* Extensibility: it is possible to add more selection criteria (labels, annotations, status, fields) without breaking interface
* Interoperability: maps can be easily passed from JS to the extension (developed in golang). Maps can be serialized and sent to disruptors running as remote processes

### Disadvantages
* Adding new criteria for selecting targets requires modifications to the disruptor
* It is not simple or natural to define OR-like criteria (e.g. nodes in "zoneA" or "zoneB"). This can be in some cases possible if the value for the selection criteria is allowed to be a list, but then the syntax becomes more complex for the most common case of one value:
```
{ labels: { "topology.kubernetes.io/zone" : [ "zoneA" , "zoneB" ] } } 
```
One alternative would to allow comma-separated values to represent list of values but this obscures the syntax:

```
{ labels: { "topology.kubernetes.io/zone" :  "zoneA,zoneB"  } } 
```

Yet another alternative would be using regular expressions but this has a performance penalty:
```
{ labels: { "topology.kubernetes.io/zone" :  "zone(A|B)"  } } 
```

## Alternatives

### Selection Functions
Pass a JS function as a selector and use it for filtering the potential targets.

In the example below the function selects pods that match a label:

```
import { Selector } from 'xk6-disruptor/selectors'
import { PodDisruptor } from 'xk6-disruptor'
func labelSelector (obj) {
  return obj.labels["app"] == "my-app" 
}
Cost podDisruptor = new PodDisruptor({ 
     selector: labelSelector
})
```

#### Advantages
* Flexibility. Any selection logic can be implemented
* Adding new criteria do not require changing the Disruptor

#### Disadvantages
* Not adequate if the number of potential targets is too large (for example, all pods of a namespace)
* Duplication of logic. Many tests may use the same common selection criteria (e.g pods by labels) that must be re-implemented each time. Users can mitigate this by creating their own libraries but this is still an additional complexity for developers. 
It could also be mitigated by offering these common selectors as built-in functions exported by the extension as in the example below:

```
import { Selectors } from 'xk6-disruptor/selectors'
import { PodDisruptor } from 'xk6-disruptor'
Cost podDisruptor = new PodDisruptor({ 
     selector: Selectors.labelSelector({app: "my-app"} ) 
})
```

* Implementing complex criteria that mixes multiple selectors implies additional code. It still can be implemented by offering builtin selectors (similar to the ones defined in the Kubernetes type package) that combine other selectors

```
port { Selectors } from 'xk6-disruptor/selectors'
import { PodDisruptor } from 'xk6-disruptor'
const podDisruptor = new PodDisruptor({ 
     selector: Selectors.and(
               Selectors.labels({app: "my-app"} ),
               Selectors.exclude(Selectors.phase("Failed"))
})
```

The selector function(JavaScript) must be executed from the disruptor (golang) increasing the complexity in the implementation of the disruptor.

### Pass targets to disruptors (do nothing in the API)
One alternative to adding some mechanism for selecting targets to the disruptors could be passing the selected target(s) to the constructor:

```
import { PodDisruptor } from 'xk6-disruptor'
Import { Kubernetes } from 'xk6-kubernetes'
const k8s = new Kubernetes()

func selectPods() {
     return k8s.pods
        .list("default")
        .filter( pod => pod.labels.app == "my-app")
}
 
Cost podDisruptor = new PodDisruptor({ 
     target: selectPods() 
})
```

#### Advantages
* Simplicity
* Flexibility. Any selection logic can be implemented
* Adding new criteria do not require changing the Disruptor

#### Disadvantages

* Transfers to the test script the complexity of finding potential targets (e.g. list all pods that back a service)
* Duplication of logic. Many tests may use the same common selection criteria (e.g pods by labels) that must be re-implemented each time. Users can mitigate this by creating their own libraries. 

## Consensus
There are many use cases to consider and users of the API may have different programming skills. The objective is therefore finding an API that makes more common cases simple and yet supports more sophisticated ones. In the regard, the consensus is to build the selector API around the attribute matching API.

However, it could also be possible to allow for extensions such as explicit listing the targets and filtering functions.


