package distributed

import (
	context "context"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
)

// AgentController listens sends requests to the coordinator, listens to
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

func NewAgentController(
	ctx context.Context, instanceID uint32, client DistributedTestClient, logger logrus.FieldLogger,
) (*AgentController, error) {
	cnc, err := client.CommandAndControl(ctx)
	if err != nil {
		return nil, err
	}

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

func (c *AgentController) GetOrCreateData(dataId string, callback func() ([]byte, error)) ([]byte, error) {
	c.logger.Debugf("GetOrCreateData(%s)", dataId)

	msg := &AgentMessage{Message: &AgentMessage_GetOrCreateDataWithID{dataId}}
	c.dataReceiveQueuesLock.Lock()
	chGetData := make(chan *ControllerMessage_DataWithID)
	c.dataReceiveQueues[dataId] = chGetData
	c.dataReceiveQueuesLock.Unlock()

	c.createDataQueuesLock.Lock()
	chCreateData := make(chan *ControllerMessage_CreateDataWithID)
	c.createDataQueues[dataId] = chCreateData
	c.createDataQueuesLock.Unlock()

	if err := c.cnc.Send(msg); err != nil {
		return nil, err
	}

	var result []byte
	var err error
	select {
	case <-chCreateData:
		c.logger.Debugf("We get to create the data for %s", dataId)
		result, err = callback()
		msgBack := &AgentMessage{
			Message: &AgentMessage_CreatedData{CreatedData: &DataPacket{
				Id:    dataId,
				Data:  result,
				Error: errStr(err),
			}},
		}
		if err := c.cnc.Send(msgBack); err != nil {
			c.logger.Errorf("Could not send back data message: %s", err)
		}
	case data := <-chGetData:
		c.logger.Debugf("Received data for %s", dataId)
		result = data.DataWithID.Data
		if data.DataWithID.Error != "" {
			err = errors.New(data.DataWithID.Error)
		}
	}

	c.dataReceiveQueuesLock.Lock()
	delete(c.dataReceiveQueues, dataId)
	c.dataReceiveQueuesLock.Unlock()

	c.createDataQueuesLock.Lock()
	delete(c.createDataQueues, dataId)
	c.createDataQueuesLock.Unlock()

	return result, err
}

func (c *AgentController) SignalAndWait(eventId string) error {
	c.logger.Debugf("SignalAndWait(%s)", eventId)

	c.doneWaitQueuesLock.Lock()
	ch := make(chan *ControllerMessage_DoneWaitWithID)
	c.doneWaitQueues[eventId] = ch
	c.doneWaitQueuesLock.Unlock()

	msg := &AgentMessage{Message: &AgentMessage_SignalAndWaitOnID{eventId}}
	if err := c.cnc.Send(msg); err != nil {
		c.logger.Errorf("SignalAndWait(%s) got an unexpected error: %s", eventId, err)
		return err
	}

	<-ch
	c.logger.Debugf("SignalAndWait(%s) done!", eventId)

	c.doneWaitQueuesLock.Lock()
	delete(c.doneWaitQueues, eventId)
	c.doneWaitQueuesLock.Unlock()
	return nil
}
