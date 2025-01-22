package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"go.k6.io/k6/internal/lib/netext/grpcext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// message is a struct that
type message struct {
	isClosing bool
	msg       []byte
}

const (
	opened = iota + 1
	closed
)

const (
	timestampMetadata = "ts"
)

type stream struct {
	vu     modules.VU
	client *Client

	logger logrus.FieldLogger

	methodDescriptor protoreflect.MethodDescriptor

	method string
	stream *grpcext.Stream

	tagsAndMeta *metrics.TagsAndMeta
	tq          *taskqueue.TaskQueue

	instanceMetrics *instanceMetrics
	builtinMetrics  *metrics.BuiltinMetrics

	obj *sobek.Object // the object that is given to js to interact with the stream

	writingState int8
	done         chan struct{}

	writeQueueCh chan message

	eventListeners *eventListeners

	timeoutCancel context.CancelFunc
}

// defineStream defines the sobek.Object that is given to js to interact with the Stream
func defineStream(rt *sobek.Runtime, s *stream) {
	must(rt, s.obj.DefineDataProperty(
		"on", rt.ToValue(s.on), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, s.obj.DefineDataProperty(
		"write", rt.ToValue(s.write), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, s.obj.DefineDataProperty(
		"end", rt.ToValue(s.end), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
}

func (s *stream) beginStream(p *callParams) error {
	req := &grpcext.StreamRequest{
		Method:                 s.method,
		MethodDescriptor:       s.methodDescriptor,
		DiscardResponseMessage: p.DiscardResponseMessage,
		TagsAndMeta:            &p.TagsAndMeta,
		Metadata:               p.Metadata,
	}

	ctx := s.vu.Context()
	var cancel context.CancelFunc

	if p.Timeout != time.Duration(0) {
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
	}

	s.timeoutCancel = cancel

	stream, err := s.client.conn.NewStream(ctx, *req)
	if err != nil {
		return fmt.Errorf("failed to create a new stream: %w", err)
	}
	s.stream = stream
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: s.instanceMetrics.Streams,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     time.Now(),
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})

	go s.loop()

	return nil
}

func (s *stream) loop() {
	ctx := s.vu.Context()
	wg := new(sync.WaitGroup)

	defer func() {
		wg.Wait()
		s.tq.Close()
	}()

	// read & write data from/to the stream
	wg.Add(2)
	go s.readData(wg)
	go s.writeData(wg)

	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			// VU is shutting down during an interrupt
			// stream events will not be forwarded to the VU
			s.tq.Queue(func() error {
				return s.closeWithError(nil)
			})
			return
		case <-s.done:
			return
		}
	}
}

func (s *stream) queueMessage(msg interface{}) {
	now := time.Now()
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: s.instanceMetrics.StreamsMessagesReceived,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     now,
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})

	s.tq.Queue(func() error {
		rt := s.vu.Runtime()
		listeners := s.eventListeners.all(eventData)

		metadataObj := rt.NewObject()
		err := metadataObj.Set(timestampMetadata, rt.ToValue(now.Unix()))
		if err != nil {
			return err
		}

		for _, messageListener := range listeners {
			if _, err := messageListener(rt.ToValue(msg), metadataObj); err != nil {
				// TODO(olegbespalov) consider logging the error
				_ = s.closeWithError(err)

				return err
			}
		}
		return nil
	})
}

// readData reads data from the stream and forward them to the readDataChan
func (s *stream) readData(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		msg, err := s.stream.ReceiveConverted()

		if err != nil && !isRegularClosing(err) {
			s.logger.WithError(err).Debug("error while reading from the stream")

			s.tq.Queue(func() error {
				return s.closeWithError(err)
			})

			return
		}

		if isRegularClosing(err) {
			s.logger.WithError(err).Debug("stream is cancelled/finished")

			s.tq.Queue(func() error {
				return s.closeWithError(err)
			})

			return
		}

		if msg != nil || !reflect.ValueOf(msg).IsNil() {
			s.queueMessage(msg)
		}
	}
}

func isRegularClosing(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, grpcext.ErrCanceled)
}

// writeData writes data to the stream
func (s *stream) writeData(wg *sync.WaitGroup) {
	defer wg.Done()

	writeChannel := make(chan message)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case msg, ok := <-writeChannel:
				if !ok {
					return
				}

				if msg.isClosing {
					err := s.stream.CloseSend()
					if err != nil {
						s.logger.WithError(err).Error("an error happened during stream closing")
					}

					s.tq.Queue(func() error {
						return s.closeWithError(err)
					})

					return
				}

				err := s.stream.Send(msg.msg)
				if err != nil {
					s.processSendError(err)
					return
				}

				metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: s.instanceMetrics.StreamsMessagesSent,
						Tags:   s.tagsAndMeta.Tags,
					},
					Time:     time.Now(),
					Metadata: s.tagsAndMeta.Metadata,
					Value:    1,
				})
			case <-s.done:
				return
			}
		}
	}()

	{
		defer close(writeChannel)

		queue := make([]message, 0)
		var wch chan message
		var msg message

		for {
			wch = nil // this way if nothing to read it will just block
			if len(queue) > 0 {
				msg = queue[0]
				wch = writeChannel
			}
			select {
			case msg = <-s.writeQueueCh:
				queue = append(queue, msg)
			case wch <- msg:
				queue = queue[:copy(queue, queue[1:])]

			case <-s.done:
				return
			}
		}
	}
}

func (s *stream) processSendError(err error) {
	if errors.Is(err, io.EOF) {
		s.logger.WithError(err).Debug("skip sending a message stream is cancelled/finished")
		err = nil
	}

	s.tq.Queue(func() error {
		return s.closeWithError(err)
	})
}

// on registers a handler for a certain event type
func (s *stream) on(event string, handler func(sobek.Value, sobek.Value) (sobek.Value, error)) {
	if handler == nil {
		common.Throw(s.vu.Runtime(), fmt.Errorf("handler for %q event isn't a callable function", event))
	}

	if err := s.eventListeners.add(event, handler); err != nil {
		s.vu.State().Logger.Warnf("can't register %s event handler: %s", event, err)
	}
}

// write writes a message to the stream
func (s *stream) write(input sobek.Value) {
	if s.writingState != opened {
		return
	}

	if common.IsNullish(input) {
		s.logger.Warnf("can't send empty message")
		return
	}

	rt := s.vu.Runtime()

	b, err := input.ToObject(rt).MarshalJSON()
	if err != nil {
		s.logger.WithError(err).Warnf("can't marshal message")
	}

	s.writeQueueCh <- message{msg: b}
}

// end closes client the stream
func (s *stream) end() {
	if s.writingState == closed {
		return
	}

	s.logger.Debugf("finishing stream %s writing", s.method)

	s.writingState = closed
	s.writeQueueCh <- message{isClosing: true}
}

func (s *stream) closeWithError(err error) error {
	s.close(err)

	return s.callErrorListeners(err)
}

// close closes the stream and call end event listeners
// Note: in the regular closing the io.EOF could come
func (s *stream) close(err error) {
	if err == nil {
		return
	}

	select {
	case <-s.done:
		s.logger.Debugf("stream %v is already closed", s.method)
		return
	default:
	}

	s.logger.Debugf("stream %s is closing", s.method)
	close(s.done)

	s.tq.Queue(func() error {
		return s.callEventListeners(eventEnd)
	})

	if s.timeoutCancel != nil {
		s.timeoutCancel()
	}
}

func (s *stream) callErrorListeners(e error) error {
	if e == nil || errors.Is(e, io.EOF) {
		return nil
	}

	rt := s.vu.Runtime()

	obj := extractError(e)

	list := s.eventListeners.all(eventError)

	if len(list) == 0 {
		s.logger.Warnf("no handlers for error registered, but an error happened: %s", e)
	}

	now := time.Now()
	metadataObj := rt.NewObject()
	err := metadataObj.Set(timestampMetadata, rt.ToValue(now.Unix()))
	if err != nil {
		return err
	}

	for _, errorListener := range list {
		if _, err := errorListener(rt.ToValue(obj), metadataObj); err != nil {
			return err
		}
	}
	return nil
}

type grpcError struct {
	// Code is a gRPC error code.
	Code codes.Code `json:"code"`
	// Details is a list details attached to the error.
	Details []interface{} `json:"details"`
	// Message is the original error message.
	Message string `json:"message"`
}

// Error to satisfy the error interface.
func (e grpcError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// extractError tries to extract error information from an error.
// If the error is not a gRPC error, it will be wrapped into a gRPC error.
func extractError(e error) grpcError {
	grpcStatus := status.Convert(e)

	w := grpcError{
		Code:    grpcStatus.Code(),
		Details: grpcStatus.Details(),
		Message: grpcStatus.Message(),
	}

	// fallback to the original error message
	if w.Message == "" {
		w.Message = e.Error()
	}

	return w
}

func (s *stream) callEventListeners(eventType string) error {
	now := time.Now()
	rt := s.vu.Runtime()

	metadataObj := rt.NewObject()
	err := metadataObj.Set(timestampMetadata, rt.ToValue(now.Unix()))
	if err != nil {
		return err
	}
	for _, listener := range s.eventListeners.all(eventType) {
		if _, err := listener(rt.ToValue(struct{}{}), metadataObj); err != nil {
			return err
		}
	}
	return nil
}

// must is a small helper that will panic if err is not nil.
func must(rt *sobek.Runtime, err error) {
	if err != nil {
		common.Throw(rt, err)
	}
}
