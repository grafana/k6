package distributed

import (
	"bytes"
	context "context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics/engine"
)

// TODO: something more polished...
type CoordinatorServer struct {
	UnimplementedDistributedTestServer
	instanceCount int
	test          *lib.Archive
	logger        logrus.FieldLogger
	metricsEngine *engine.MetricsEngine

	testStartTimeLock sync.Mutex
	testStartTime     *time.Time

	cc              *coordinatorController
	currentInstance int32 // TODO: something a bit better, support full execution plans from JSON?
	ess             lib.ExecutionSegmentSequence
	archive         []byte
	wg              *sync.WaitGroup
}

func NewCoordinatorServer(
	instanceCount int, test *lib.Archive, metricsEngine *engine.MetricsEngine, logger logrus.FieldLogger,
) (*CoordinatorServer, error) {
	segments, err := test.Options.ExecutionSegment.Split(int64(instanceCount))
	if err != nil {
		return nil, err
	}
	ess, err := lib.NewExecutionSegmentSequence(segments...)
	if err != nil {
		return nil, err
	}

	// TODO: figure out some way to add metrics from the instance to the metricsEngine

	buf := &bytes.Buffer{}
	if err = test.Write(buf); err != nil {
		return nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(instanceCount)

	cs := &CoordinatorServer{
		instanceCount: instanceCount,
		test:          test,
		metricsEngine: metricsEngine,
		logger:        logger,
		ess:           ess,
		cc:            newCoordinatorController(instanceCount, logger),
		archive:       buf.Bytes(),
		wg:            wg,
	}

	go cs.monitorProgress()

	return cs, nil
}

func (cs *CoordinatorServer) monitorProgress() {
	wg := cs.cc.getSignalWG("test-start") // TODO: use constant when we refactor scheduler.go
	wg.Wait()
	cs.logger.Info("All instances ready to start initializing VUs...")

	wg = cs.cc.getSignalWG("test-ready-to-run-setup") // TODO: use constant when we refactor scheduler.go
	wg.Wait()
	cs.logger.Info("VUs initialized, setup()...")
	cs.testStartTimeLock.Lock()
	t := time.Now()
	cs.testStartTime = &t
	cs.testStartTimeLock.Unlock()

	wg = cs.cc.getSignalWG("setup-done") // TODO: use constant when we refactor scheduler.go
	wg.Wait()
	cs.logger.Info("setup() done, starting test!")

	wg = cs.cc.getSignalWG("test-done") // TODO: use constant when we refactor scheduler.go
	wg.Wait()
	cs.logger.Info("Instances finished with the test")
}

func (cs *CoordinatorServer) GetCurrentTestRunDuration() time.Duration {
	cs.testStartTimeLock.Lock()
	startTime := cs.testStartTime
	cs.testStartTimeLock.Unlock()

	if startTime == nil {
		return 0
	}
	return time.Since(*startTime)
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
		InstanceID: uint32(instanceID),
		Archive:    cs.archive,
		Options:    options,
	}, nil
}

func (cs *CoordinatorServer) CommandAndControl(stream DistributedTest_CommandAndControlServer) error {
	defer cs.wg.Done()
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

func (cs *CoordinatorServer) SendMetrics(ctx context.Context, dumpMsg *MetricsDump) (*MetricsDumpResponse, error) {
	// TODO: something nicer?
	for _, md := range dumpMsg.Metrics {
		if err := cs.metricsEngine.ImportMetric(md.Name, md.Data); err != nil {
			cs.logger.Errorf("Error merging sink for metric %s: %w", md.Name, err)
			// return nil, err
		}
	}
	return &MetricsDumpResponse{}, nil
}

func (cs *CoordinatorServer) Wait() {
	cs.wg.Wait()
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

func (cc *coordinatorController) getSignalWG(signalID string) *sync.WaitGroup {
	cc.signalsLock.Lock()
	wg, ok := cc.signals[signalID]
	if !ok {
		wg = &sync.WaitGroup{}
		wg.Add(cc.instanceCount)
		cc.signals[signalID] = wg
	}
	cc.signalsLock.Unlock()
	return wg
}

func (cc *coordinatorController) getDataWaiter(dwID string) *dataWaiter {
	cc.dataRegistryLock.Lock()
	dw, ok := cc.dataRegistry[dwID]
	if !ok {
		dw = &dataWaiter{
			done: make(chan struct{}),
		}
		cc.dataRegistry[dwID] = dw
	}
	cc.dataRegistryLock.Unlock()
	return dw
}

// TODO: split apart and simplify
func (cc *coordinatorController) handleInstanceStream(
	instanceID uint32, stream DistributedTest_CommandAndControlServer,
) (err error) {
	cc.logger.Debug("Starting to handle command and control stream for instance %d", instanceID)
	defer cc.logger.Infof("Instance %d disconnected", instanceID)

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
			wg := cc.getSignalWG(msg.SignalAndWaitOnID)
			go handleSignal(msg.SignalAndWaitOnID, wg)

		case *AgentMessage_GetOrCreateDataWithID:
			dw := cc.getDataWaiter(msg.GetOrCreateDataWithID)
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
