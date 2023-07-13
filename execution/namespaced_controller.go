package execution

// GetNamespacedController controller that wraps another controller, with
// methods that prefix all eventIDs with the given namespace before calling it.
func GetNamespacedController(namespace string, controller Controller) Controller {
	return &namespacedController{namespace: namespace, c: controller}
}

type namespacedController struct {
	namespace string
	c         Controller
}

func (nc namespacedController) GetOrCreateData(id string, callback func() ([]byte, error)) ([]byte, error) {
	return nc.c.GetOrCreateData(nc.namespace+"/"+id, callback)
}

func (nc namespacedController) Subscribe(eventID string) (wait func() error) {
	return nc.c.Subscribe(nc.namespace + "/" + eventID)
}

func (nc namespacedController) Signal(eventID string, err error) error {
	return nc.c.Signal(nc.namespace+"/"+eventID, err)
}
