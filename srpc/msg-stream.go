package srpc

import (
	"context"
)

// MsgStreamRw is the read-write interface for MsgStream.
type MsgStreamRw interface {
	// ReadOne reads a single message and returns.
	//
	// returns io.EOF if the stream ended.
	ReadOne() ([]byte, error)
	// WriteCallData writes a call data packet.
	WriteCallData(data []byte, complete bool, err error) error
}

// MsgStream implements the stream interface passed to implementations.
type MsgStream struct {
	// ctx is the stream context
	ctx context.Context
	// rw is the msg stream read-writer
	rw MsgStreamRw
	// closeCb is the close callback
	closeCb func()
}

// NewMsgStream constructs a new Stream with a ClientRPC.
// dataCh should be closed when no more messages will arrive.
func NewMsgStream(
	ctx context.Context,
	rw MsgStreamRw,
	closeCb func(),
) *MsgStream {
	return &MsgStream{
		ctx:     ctx,
		rw:      rw,
		closeCb: closeCb,
	}
}

// Context is canceled when the Stream is no longer valid.
func (r *MsgStream) Context() context.Context {
	return r.ctx
}

// MsgSend sends the message to the remote.
func (r *MsgStream) MsgSend(msg Message) error {
	select {
	case <-r.ctx.Done():
		return context.Canceled
	default:
	}

	msgData, err := msg.MarshalVT()
	if err != nil {
		return err
	}
	return r.rw.WriteCallData(msgData, false, nil)
}

// MsgRecv receives an incoming message from the remote.
// Parses the message into the object at msg.
func (r *MsgStream) MsgRecv(msg Message) error {
	data, err := r.rw.ReadOne()
	if err != nil {
		return err
	}
	return msg.UnmarshalVT(data)
}

// CloseSend signals to the remote that we will no longer send any messages.
func (r *MsgStream) CloseSend() error {
	return r.rw.WriteCallData(nil, true, nil)
}

// Close closes the stream.
func (r *MsgStream) Close() error {
	_ = r.CloseSend()
	if r.closeCb != nil {
		r.closeCb()
	}

	return nil
}

// _ is a type assertion
var _ Stream = ((*MsgStream)(nil))
