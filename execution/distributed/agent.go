package distributed

import (
	context "context"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/execution"
)

// AgentController implements the execution.Controller interface for distributed
// tests. Every `k6 agent` in the test can use it to synchronize itself with the
// other instances. Itq sends requests to the coordinator, listens to
// responses and controls the local test on the agent instance.
type AgentController struct {
	instanceID uint32
	cnc        DistributedTest_CommandAndControlClient
	logger     logrus.FieldLogger

	// TODO: something much more robust and nicer to use...
	doneWaitQueuesLock    sync.Mutex
	doneWaitQueues        map[string]chan *ControllerMessage_DoneWaitWithID
	dataReceiveQueuesLock sync.Mutex
	dataReceiveQueues     map[string]chan *ControllerMessage_DataWithID
	createDataQueuesLock  sync.Mutex
	createDataQueues      map[string]chan *ControllerMessage_CreateDataWithID
}

var _ execution.Controller = &AgentController{}

// NewAgentController creates a new AgentController for the given instance. It
// uses the supplied k6 coordinator server to synchronize the current instance
// with other instances in the same test run.
func NewAgentController(
	ctx context.Context, instanceID uint32, client DistributedTestClient, parentLogger logrus.FieldLogger,
) (*AgentController, error) {
	cnc, err := client.CommandAndControl(ctx)
	if err != nil {
		return nil, err
	}

	logger := parentLogger.WithField("component", "agent-controller")
	logger.Debugf("Sending instance ID %d to coordinator", instanceID)
	err = cnc.Send(&AgentMessage{Message: &AgentMessage_InitInstanceID{instanceID}})
	if err != nil {
		return nil, err
	}

	ac := &AgentController{
		instanceID:        instanceID,
		cnc:               cnc,
		logger:            logger,
		doneWaitQueues:    make(map[string]chan *ControllerMessage_DoneWaitWithID),
		dataReceiveQueues: make(map[string]chan *ControllerMessage_DataWithID),
		createDataQueues:  make(map[string]chan *ControllerMessage_CreateDataWithID),
	}

	go func() {
		for {
			msgContainer, err := cnc.Recv()
			if err != nil {
				logger.WithError(err).Debug("received an unexpected error from recv stream")
				return
			}

			switch msg := msgContainer.Message.(type) {
			case *ControllerMessage_DoneWaitWithID:
				ac.doneWaitQueuesLock.Lock()
				ac.doneWaitQueues[msg.DoneWaitWithID] <- msg
				ac.doneWaitQueuesLock.Unlock()
			case *ControllerMessage_DataWithID:
				ac.dataReceiveQueuesLock.Lock()
				ac.dataReceiveQueues[msg.DataWithID.Id] <- msg
				ac.dataReceiveQueuesLock.Unlock()
			case *ControllerMessage_CreateDataWithID:
				ac.createDataQueuesLock.Lock()
				ac.createDataQueues[msg.CreateDataWithID] <- msg
				ac.createDataQueuesLock.Unlock()
			default:
				logger.Errorf("Unknown controller message type '%#v'", msg)
			}
		}
	}()

	return ac, nil
}

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// GetOrCreateData requests the data chunk with the given id, if it already
// exists. If it doesn't (i.e. if this is the first time this function is called
// with that id), the given callback is called and its result and error are
// saved for the id.
//
// This is an atomic function, so any calls to it while the callback is being
// executed the the same ID will wait for it to finish.
func (c *AgentController) GetOrCreateData(dataID string, callback func() ([]byte, error)) ([]byte, error) {
	c.logger.Debugf("GetOrCreateData(%s)", dataID)

	msg := &AgentMessage{Message: &AgentMessage_GetOrCreateDataWithID{dataID}}
	c.dataReceiveQueuesLock.Lock()
	chGetData := make(chan *ControllerMessage_DataWithID)
	c.dataReceiveQueues[dataID] = chGetData
	c.dataReceiveQueuesLock.Unlock()

	c.createDataQueuesLock.Lock()
	chCreateData := make(chan *ControllerMessage_CreateDataWithID)
	c.createDataQueues[dataID] = chCreateData
	c.createDataQueuesLock.Unlock()

	if err := c.cnc.Send(msg); err != nil {
		return nil, err
	}

	var result []byte
	var err error
	select {
	case <-chCreateData:
		c.logger.Debugf("We get to create the data for %s", dataID)
		result, err = callback()
		msgBack := &AgentMessage{
			Message: &AgentMessage_CreatedData{CreatedData: &DataPacket{
				Id:    dataID,
				Data:  result,
				Error: errStr(err),
			}},
		}
		if serr := c.cnc.Send(msgBack); err != nil {
			c.logger.Errorf("Could not send back data message: %s", serr)
		}
	case data := <-chGetData:
		c.logger.Debugf("Received data for %s", dataID)
		result = data.DataWithID.Data
		if data.DataWithID.Error != "" {
			err = errors.New(data.DataWithID.Error)
		}
	}

	c.dataReceiveQueuesLock.Lock()
	delete(c.dataReceiveQueues, dataID)
	c.dataReceiveQueuesLock.Unlock()

	c.createDataQueuesLock.Lock()
	delete(c.createDataQueues, dataID)
	c.createDataQueuesLock.Unlock()

	return result, err
}

// Subscribe creates a listener for the specified event ID and returns a
// callback that can wait until all other instances have reache it.
func (c *AgentController) Subscribe(eventID string) (wait func() error) {
	c.logger.Debugf("Subscribe(%s)", eventID)

	c.doneWaitQueuesLock.Lock()
	ch := make(chan *ControllerMessage_DoneWaitWithID)
	c.doneWaitQueues[eventID] = ch
	c.doneWaitQueuesLock.Unlock()

	// TODO: implement proper error handling, network outage handling, etc.
	return func() error {
		<-ch
		c.doneWaitQueuesLock.Lock()
		delete(c.doneWaitQueues, eventID)
		c.doneWaitQueuesLock.Unlock()
		return nil
	}
}

// Signal sends a signal to the coordinator that the current instance has
// reached the given event ID, or that it has had an error.
func (c *AgentController) Signal(eventID string, sigErr error) error {
	c.logger.Debugf("Signal(%s, %q)", eventID, sigErr)

	msg := &AgentMessage{Message: &AgentMessage_SignalAndWaitOnID{eventID}}
	if sigErr != nil {
		// TODO: something a bit more robust and information-packed, also
		// including the event ID in the error
		msg.Message = &AgentMessage_Error{sigErr.Error()}
	}

	if err := c.cnc.Send(msg); err != nil {
		c.logger.Errorf("Signal(%s) got an unexpected error: %s", eventID, err)
		return err
	}
	return nil
}
