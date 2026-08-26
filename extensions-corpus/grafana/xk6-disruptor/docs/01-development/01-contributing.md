# Contributing to xk6-disruptor

This document describes how you can contribute to the xk6-disruptor project.

For proposing significant changes (breaking changes in the API, significant refactoring, implementation of complex features) we suggest creating a [design proposal document](./design-docs/README.md) before starting the implementation, to ensure consensus and avoid reworking on aspects on which there is not agreement.

## Release process

The [ci has actions](/.github/workflows/publish.yml) that automate the release process. This process starts automatically when a new tag is pushed with the semver of the new release:

```bash
git tag <semver>
git push origin <semver>
```

Before creating the release tag, the release notes for the new version must be added to the [releases](/releases/) directory and merged into main by means of a pull requests.

## Build locally

As contributor, you will need to build locally the disruptor extension with your changes.

### Requirements

Before starting to develop you have [Go](https://golang.org/doc/install), [Git](https://git-scm.com/) and [Docker](https://docs.docker.com/get-docker/) installed. In order to execute use the targets available in the [Makefile](#makefile) you will also need the `make` tool installed. 

### Clone repository

If you you have not already done so, clone this repository to your local machine:
```bash
$ git clone https://github.com/grafana/xk6-disruptor.git
$ cd xk6-disruptor
```

## Integration tests

Integration tests are implemented in the same packages than the component they test. Execution is conditioned using the `integration` build tag. 

In order to run integration tests (if any) for a package, use the following command:

```sh
go test -tags integration ./path/to/package/... 
```


### Extension/agent image versions dependencies

The xk6-disruptor extension requires an agent image to inject it in its targets.

It is important that the versions of the extension and the agent image are in sync to guarantee compatibility.

When a new release of the disruptor is created by the CI the extension's binary and the agent image are created with matching versions. For example, extension version `v.0.x.0` will use agent image tagged as `v0.x.0`.

Also, an agent image is generated automatically by the CI on each commit to the `main` branch and it is labeled as `latest`.

If you checkout the extension source code and build it locally, it will by default reference the agent image labeled as `latest`.

In this way, an extension built locally from the `main` branch will match the version of the `latest` agent image.

Notice that if you build the agent image locally it will be by default also labeled as `latest`.

### Building the xk6-disruptor-agent image

If you modify the [xk6-disruptor-agent](./02-architecture.md#xk6-disruptor-agent) you have to build the image and made it available in the test environment.

For building the image use the following command:

```bash
$ make agent-image
```

Once the image is built, how to make it available to the Kubernetes cluster depends on the platform you use for testing.

If you are using a local cluster for your tests such as [Kind](https://kind.sigs.k8s.io/) or [Minikube](https://github.com/kubernetes/minikube) you can make the image available by loading it into the test cluster. 

If using `kind` the following command loads the image into the cluster

```
kind load docker-image ghcr.io/grafana/xk6-disruptor-agent:latest
```

If using `minikube` the following command loads the image into the cluster:

```bash
minikube image load ghcr.io/grafana/xk6-disruptor-agent:latest
```

## Debugging the disruptor agent

The disruptor agent is the responsible for injecting faults in the targets (e.g. pods). The agent is injected into the targets by the xk6-disruptor extension as an ephemeral container with the name `xk6-agent`.

### Running manually

Once the agent is injected in a target (using the [xk6-disruptor API](https://k6.io/docs/javascript-api/xk6-disruptor/api/) in a k6 test), you can debug the agent by running it manually.

First enter the agent's container at the target pod using an interactive console:

```bash
kubectl exec -it <target pod> -c xk6-agent -- sh
```

Once you get the prompt, you can inject faults by running the `xk6-disruptor-agent` command:

```bash
xk6-disruptor-agent [arguments for fault injection]
```

### Running locally

It is possible to run the agent locally in your machine using the following command:

```bash
xk6-disruptor-agent [arguments for fault injection]
```

This is useful for debugging and also to facilitate [CPU and memory profiling](#tracing-and-profiling)

### Running as a proxy

When debugging issues with protocols, you can run the agent in your local machine as a proxy that redirects the traffic to an upstream destination.

For running the protocol fault injection as a (non-transparent) proxy use the `--transparent=false` option:

```bash
xk6-disruptor-agent <protocol> --transparent=false [arguments for fault injection]
```

### Tracing and profiling

In order to facilitate debugging `xk6-disruptor-agent` offers options for generating execution traces:
* `--trace`: generate traces. The `--trace-file` option allows specifying the output file for traces (default `trace.out`)
* `--cpu-profile`: generate CPU profiling information. The `--cpu-profile-file` option allows specifying the output file for profile information (default `cpu.pprof`)
* `--mem-profile`: generate memory profiling information. By default, it sets the [memory profile rate](https://pkg.go.dev/runtime#pkg-variables) to `1`, which will profile every allocation. This rate can be controlled using the `--mem-profile-rate` option. The `--mem-profile-file` option allows specifying the output file for profile information (default `mem.pprof`)
* `--metrics`: generate [go runtime metrics](https://pkg.go.dev/runtime/metrics). The metrics are collected at intervals defined by the `--metrics-rate` argument (default to `1s`). At the end of the agent execution the minimum, maximum and average value for each collected metric is reported to the file specified in `--metrics-file` (default `metrics.out`).

If you run the [disruptor manually](#running-manually) in a pod you have to copy them from the target pod to your local machine. For example, for copying the `trace.out` file:

```bash
kubectl cp <target pod>:trace.out -c xk6-agent trace.out
```

## e2e tests

End to end tests are meant to test the components of the project in a test environment without mocks.

The test environment is created using [kind](https://kind.sigs.k8s.io/). The e2e testutils package automates the process of creating and configuring the test clusters.

However, it is convenient to have `kind` installed in case you need to manually interact with the test clusters.

These tests are slow and resource consuming. To prevent them to be executed as part of the `test` target
it is recommended to make their execution conditioned to the `e2e` build tags by adding the following compiler
directives to each test file:

```go
//go:build e2e
// +build e2e
```

The e2e tests are built and executed using the `e2e` target in the Makefile:
```
$ make e2e
```

In order to facilitate the development of e2e tests, diverse helper functions have been created, organized in the following packages:

* [pkg/testutils/e2e/cluster](pkg/testutils/e2e/cluster) functions for creating test clusters using [kind](https://kind.sigs.k8s.io/)
* [pkg/testutils/e2e/fixtures](pkg/testutils/e2e/fixtures) functions for creating test resources
* [pkg/testutils/e2e/checks](pkg/testutils/e2e/checks) functions for verifying conditions during the test

We strongly encourage to keep adding reusable functions to these helper packages instead of implementing fixtures and validations for each test, unless strictly necessarily.

Also, you should create the resources for each test in a different namespace to isolate parallel tests and facilitate the teardown. 

The following example shows the structure of a e2e test that creates a cluster and then executes tests using this infrastructure:

```go
//go:build e2e
// +build e2e

package package-to-test

import (
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
)

// Test_E2E function creates the resources for the tests and executes the test functions
func Test_E2E(t *testing.T) {

	// create cluster
	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-test"),
	)
        if err != nil {
	        t.Errorf("failed to create cluster: %v", err)
	        return
        }
	// delete cluster when all sub-tests end
	t.Cleanup(func(){
		_ = cluster.Cleanup()
	})

	// get kubernetes client
	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	// Execute test on cluster
	t.Run("Test", func(t *testing.T){
		// create namespace for test. Will be deleted automatically when the test ends
		namespace, err := namespace.CreateNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}

		// create test resources using k8s fixtures

		// execute test
	})
}
```

### Accessing services from an e2e test

By default, the e2e clusters are configured to run an ingress controller to allow the access to services deployed in the cluster from the e2e tests.

To allow accessing the ingress, the e2e cluster requires a port to be exposed to the host where the test is running. By default his port is `30080`.

This may cause interference between tests that make a test fail with the following message `failed to create cluster: host port is not available 30080`.

If this happens deleting any remaining test cluster and retrying the failed test alone will generally solve this issue.

The port to be exposed by the test cluster can be changed using the `WithIngressPort` option:

```go
	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-test"),
		cluster.WithIngressPort(30083),
	)
```

### Debugging e2e tests

In the examples above, once the e2e test is completed, the test cluster is deleted using the `Cleanup` method. This is inconvenient for debugging failed tests.

This behavior is controlled by the `AutoCleanup` option in the `E2eClusterConfig`. By default it is true. This option can be disable using the `WithAutoCleanup(false)` option in the `BuildE2eCluster` function or by setting `E2E_AUTOCLEANUP=0` in your environment. When this option is disable, the `Cleanup` method will leave the cluster intact.

### Reusing e2e test clusters

By default, each e2e test creates a new test cluster to ensure it is properly configured and prevent any left-over from previous runs to affect the test.

However, in some circumstances, creating a new cluster for each test is time-consuming.

It is possible to specify that we want to reuse a test cluster if one exists using the `Reuse` option in the `E2eClusterConfig`. This option can be set with the `WithReuse` configuration option or setting the `E2E_REUSE=1` in your environment. Notice that `WithReuse(true)` forces `WithAutoCleanup(false)` as a reused cluster should be preserved until explicitly deleted.

> Note: if you are testing changes in the agent, be sure you generate the image and [upload it to the cluster](#building-the-xk6-disruptor-agent-image) using kind

When you don't longer needs the cluster, you can delete it using the [e2e-cluster tool](#e2e-cluster-tool) or `kind`:

```sh
kind delete cluster --name=<cluster name>
```

### Keeping Kubernetes resources for debugging

By default, test namespaces created with `CreateTestNamespace` are automatically deleted when a test ends. This is inconvenient for debugging failed tests.

This behavior is controlled by passing the `WithKeepOnFail` option when creating the namespace or by setting `E2E_KEEPONFAIL=1` in the environment when running an e2e test.

### e2e-cluster tool

The `e2e-cluster` tool allows the setup and cleanup of e2e clusters. It is convenient for creating a cluster that will be reused by multiple tests, and clean it up when it is no longer used. 

If you plan to create more than one test cluster, you should ensure each has an unique name and also, each one uses a different port for the ingress controller service.

> In order to each test to use the cluster you must set `E2E_REUSE=1` in the environment variables.

```sh
# create cluster with default options
e2e-cluster setup
cluster 'e2e-test' created

# execute tests reusing the cluster
E2E_REUSE=1 go test ...

## cleanup
cluster cleanup
```

It is also possible to create multiple test clusters and use them for different tests. In this case, it is important to remember setting the environment variables to override default options in the tests:

```sh
# create cluster with non-default options
e2e-cluster setup --name other-e2e-cluster --port 30090
cluster 'other-e2e-cluster' created

# execute tests reusing the cluster
# override the cluster name to ensure the test reuses the cluster created above
# override the ingress port to use the one configured in the cluster
E2E_REUSE=1 E2E_NAME=other-e2e-cluster E2E_PORT=30090 go test ...

## cleanup
cluster cleanup --name other-e2e-cluster
```

This tool is create with the command `go install ./cmd/e2e-cluster` that installs the binary `e2e-cluster`

For more details, install the tool and execute `e2e-cluster --help`


