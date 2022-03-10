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
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// TODO: something more polished...
type CoordinatorServer struct {
	UnimplementedDistributedTestServer
	instanceCount int
	test          *lib.Archive
	logger        logrus.FieldLogger

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
		archive:       buf.Bytes(),
	}
	return cs, nil
}

func (cs *CoordinatorServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	instanceID := atomic.AddInt32(&cs.currentInstance, 1)
	if instanceID > int32(cs.instanceCount) {
		return nil, fmt.Errorf("we don't need any more instances")
	}
	cs.logger.Infof("Instance %d connected!", instanceID)

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
	return status.Errorf(codes.Unimplemented, "method CommandAndControl not implemented")
}

// Controller controls distributed tests.
type CoordinatorController struct {
	dataRegistryLock sync.Mutex
	dataRegistry     map[string]dataEntry

	signalsLock sync.Mutex
	signals     map[string]*sync.WaitGroup

	instanceCount int
}

type dataEntry struct {
	once *sync.Once
	data []byte
}

func NewCoordinatorController(instanceCount int) *CoordinatorController {
	return &CoordinatorController{instanceCount: instanceCount}
}

func (c *CoordinatorController) GetOrCreateData(id string, callback func() ([]byte, error)) ([]byte, error) {
	return callback()
}

func (c *CoordinatorController) SignalAndWait(eventId string) error {
	return nil
}
