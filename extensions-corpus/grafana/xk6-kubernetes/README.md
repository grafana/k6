[![Go Reference](https://pkg.go.dev/badge/github.com/grafana/xk6-kubernetes.svg)](https://pkg.go.dev/github.com/grafana/xk6-kubernetes)
[![Version Badge](https://img.shields.io/github/v/release/grafana/xk6-kubernetes?style=flat-square)](https://github.com/grafana/xk6-kubernetes/releases)
![Build Status](https://img.shields.io/github/actions/workflow/status/grafana/xk6-kubernetes/ci.yml?style=flat-square)

# xk6-kubernetes
A k6 extension for interacting with Kubernetes clusters while testing.

## Build

To build a custom `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

1. Download [xk6](https://github.com/grafana/xk6):
  
    ```bash
    go install go.k6.io/xk6/cmd/xk6@latest
    ```

2. [Build the k6 binary](https://github.com/grafana/xk6#command-usage):
  
    ```bash
    xk6 build --with github.com/grafana/xk6-kubernetes
    ```

    The `xk6 build` command creates a k6 binary that includes the xk6-kubernetes extension in your local folder. This k6 binary can now run a k6 test using [xk6-kubernetes APIs](#apis).


### Development
To make development a little smoother, use the `Makefile` in the root folder. The default target will format your code, run tests, and create a `k6` binary with your local code rather than from GitHub.

```shell
git clone git@github.com:grafana/xk6-kubernetes.git
cd xk6-kubernetes
make
```

Using the `k6` binary with `xk6-kubernetes`, run the k6 test as usual:

```bash
./k6 run k8s-test-script.js

```
# Usage

By default, the API assumes a `kubeconfig` configuration is available at `$HOME/.kube`.

Alternatively, you can pass in the following options as a javascript Object to the Kubernetes constructor to configure access to the Kubernetes API server:

| Option | Value | Description |
| -- | --| ---- |
| config_path | /path/to/kubeconfig | Kubeconfig file location. You can also set this to __ENV.KUBECONFIG to use the location pointed by the `KUBECONFIG` environment variable |
| server | <SERVER_HOST> | Kubernetes API server URL |
| token | <TOKEN> | Bearer Token for authenticating to the Kubernetes API server |

```javascript

import { Kubernetes } from 'k6/x/kubernetes';

export default function () {
  const k = new Kubernetes({
    config_path: '/path/to/kubeconfig',
  });
}
```

# APIs

## Generic API

This API offers methods for creating, retrieving, listing and deleting resources of any of the supported kinds.

|  Method     | Parameters|   Description |
| ------------ | ---| ------ |
| apply         | manifest string| creates a Kubernetes resource given a YAML manifest or updates it if already exists |
| create         | spec object | creates a Kubernetes resource given its specification |
| delete         | kind  | removes the named resource |
|                | name  |
|                | namespace|
| get         | kind| returns the named resource |
|                | name  |
|                | namespace |
| list         | kind| returns a collection of resources of a given kind
|                | namespace |
| update         | spec object | updates an existing resource

### Examples

#### Creating a pod using a specification 
```javascript
import { Kubernetes } from 'k6/x/kubernetes';

const podSpec = {
    apiVersion: "v1",
    kind:       "Pod",
    metadata: {
        name:      "busybox",
        namespace: "testns"
    },
    spec: {
        containers: [
            {
                name:    "busybox",
                image:   "busybox",
                command: ["sh", "-c", "sleep 30"]
            }
        ]
    }
}

export default function () {
  const kubernetes = new Kubernetes();

  kubernetes.create(pod)

  const pods = kubernetes.list("Pod", "testns");

  console.log(`${pods.length} Pods found:`);
  pods.map(function(pod) {
    console.log(`  ${pod.metadata.name}`)
  });
}
```

#### Creating a job using a YAML manifest
```javascript
import { Kubernetes } from 'k6/x/kubernetes';

const manifest = `
apiVersion: batch/v1
kind: Job
metadata:
  name: busybox
  namespace: testns
spec:
  template:
    spec:
      containers:
      - name: busybox
        image: busybox
        command: ["sleep", "300"]
    restartPolicy: Never
`

export default function () {
  const kubernetes = new Kubernetes();

  kubernetes.apply(manifest)

  const jobs = kubernetes.list("Job", "testns");

  console.log(`${jobs.length} Jobs found:`);
  pods.map(function(job) {
    console.log(`  ${job.metadata.name}`)
  });
}
```

#### Interacting with objects created by CRDs

For objects outside of the core API, use the fully-qualified resource name.

```javascript

import { Kubernetes } from 'k6/x/kubernetes';

const manifest = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: yaml-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /my-service-path
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 80
`

export default function () {
  const kubernetes = new Kubernetes();

  kubernetes.apply(manifest);

  const ingresses = kubernetes.list("Ingress.networking.k8s.io", "default")

  console.log(`${ingresses.length} Ingress found:`);
  ingresses.map(function(ingress) {
    console.log(`  ${ingress.metadata.name}`)
  });
}


```

## Helpers

The `xk6-kubernetes` extension offers helpers to facilitate common tasks when setting up a tests. All helper functions work in a namespace to facilitate the development of tests segregated by namespace. The helpers are accessed using the following method:

|  Method      | Parameters|   Description |
| -------------| ---| ------ |
| helpers      | namespace | returns helpers that operate in the given namespace. If none is specified, "default" is used |

The methods above return an object that implements the following helper functions:

|  Method     | Parameters|   Description |
| ------------ | --------| ------ |
| getExternalIP        | service        | returns the external IP of a service if any is assigned before timeout expires|
|                      | timeout in seconds | |
| waitPodRunning | pod name | waits until the pod is in 'Running' state or the timeout expires. Returns a boolean indicating of the pod was ready or not. Throws an error if the pod is Failed. |
|                | timeout in seconds | |
| waitServiceReady         | service name | waits until the given service has at least one endpoint ready or the timeout expires |
|                | timeout in seconds | |



### Examples

### Creating a pod and wait until it is running

```javascript
import { Kubernetes } from 'k6/x/kubernetes';

let podSpec = {
    apiVersion: "v1",
    kind:       "Pod",
    metadata: {
        name:      "busybox",
        namespace:  "default"
    },
    spec: {
        containers: [
            {
                name:    "busybox",
                image:   "busybox",
                command: ["sh", "-c", "sleep 30"]
            }
        ]
    }
}

export default function () {
  const kubernetes = new Kubernetes();

  // create pod
  kubernetes.create(pod)

  // get helpers for test namespace
  const helpers = kubernetes.helpers()

  // wait for pod to be running
  const timeout = 10
  if (!helpers.waitPodRunning(pod.metadata.name, timeout)) {
      console.log(`"pod ${pod.metadata.name} not ready after ${timeout} seconds`)
  }
}
```
