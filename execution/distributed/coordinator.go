package distributed

import (
	"bytes"
	context "context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
)

// TODO: something more polished...
type CoordinatorServer struct {
	UnimplementedDistributedTestServer
	instanceCount int
	test          *lib.Archive
	logger        logrus.FieldLogger

	cc              *coordinatorController
	currentInstance int32 // TODO: something a bit better, support full execution plans from JSON?
	ess             lib.ExecutionSegmentSequence
	archive         []byte
}

func NewCoordinatorServer(
	instanceCount int, test *lib.Archive, logger logrus.FieldLogger,
) (*CoordinatorServer, error) {
	segments, err := test.Options.ExecutionSegment.Split(int64(instanceCount))
	if err != nil {
		return nil, err
	}
	ess, err := lib.NewExecutionSegmentSequence(segments...)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err = test.Write(buf); err != nil {
		return nil, err
	}

	cs := &CoordinatorServer{
		instanceCount: instanceCount,
		test:          test,
		logger:        logger,
		ess:           ess,
		cc:            newCoordinatorController(instanceCount, logger),
		archive:       buf.Bytes(),
	}
	return cs, nil
}

func (cs *CoordinatorServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	instanceID := atomic.AddInt32(&cs.currentInstance, 1)
	if instanceID > int32(cs.instanceCount) {
		return nil, fmt.Errorf("we don't need any more instances")
	}
	cs.logger.Infof("Instance %d of %d connected!", instanceID, cs.instanceCount)

	instanceOptions := cs.test.Options
	instanceOptions.ExecutionSegment = cs.ess[instanceID-1]
	instanceOptions.ExecutionSegmentSequence = &cs.ess
	options, err := json.Marshal(instanceOptions)
	if err != nil {
		return nil, err
	}

	return &RegisterResponse{
		Archive: cs.archive,
		Options: options,
	}, nil
}

func (cs *CoordinatorServer) CommandAndControl(stream DistributedTest_CommandAndControlServer) error {
	msgContainer, err := stream.Recv()
	if err != nil {
		return err
	}

	initInstMsg, ok := msgContainer.Message.(*AgentMessage_InitInstanceID)
	if !ok {
		return fmt.Errorf("received wrong message type")
	}

	return cs.cc.handleInstanceStream(initInstMsg.InitInstanceID, stream)
}

type coordinatorController struct {
	logger logrus.FieldLogger

	dataRegistryLock sync.Mutex
	dataRegistry     map[string]*dataWaiter

	signalsLock sync.Mutex
	signals     map[string]*sync.WaitGroup

	instanceCount int
}

type dataWaiter struct {
	once sync.Once
	done chan struct{}
	data []byte
	err  string
}

func newCoordinatorController(instanceCount int, logger logrus.FieldLogger) *coordinatorController {
	return &coordinatorController{
		logger:        logger,
		instanceCount: instanceCount,
		dataRegistry:  make(map[string]*dataWaiter),
		signals:       make(map[string]*sync.WaitGroup),
	}
}

// TODO: split apart and simplify
// nolint: funlen
func (cc *coordinatorController) handleInstanceStream(
	instanceID uint32, stream DistributedTest_CommandAndControlServer,
) (err error) {
	cc.logger.Debugf("Starting to handle command and control stream for instance %d", instanceID)
	defer cc.logger.Infof("Instance %d finished!", instanceID)

	handleSignal := func(id string, wg *sync.WaitGroup) {
		wg.Done()
		wg.Wait()
		err := stream.Send(&ControllerMessage{
			InstanceID: instanceID,
			Message:    &ControllerMessage_DoneWaitWithID{id},
		})
		if err != nil {
			cc.logger.Error(err)
		}
	}
	handleData := func(id string, dw *dataWaiter) {
		thisInstanceCreatedTheData := false
		dw.once.Do(func() {
			err := stream.Send(&ControllerMessage{
				InstanceID: instanceID,
				Message:    &ControllerMessage_CreateDataWithID{id},
			})
			if err != nil {
				cc.logger.Error(err)
			}
			<-dw.done
			thisInstanceCreatedTheData = true
		})
		if thisInstanceCreatedTheData {
			return // nothing to do
		}
		err := stream.Send(&ControllerMessage{
			InstanceID: instanceID,
			Message: &ControllerMessage_DataWithID{DataWithID: &DataPacket{
				Id:    id,
				Data:  dw.data,
				Error: dw.err,
			}},
		})
		if err != nil {
			cc.logger.Error(err)
		}
	}

	for {
		msgContainer, err := stream.Recv()
		if err != nil {
			return err
		}

		switch msg := msgContainer.Message.(type) {
		case *AgentMessage_SignalAndWaitOnID:
			cc.signalsLock.Lock()
			wg, ok := cc.signals[msg.SignalAndWaitOnID]
			if !ok {
				wg = &sync.WaitGroup{}
				wg.Add(cc.instanceCount)
				cc.signals[msg.SignalAndWaitOnID] = wg
			}
			cc.signalsLock.Unlock()
			go handleSignal(msg.SignalAndWaitOnID, wg)

		case *AgentMessage_GetOrCreateDataWithID:
			cc.dataRegistryLock.Lock()
			dw, ok := cc.dataRegistry[msg.GetOrCreateDataWithID]
			if !ok {
				dw = &dataWaiter{
					done: make(chan struct{}),
				}
				cc.dataRegistry[msg.GetOrCreateDataWithID] = dw
			}
			cc.dataRegistryLock.Unlock()
			go handleData(msg.GetOrCreateDataWithID, dw)

		case *AgentMessage_CreatedData:
			cc.dataRegistryLock.Lock()
			dw, ok := cc.dataRegistry[msg.CreatedData.Id]
			if !ok {
				return fmt.Errorf("expected data waiter object for %s to be created already", msg.CreatedData.Id)
			}
			cc.dataRegistryLock.Unlock()
			dw.data = msg.CreatedData.Data
			dw.err = msg.CreatedData.Error
			close(dw.done)
		default:
			return fmt.Errorf("Unknown controller message type '%#v'", msg)
		}
	}
}
