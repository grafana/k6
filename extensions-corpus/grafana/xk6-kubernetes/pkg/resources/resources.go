// Package resources implement the interface for accessing kubernetes resources
package resources

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/xk6-kubernetes/pkg/utils"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// UnstructuredOperations defines generic functions that operate on any kind of Kubernetes object
type UnstructuredOperations interface {
	Apply(manifest string) error
	Create(obj map[string]interface{}) (map[string]interface{}, error)
	Delete(kind string, name string, namespace string) error
	Get(kind string, name string, namespace string) (map[string]interface{}, error)
	List(kind string, namespace string) ([]map[string]interface{}, error)
	Update(obj map[string]interface{}) (map[string]interface{}, error)
}

// StructuredOperations defines generic operations that handles runtime objects such as corev1.Pod.
// It facilitates handling objects in the situations where their type is known as opposed to the
// UnstructuredOperations
type StructuredOperations interface {
	// Create creates a resource described in the runtime object given as input and returns the resource created.
	// The resource must be passed by value (e.g corev1.Pod) and a value (not a reference) will be returned
	Create(obj interface{}) (interface{}, error)
	// Delete deletes a resource given its kind, name and namespace
	Delete(kind string, name string, namespace string) error
	// Get retrieves a resource into the given placeholder given its kind, name and namespace
	Get(kind string, name string, namespace string, obj interface{}) error
	// List retrieves a list of resources in the given slice given their kind and namespace
	List(kind string, namespace string, list interface{}) error
	// Update updates an existing resource and returns the updated version
	// The resource must be passed by value (e.g corev1.Pod) and a value (not a reference) will be returned
	Update(obj interface{}) (interface{}, error)
}

// structured holds the
type structured struct {
	client *Client
}

// Client holds the state to access kubernetes
type Client struct {
	ctx        context.Context
	dynamic    dynamic.Interface
	mapper     meta.RESTMapper
	serializer runtime.Serializer
}

// NewFromConfig creates a new Client using the provided kubernetes client configuration
func NewFromConfig(ctx context.Context, config *rest.Config) (*Client, error) {
	dynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return NewFromClient(ctx, dynamic), nil
}

// NewFromClient creates a new client from a dynamic Kubernetes client
func NewFromClient(ctx context.Context, dynamic dynamic.Interface) *Client {
	return &Client{
		ctx:        ctx,
		dynamic:    dynamic,
		serializer: yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme),
	}
}

// WithMapper specifies the RESTMapper for the client to utilize
func (c *Client) WithMapper(mapper meta.RESTMapper) *Client {
	c.mapper = mapper
	return c
}

// getResource maps kinds to api resources
func (c *Client) getResource(kind string, namespace string, versions ...string) (dynamic.ResourceInterface, error) {
	gk := schema.ParseGroupKind(kind)
	if c.mapper == nil {
		return nil, fmt.Errorf("RESTMapper not initialized")
	}

	mapping, err := c.mapper.RESTMapping(gk, versions...)
	if err != nil {
		return nil, err
	}

	var resource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = c.dynamic.Resource(mapping.Resource).Namespace(namespace)
	} else {
		resource = c.dynamic.Resource(mapping.Resource)
	}

	return resource, nil
}

// Apply creates a resource in a kubernetes cluster from a YAML manifest
func (c *Client) Apply(manifest string) error {
	uObj := &unstructured.Unstructured{}
	_, gvk, err := c.serializer.Decode([]byte(manifest), nil, uObj)
	if err != nil {
		return fmt.Errorf("failed to decode manifest: %w", err)
	}

	name := uObj.GetName()
	namespace := uObj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	resource, err := c.getResource(gvk.GroupKind().String(), namespace, gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get resource: %w", err)
	}

	_, err = resource.Apply(
		c.ctx,
		name,
		uObj,
		metav1.ApplyOptions{
			FieldManager: "xk6-kubernetes",
		},
	)
	return err
}

// Create creates a resource in a kubernetes cluster from an object with its specification
func (c *Client) Create(obj map[string]interface{}) (map[string]interface{}, error) {
	uObj := &unstructured.Unstructured{
		Object: obj,
	}

	gvk := uObj.GroupVersionKind()
	namespace := uObj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	resource, err := c.getResource(gvk.GroupKind().String(), namespace)
	if err != nil {
		return nil, err
	}

	resp, err := resource.Create(
		c.ctx,
		uObj,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}
	return resp.UnstructuredContent(), nil
}

// Get returns an object given its kind, name and namespace
func (c *Client) Get(kind string, name string, namespace string) (map[string]interface{}, error) {
	resource, err := c.getResource(kind, namespace)
	if err != nil {
		return nil, err
	}

	resp, err := resource.Get(
		c.ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}
	return resp.UnstructuredContent(), nil
}

// List returns a list of objects given its kind and namespace
func (c *Client) List(kind string, namespace string) ([]map[string]interface{}, error) {
	resource, err := c.getResource(kind, namespace)
	if err != nil {
		return nil, err
	}

	resp, err := resource.List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	list := []map[string]interface{}{}
	for _, uObj := range resp.Items {
		list = append(list, uObj.UnstructuredContent())
	}
	return list, nil
}

// Delete deletes an object given its kind, name and namespace
func (c *Client) Delete(kind string, name string, namespace string) error {
	resource, err := c.getResource(kind, namespace)
	if err != nil {
		return err
	}
	err = resource.Delete(c.ctx, name, metav1.DeleteOptions{})

	return err
}

// Update updates a resource in a kubernetes cluster from an object with its specification
func (c *Client) Update(obj map[string]interface{}) (map[string]interface{}, error) {
	uObj := &unstructured.Unstructured{
		Object: obj,
	}

	gvk := uObj.GroupVersionKind()
	namespace := uObj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}
	resource, err := c.getResource(gvk.GroupKind().String(), namespace)
	if err != nil {
		return nil, err
	}

	resp, err := resource.Update(
		c.ctx,
		uObj,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return nil, err
	}
	return resp.UnstructuredContent(), nil
}

// Structured returns a reference to a StructuredOperations interface
func (c *Client) Structured() StructuredOperations {
	return &structured{
		client: c,
	}
}

// Creates a resources defined in the runtime object provided as input
func (s *structured) Create(obj interface{}) (interface{}, error) {
	uObj, err := utils.RuntimeToGeneric(&obj)
	if err != nil {
		return nil, err
	}

	created, err := s.client.Create(uObj)
	if err != nil {
		return nil, err
	}

	// create a new object of the same time than one provided as input
	result := reflect.New(reflect.TypeOf(obj))
	err = utils.GenericToRuntime(created, result.Interface())
	if err != nil {
		return nil, err
	}

	return result.Elem().Interface(), nil
}

func (s *structured) Get(kind string, name string, namespace string, obj interface{}) error {
	gObj, err := s.client.Get(kind, name, namespace)
	if err != nil {
		return err
	}

	return utils.GenericToRuntime(gObj, obj)
}

func (s *structured) Delete(kind string, name string, namespace string) error {
	return s.client.Delete(kind, name, namespace)
}

func (s *structured) List(kind string, namespace string, objList interface{}) error {
	objListType := reflect.ValueOf(objList).Elem().Kind().String()
	if objListType != reflect.Slice.String() {
		return fmt.Errorf("must provide an slice to return results but %s received", objListType)
	}

	list, err := s.client.List(kind, namespace)
	if err != nil {
		return err
	}

	// get the type of the elements of the input slice for creating new instanced
	// used to convert from the generic structure to the corresponding runtime object
	rtList := reflect.ValueOf(objList).Elem()
	rtType := reflect.TypeOf(objList).Elem().Elem()
	for _, gObj := range list {
		rtObj := reflect.New(rtType)
		err = utils.GenericToRuntime(gObj, rtObj.Interface())
		if err != nil {
			return err
		}

		rtList.Set(reflect.Append(rtList, rtObj.Elem()))
	}
	return nil
}

func (s *structured) Update(obj interface{}) (interface{}, error) {
	uObj, err := utils.RuntimeToGeneric(&obj)
	if err != nil {
		return nil, err
	}

	updated, err := s.client.Update(uObj)
	if err != nil {
		return nil, err
	}

	// create a new object of the same time than one provided as input
	result := reflect.New(reflect.TypeOf(obj))
	err = utils.GenericToRuntime(updated, result.Interface())
	if err != nil {
		return nil, err
	}

	return result.Elem().Interface(), nil
}
