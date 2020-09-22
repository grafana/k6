package dns

import (
	"io"
	"sync"
	"time"
)

type pipeline struct {
	Conn

	rmu, wmu sync.Mutex

	mu       sync.Mutex
	inflight map[int]pipelineTx
	readerr  error
}

func (p *pipeline) alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.readerr == nil
}

func (p *pipeline) conn() Conn {
	return &pipelineConn{
		pipeline: p,
		tx: pipelineTx{
			msgerrc: make(chan msgerr),
			abortc:  make(chan struct{}),
		},
	}
}

func (p *pipeline) run() {
	var err error
	for {
		var msg Message

		p.rmu.Lock()
		if err = p.Recv(&msg); err != nil {
			break
		}
		p.rmu.Unlock()

		p.mu.Lock()
		tx, ok := p.inflight[msg.ID]
		delete(p.inflight, msg.ID)
		p.mu.Unlock()

		if !ok {
			continue
		}

		go tx.deliver(msgerr{msg: &msg})
	}
	p.rmu.Unlock()

	p.mu.Lock()
	p.readerr = err
	txs := make([]pipelineTx, 0, len(p.inflight))
	for _, tx := range p.inflight {
		txs = append(txs, tx)
	}
	p.mu.Unlock()

	for _, tx := range txs {
		go tx.deliver(msgerr{err: err})
	}
}

type pipelineConn struct {
	*pipeline

	aborto sync.Once
	tx     pipelineTx

	readDeadline, writeDeadline time.Time
}

func (c *pipelineConn) Close() error {
	c.aborto.Do(c.tx.abort)
	return nil
}

func (c *pipelineConn) Recv(msg *Message) error {
	var me msgerr
	select {
	case me = <-c.tx.msgerrc:
	case <-c.tx.abortc:
		return io.ErrUnexpectedEOF
	}

	if err := me.err; err != nil {
		return err
	}

	*msg = *me.msg // shallow copy
	return nil
}

func (c *pipelineConn) Send(msg *Message) error {
	if err := c.register(msg); err != nil {
		return err
	}

	c.wmu.Lock()
	defer c.wmu.Unlock()

	if err := c.Conn.SetWriteDeadline(c.writeDeadline); err != nil {
		return err
	}

	return c.Conn.Send(msg)
}

func (c *pipelineConn) SetDeadline(t time.Time) error {
	c.SetReadDeadline(t)
	c.SetWriteDeadline(t)
	return nil
}

func (c *pipelineConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *pipelineConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

func (c *pipelineConn) register(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.inflight[msg.ID]; ok {
		return ErrConflictingID
	}

	c.inflight[msg.ID] = c.tx
	return nil
}

type pipelineTx struct {
	msgerrc chan msgerr
	abortc  chan struct{}
}

func (p pipelineTx) abort() { close(p.abortc) }

func (p pipelineTx) deliver(me msgerr) {
	select {
	case p.msgerrc <- me:
	case <-p.abortc:
	}
}
