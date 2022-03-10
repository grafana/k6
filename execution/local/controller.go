package local

// Controller controls local tests.
type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) GetOrCreateData(id string, callback func() ([]byte, error)) ([]byte, error) {
	return callback()
}

func (c *Controller) SignalAndWait(eventId string) error {
	return nil
}
