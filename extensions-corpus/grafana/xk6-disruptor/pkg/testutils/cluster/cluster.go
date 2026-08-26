// Package cluster implements helpers for creating test clusters using
// [kind] as a library.
// The helpers facilitate some common customizations of clusters, such as
// creating multiple worker nodes or exposing a node port to facilitate the
// access to services deployed in the cluster using NodePort services.
// Other customizations can be achieved by providing a valid [kind configuration].
//
// [kind]: https://github.com/kubernetes-sigs/kind
// [kind configuration]: https://kind.sigs.k8s.io/docs/user/configuration
package cluster

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

const baseKindConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane`

const etcdPatch = `
kubeadmConfigPatches:
- |
  kind: ClusterConfiguration
  etcd:
    local:
      dataDir: /tmp/etcd`

const kindPortMapping = `
  - containerPort: %d
    hostPort: %d
    listenAddress: "0.0.0.0"
    protocol: tcp`

// NodePort defines the mapping of a node port to host port
type NodePort struct {
	// NodePort to access in the cluster
	NodePort int32
	// Port used in the host to access the node port
	HostPort int32
}

// Options defines options for customizing the cluster
type Options struct {
	// cluster configuration to use. Overrides other options (NodePorts, Workers)
	Config string
	// List of images to pre-load on each node.
	// The images must be available locally (e.g. with docker pull <image>)
	Images []string
	// maximum time to wait for cluster creation.
	Wait time.Duration
	// node ports to expose
	NodePorts []NodePort
	// number of worker nodes
	Workers int
	// Kubernetes version
	Version string
	// Path to Kubeconfig
	Kubeconfig string
	// UseEtcdRAMDisk
	UseEtcdRAMDisk bool
}

// Config contains the configuration for creating a cluster
type Config struct {
	// name of the cluster
	name string
	// options for creating cluster
	options Options
}

// DefaultConfig creates a ClusterConfig with default options and
// default name "test-cluster"
func DefaultConfig() (*Config, error) {
	return NewConfig("test-cluster", Options{})
}

// NewConfig creates a ClusterConfig with the ClusterOptions
func NewConfig(name string, options Options) (*Config, error) {
	if name == "" {
		return nil, fmt.Errorf("cluster name is mandatory")
	}

	for _, np := range options.NodePorts {
		if np.HostPort == 0 || np.NodePort == 0 {
			return nil, fmt.Errorf("node port and host port are required in a NodePort")
		}
	}

	return &Config{
		name:    name,
		options: options,
	}, nil
}

// Render returns the Kind configuration for creating a cluster
// with this ClusterConfig
func (c *Config) Render() (string, error) {
	if c.options.Config != "" {
		return c.options.Config, nil
	}

	var config strings.Builder
	config.WriteString(baseKindConfig)
	if len(c.options.NodePorts) > 0 {
		// create section for port mappings in master node and add each mapped port
		fmt.Fprint(&config, "\n  extraPortMappings:")
		for _, np := range c.options.NodePorts {
			fmt.Fprintf(&config, kindPortMapping, np.NodePort, np.HostPort)
		}
	}

	for range c.options.Workers {
		fmt.Fprintf(&config, "\n- role: worker")
	}

	if c.options.UseEtcdRAMDisk {
		fmt.Fprintf(&config, "\n%s", etcdPatch)
	}

	return config.String(), nil
}

// Cluster an active test cluster
type Cluster struct {
	//  path to the Kubeconfig
	kubeconfig string
	// kind cluster provider
	provider kind.Provider
	// name of the cluster
	name string
}

// try to bind to host port to check availability
func checkHostPort(port int32) error {
	l, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return fmt.Errorf("host port is not available %d", port)
	}
	// ignore error
	_ = l.Close()

	return nil
}

// loadImages loads the images in the list to all cluster nodes' local repositories
func loadImages(images []string, nodes []nodes.Node) error {
	imagesTar, err := os.CreateTemp(os.TempDir(), "image*.tar")
	if err != nil {
		return err
	}

	defer func() {
		// ignore error. Nothing to do if cannot remove image
		_ = os.Remove(imagesTar.Name())
	}()

	// pull if not present
	for _, image := range images {
		imageCmd := []string{"image", "ls", "-q", image}
		var output []byte

		output, err = exec.Command("docker", imageCmd...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("could not list image: %q: %w", image, err)
		}

		// image is not present
		if len(output) == 0 {
			pullCmd := []string{"pull", image}
			output, err = exec.Command("docker", pullCmd...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("could not pull image: %q: %s", image, string(output))
			}
		}
	}

	// save the images to a tar
	saveCmd := append([]string{"save", "-o", imagesTar.Name()}, images...)
	output, err := exec.Command("docker", saveCmd...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error saving images: %s", string(output))
	}

	// load the image on each node of the cluster
	for _, n := range nodes {
		image, err := os.Open(imagesTar.Name())
		if err != nil {
			return err
		}
		err = nodeutils.LoadImageArchive(n, image)
		// ignore error. Nothing to do if cannot close file
		_ = image.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// pullImages pulls images to local node
func pullImages(images []string) error {
	for _, image := range images {
		output, err := exec.Command("docker", "pull", image).CombinedOutput()
		if err != nil {
			return fmt.Errorf("error pulling image %s: %s", image, string(output))
		}
	}

	return nil
}

// Create creates a test cluster with the given name
func (c *Config) Create() (*Cluster, error) {
	// before creating cluster check host ports are available
	// to avoid weird kind error creating cluster
	for _, np := range c.options.NodePorts {
		err := checkHostPort(np.HostPort)
		if err != nil {
			return nil, err
		}
	}

	provider := kind.NewProvider()

	config, err := c.Render()
	if err != nil {
		return nil, err
	}

	kindOptions := []kind.CreateOption{
		kind.CreateWithRawConfig([]byte(config)),
	}

	// if Kubernetes version is specified, try to pull image to check it is supported
	if c.options.Version != "" {
		nodeImage := fmt.Sprintf("kindest/node:%s", c.options.Version)
		err = pullImages([]string{nodeImage})
		if err != nil {
			return nil, fmt.Errorf("could not pull kind node image for version %s: %w", c.options.Version, err)
		}
		kindOptions = append(kindOptions, kind.CreateWithNodeImage(nodeImage))
	}

	if c.options.Wait > 0 {
		kindOptions = append(kindOptions, kind.CreateWithWaitForReady(c.options.Wait))
	}

	err = provider.Create(
		c.name,
		kindOptions...,
	)
	if err != nil {
		return nil, err
	}

	kubeconfig := c.options.Kubeconfig
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.TempDir(), c.name)
	}
	err = provider.ExportKubeConfig(c.name, kubeconfig, false)
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{
		name:       c.name,
		kubeconfig: kubeconfig,
		provider:   *provider,
	}

	// pre-load images
	if len(c.options.Images) > 0 {
		err = cluster.Load(c.options.Images...)
		if err != nil {
			return nil, fmt.Errorf("preloading images: %w", err)
		}
	}

	return cluster, nil
}

// Delete deletes a test cluster
func (c *Cluster) Delete() error {
	return c.provider.Delete(
		c.name,
		c.kubeconfig,
	)
}

// Kubeconfig returns the path to the kubeconfig for the test cluster
func (c *Cluster) Kubeconfig() string {
	return c.kubeconfig
}

// Name returns the name of the cluster
func (c *Cluster) Name() string {
	return c.name
}

// Load loads the supplied images into the cluster.
func (c *Cluster) Load(images ...string) error {
	nodes, err := c.provider.ListInternalNodes(c.name)
	if err != nil {
		return err
	}
	err = loadImages(images, nodes)
	if err != nil {
		return err
	}

	return nil
}

// GetCluster returns an existing cluster if exists, nil otherwise
func GetCluster(name string, kubeconfig string) (*Cluster, error) {
	if name == "" || kubeconfig == "" {
		return nil, fmt.Errorf("cluster name and kubeconfig path are required")
	}

	provider := kind.NewProvider()

	clusters, err := provider.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving list of clusters %w", err)
	}

	//nolint:nilnil
	if !strings.Contains(strings.Join(clusters, ","), name) {
		return nil, nil
	}

	err = provider.ExportKubeConfig(name, kubeconfig, false)
	if err != nil {
		return nil, err
	}

	return &Cluster{
		name:       name,
		kubeconfig: kubeconfig,
		provider:   *provider,
	}, nil
}

// DeleteCluster deletes an existing cluster
func DeleteCluster(name string, silent bool) error {
	if name == "" {
		return fmt.Errorf("cluster name is required")
	}

	// create kubeconfig required by GetCluster. It is not used here
	// but the provider requires ir for deleing the cluster (!?)
	kubeconfig, err := os.CreateTemp(os.TempDir(), "kubeconfig")
	if err != nil {
		return fmt.Errorf("could not create kubeconfig file for cluster %w", err)
	}
	defer func() {
		_ = os.Remove(kubeconfig.Name())
	}()

	cluster, err := GetCluster(name, kubeconfig.Name())
	if err != nil {
		return fmt.Errorf("could not retrieve cluster %q: %w", name, err)
	}

	if cluster != nil {
		return cluster.Delete()
	}

	if !silent {
		return fmt.Errorf("cluster %q does not exists", name)
	}

	return nil
}
