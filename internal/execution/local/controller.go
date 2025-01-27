// Package local implements the execution.Controller interface for local
// (single-machine) k6 execution.
package local

// Controller "controls" local tests. It doesn't actually do anything, it just
// implements the execution.Controller interface with no-op operations. The
// methods don't do anything because local tests have only a single instance.
//
// However, for test suites (https://github.com/grafana/k6/issues/1342) in the
// future, we will probably need to actually implement some of these methods and
// introduce simple synchronization primitives even for a single machine...
type Controller struct{}

// NewController creates a new local execution Controller.
func NewController() *Controller {
	return &Controller{}
}

// GetOrCreateData immediately calls the given callback and returns its results.
func (c *Controller) GetOrCreateData(_ string, callback func() ([]byte, error)) ([]byte, error) {
	return callback()
}

// Subscribe is a no-op, it doesn't actually wait for anything, because there is
// nothing to wait on - we only have one instance in local tests.
//
// TODO: actually use waitgroups, since this may actually matter for test
// suites, even for local test runs. That's because multiple tests might be
// executed even by a single instance, and if we have complicated flows (e.g.
// "test C is executed only after test A and test B finish"), the easiest way
// would be for different tests in the suite to reuse this Controller API *both*
// local and distributed runs.
func (c *Controller) Subscribe(_ string) func() error {
	return func() error {
		return nil
	}
}

// Signal is a no-op, it doesn't actually do anything for local test runs.
//
// TODO: similar to Wait() above, this might actually be required for
// complex/branching test suites, even during local non-distributed execution.
func (c *Controller) Signal(_ string, _ error) error {
	return nil
}
