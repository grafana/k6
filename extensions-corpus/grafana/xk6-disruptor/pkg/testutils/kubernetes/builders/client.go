package builders

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	// ObjectEventAll subscribe to all object events
	ObjectEventAll ObjectEvent = "ALL"
	// ObjectEventAdded subscribe to object creation events
	ObjectEventAdded ObjectEvent = "ADDED"
	// ObjectEventDeleted subscribe to object delete events
	ObjectEventDeleted ObjectEvent = "DELETED"
	// ObjectEventModified subscribe to object update events
	ObjectEventModified ObjectEvent = "MODIFIED"
)

// ObjectEvent defines an event in an object
type ObjectEvent string

// PodObserver is a function that receives notifications of events on an Pod
// and can update it by returning a non-nil value. In addition, the PodObserver
// returns a boolean value indicating if it wants to keep receiving events or not.
//
// Note: PodObserver that subscribe to update events should implement a mechanism for
// avoiding an update loop. They can for instance check the object is in a particular state
// before updating. Additionally, they can unsubscribe from further updates.
type PodObserver func(ObjectEvent, *corev1.Pod) (*corev1.Pod, bool, error)

// ClientBuilder defines a fluent API for configuring a fake client for testing
type ClientBuilder interface {
	// returns an instance of a fake.Clientset.
	Build() (*fake.Clientset, error)
	// WithObjects initializes the client with the given runtime.Objects
	WithObjects(objs ...runtime.Object) ClientBuilder
	// WithNamespace initializes the client with the given namespace
	WithNamespace(namespace string) ClientBuilder
	// WithPods initializes the client with the given Pods
	WithPods(pods ...corev1.Pod) ClientBuilder
	// WithServices initializes the client with the given Services
	WithServices(pods ...corev1.Service) ClientBuilder
	// WithPodObserver adds a PodObserver that receives notifications of specific events
	WithPodObserver(namespace string, event ObjectEvent, observer PodObserver) ClientBuilder
	// WithContext sets a context allows cancelling object observers
	WithContext(ctx context.Context) ClientBuilder
	// WithErrorChannel sets a channel for reporting errors from observers
	WithErrorChannel(chan error) ClientBuilder
}

// clientBuilder builds fake instances of Kubernetes.Interface
type clientBuilder struct {
	ctx       context.Context
	client    *fake.Clientset
	errors    []error
	errCh     chan error
	observers []func()
	ready     sync.WaitGroup
}

// NewClientBuilder returns a ClientBuilder
func NewClientBuilder() ClientBuilder {
	return &clientBuilder{
		ctx:    context.TODO(),
		client: fake.NewSimpleClientset(),
		errCh:  make(chan error),
	}
}

func (b *clientBuilder) WithContext(ctx context.Context) ClientBuilder {
	b.ctx = ctx
	return b
}

func (b *clientBuilder) WithErrorChannel(errCh chan error) ClientBuilder {
	b.errCh = errCh
	return b
}

func (b *clientBuilder) WithObjects(objs ...runtime.Object) ClientBuilder {
	for _, o := range objs {
		err := b.client.Tracker().Add(o)
		if err != nil {
			b.errors = append(b.errors, err)
			break
		}
	}
	return b
}

func (b *clientBuilder) WithPods(pods ...corev1.Pod) ClientBuilder {
	for p := range pods {
		_, err := b.client.CoreV1().
			Pods(pods[p].Namespace).
			Create(
				context.TODO(),
				&pods[p],
				metav1.CreateOptions{},
			)
		if err != nil {
			b.errors = append(b.errors, err)
		}
	}
	return b
}

func (b *clientBuilder) WithNamespace(namespace string) ClientBuilder {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	_, err := b.client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		b.errors = append(b.errors, err)
	}

	return b
}

func (b *clientBuilder) WithServices(services ...corev1.Service) ClientBuilder {
	for s := range services {
		_, err := b.client.CoreV1().
			Services(services[s].
				Namespace).
			Create(
				context.TODO(),
				&services[s],
				metav1.CreateOptions{},
			)
		if err != nil {
			b.errors = append(b.errors, err)
		}
	}
	return b
}

//nolint:gocognit
func (b *clientBuilder) WithPodObserver(namespace string, event ObjectEvent, observer PodObserver) ClientBuilder {
	// goroutine that watches for events that match the observer's subscription
	obsFunc := func() {
		watcher, err := b.client.CoreV1().Pods(namespace).Watch(
			context.TODO(),
			metav1.ListOptions{},
		)
		if err != nil {
			b.errCh <- fmt.Errorf("failed watching pods : %w", err)
			return
		}
		defer watcher.Stop()

		b.ready.Done()
		for {
			select {
			case <-b.ctx.Done():
				return
			case watcherEvent := <-watcher.ResultChan():
				if watcherEvent.Type == watch.Error {
					b.errCh <- fmt.Errorf("error event received watching pods")
					return
				}
				pod, isPod := watcherEvent.Object.(*corev1.Pod)

				// if we receive an unexpected object, ignore
				if !isPod {
					continue
				}
				objectEvent := ObjectEvent(watcherEvent.Type)
				if event != ObjectEventAll && objectEvent != event {
					continue
				}

				// call observer
				updated, cont, err := observer(objectEvent, pod)
				// if observer returns error notify and stop watching
				if err != nil {
					b.errCh <- fmt.Errorf("observer returned error: %w", err)
					return
				}

				// if observer returns a non nul pod, update it
				if updated != nil {
					_, err := b.client.
						CoreV1().
						Pods(namespace).
						Update(context.TODO(), updated, metav1.UpdateOptions{})
					if err != nil {
						b.errCh <- fmt.Errorf("failed updating pod %w", err)
						return
					}
				}

				// observer does not want to continue watching actions, stop watching
				if !cont {
					return
				}
			}
		}
	}

	b.observers = append(b.observers, obsFunc)

	return b
}

func (b *clientBuilder) Build() (*fake.Clientset, error) {
	if len(b.errors) > 0 {
		// TODO: use errors.Join when de code is updated to go >= 1.20
		return nil, fmt.Errorf("errors building client %v", b.errors)
	}

	// start observers as goroutines
	for _, observer := range b.observers {
		b.ready.Add(1)
		go observer()
	}

	// wait until all observers have started
	b.ready.Wait()

	return b.client, nil
}
