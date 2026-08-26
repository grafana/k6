// Package cluster offers helpers for setting a cluster for e2e testing
package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fetch"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubectl"
	"github.com/grafana/xk6-disruptor/pkg/utils"
)

// PostInstall defines a function that runs after the cluster is created
// It can be used for adding components (e.g. addons)
type PostInstall func(ctx context.Context, cluster E2eCluster) error

// E2eClusterConfig defines the configuration of a e2e test cluster
type E2eClusterConfig struct {
	Name           string
	Images         []string
	IngressAddr    string
	IngressPort    int32
	PostInstall    []PostInstall
	Reuse          bool
	Wait           time.Duration
	AutoCleanup    bool
	Kubeconfig     string
	UseEtcdRAMDisk bool
	EnvOverride    bool
}

// E2eCluster defines the interface for accessing an e2e cluster
type E2eCluster interface {
	// Cleanup deletes the cluster if the auto cleanup option was enabled
	Cleanup() error
	// Delete deletes the cluster regardless of the auto cleanup option setting
	Delete() error
	// Ingress returns the url to the cluster's ingress
	Ingress() string
	// Kubeconfig returns the path to the cluster's kubeconfig file
	Kubeconfig() string
	// Name returns the name of the cluster
	Name() string
	// Load loads the supplied images to the clusters' nodes.
	Load(images ...string) error
}

const contourConfig = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: contour
  namespace: projectcontour
data:
  contour.yaml: |
    disablePermitInsecure: false
    ingress-status-address: local.projectcontour.io

`

const contourBaseURL = "https://raw.githubusercontent.com/projectcontour/contour/main/examples/contour/"

// InstallContourIngress installs a customized contour ingress
func InstallContourIngress(ctx context.Context, cluster E2eCluster) error {
	manifests := []string{
		"00-common.yaml",
		"01-crds.yaml",
		"02-job-certgen.yaml",
		"02-rbac.yaml",
		"02-role-contour.yaml",
		"02-service-contour.yaml",
		"02-service-envoy.yaml",
		"03-contour.yaml",
		"03-envoy.yaml",
	}

	client, err := kubectl.NewFromKubeconfig(ctx, cluster.Kubeconfig())
	if err != nil {
		return err
	}

	// create contour resources
	for _, manifest := range manifests {
		url := contourBaseURL + manifest
		yaml, err2 := fetch.FromURL(url)
		if err2 != nil {
			return err2
		}

		err2 = client.Apply(ctx, string(yaml))
		if err2 != nil {
			return err2
		}
	}

	// apply custom configuration
	err = client.Apply(ctx, string(contourConfig))
	if err != nil {
		return err
	}

	return nil
}

// DefaultE2eClusterConfig builds the default configuration for an e2e test cluster
// TODO: allow override of default port using an environment variable (E2E_INGRESS_PORT)
func DefaultE2eClusterConfig() E2eClusterConfig {
	return E2eClusterConfig{
		Name:        "e2e-test",
		Images:      []string{"ghcr.io/grafana/xk6-disruptor-agent:latest"},
		IngressAddr: "localhost",
		IngressPort: 30080,
		Reuse:       false,
		AutoCleanup: true,
		Wait:        60 * time.Second,
		Kubeconfig:  filepath.Join(os.TempDir(), "e2e-test"),
		PostInstall: []PostInstall{
			InstallContourIngress,
		},
		UseEtcdRAMDisk: true,
		EnvOverride:    true,
	}
}

// E2eClusterOption allows modifying an E2eClusterOption
type E2eClusterOption func(E2eClusterConfig) (E2eClusterConfig, error)

// WithIngressPort sets the ingress port
func WithIngressPort(port int32) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.IngressPort = port
		return c, nil
	}
}

// WithIngressAddress sets the ingress address
func WithIngressAddress(address string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.IngressAddr = address
		return c, nil
	}
}

// WithName sets the cluster name
func WithName(name string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Name = name
		return c, nil
	}
}

// WithKubeconfig sets the path to the kubeconfig file
func WithKubeconfig(kubeconfig string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Kubeconfig = kubeconfig
		return c, nil
	}
}

// WithWait sets the timeout for cluster creation
func WithWait(timeout time.Duration) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Wait = timeout
		return c, nil
	}
}

// WithAutoCleanup specifies if the cluster must automatically deleted when test ends
func WithAutoCleanup(autoCleanup bool) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.AutoCleanup = autoCleanup
		return c, nil
	}
}

// WithReuse specifies if an existing cluster with the same name must be reused (true) or deleted (false)
// WithReuse(true) implies WithAutoCleanup(false)
func WithReuse(reuse bool) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Reuse = reuse
		return c, nil
	}
}

// WithEtcdRAMDisk specifies if the cluster must be configured to use a RAM disk for Etcd
func WithEtcdRAMDisk(ramdisk bool) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.UseEtcdRAMDisk = ramdisk
		return c, nil
	}
}

// WithEnvOverride specifies if the cluster configuration can be overridden by environment variables
func WithEnvOverride(override bool) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.EnvOverride = override
		return c, nil
	}
}

// WithImages adds images to the list of images to be pre-loaded into the cluster
func WithImages(images ...string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Images = append(c.Images, images...)
		return c, nil
	}
}

// e2eCluster maintains the status of a cluster
type e2eCluster struct {
	cluster     *cluster.Cluster
	ingress     string
	name        string
	autoCleanup bool
}

// creates and configures a e2e cluster
func createE2eCluster(e2eConfig E2eClusterConfig) (*e2eCluster, error) {
	// create cluster
	config, err := cluster.NewConfig(
		e2eConfig.Name,
		cluster.Options{
			Images: e2eConfig.Images,
			Wait:   e2eConfig.Wait,
			NodePorts: []cluster.NodePort{
				{
					HostPort: e2eConfig.IngressPort,
					NodePort: 80,
				},
			},
			UseEtcdRAMDisk: e2eConfig.UseEtcdRAMDisk,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster config: %w", err)
	}

	c, err := config.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	ingress := fmt.Sprintf("%s:%d", e2eConfig.IngressAddr, e2eConfig.IngressPort)
	cluster := &e2eCluster{
		cluster:     c,
		ingress:     ingress,
		name:        e2eConfig.Name,
		autoCleanup: e2eConfig.AutoCleanup,
	}

	// TODO: set a deadline for the context passed to post install functions
	for _, postInstall := range e2eConfig.PostInstall {
		err = postInstall(context.TODO(), cluster)
		if err != nil {
			_ = cluster.Cleanup()
			return nil, err
		}
	}

	// FIXME: add some form of check to avoid fixed waits
	time.Sleep(e2eConfig.Wait)

	return cluster, nil
}

// merge options from environment variables
func mergeEnvVariables(config E2eClusterConfig) E2eClusterConfig {
	if !config.EnvOverride {
		return config
	}
	config.AutoCleanup = utils.GetBooleanEnvVar("E2E_AUTOCLEANUP", config.AutoCleanup)
	config.Reuse = utils.GetBooleanEnvVar("E2E_REUSE", config.Reuse)
	config.UseEtcdRAMDisk = utils.GetBooleanEnvVar("E2E_ETCD_RAMDISK", config.UseEtcdRAMDisk)
	config.Name = utils.GetStringEnvVar("E2E_NAME", config.Name)
	config.IngressPort = utils.GetInt32EnvVar("E2E_PORT", config.IngressPort)
	return config
}

// BuildE2eCluster builds a cluster for e2e tests
func BuildE2eCluster(
	e2eConfig E2eClusterConfig,
	ops ...E2eClusterOption,
) (e2ec E2eCluster, err error) {
	// apply option functions
	for _, option := range ops {
		e2eConfig, err = option(e2eConfig)
		if err != nil {
			return nil, err
		}
	}

	e2eConfig = mergeEnvVariables(e2eConfig)

	// check if cluster exists
	c, err := cluster.GetCluster(e2eConfig.Name, e2eConfig.Kubeconfig)
	if err != nil {
		return nil, err
	}

	// if exists
	if c != nil {
		// if Reuse option is specified, return existing cluster
		if e2eConfig.Reuse {
			ingress := fmt.Sprintf("%s:%d", e2eConfig.IngressAddr, e2eConfig.IngressPort)
			return &e2eCluster{
				cluster:     c,
				ingress:     ingress,
				name:        e2eConfig.Name,
				autoCleanup: false, // reuse implies no auto-cleanup
			}, nil
		}

		// otherwise, delete it
		err = c.Delete()
		if err != nil {
			return nil, err
		}
	}

	// we need to create a new cluster
	return createE2eCluster(e2eConfig)
}

// DeleteE2eCluster deletes an existing e2e cluster
func DeleteE2eCluster(name string, quiet bool) error {
	return cluster.DeleteCluster(name, quiet)
}

func (c *e2eCluster) Cleanup() error {
	if !c.autoCleanup {
		return nil
	}

	return c.cluster.Delete()
}

func (c *e2eCluster) Delete() error {
	return c.cluster.Delete()
}

func (c *e2eCluster) Name() string {
	return c.name
}

func (c *e2eCluster) Ingress() string {
	return c.ingress
}

func (c *e2eCluster) Kubeconfig() string {
	return c.cluster.Kubeconfig()
}

func (c *e2eCluster) Load(images ...string) error {
	return c.cluster.Load(images...)
}
