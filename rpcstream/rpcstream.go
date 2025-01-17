package rpcstream

import (
	"bytes"
	"context"
	"io"

	"github.com/aperturerobotics/starpc/srpc"
	"github.com/pkg/errors"
)

// RpcStream implements a RPC call stream over a RPC call. Used to implement
// sub-components which have a different set of services & calls available.
type RpcStream interface {
	srpc.Stream
	Send(*RpcStreamPacket) error
	Recv() (*RpcStreamPacket, error)
}

// RpcStreamGetter returns the Mux for the component ID from the remote.
// Returns a release function to call when done with the Mux.
// Returns nil, nil, nil if not found.
type RpcStreamGetter func(ctx context.Context, componentID string) (srpc.Invoker, func(), error)

// RpcStreamCaller is a function which starts the RpcStream call.
type RpcStreamCaller[T RpcStream] func(ctx context.Context) (T, error)

// OpenRpcStream opens a RPC stream with a remote.
//
// if waitAck is set, waits for acknowledgment from the remote before returning.
func OpenRpcStream[T RpcStream](ctx context.Context, rpcCaller RpcStreamCaller[T], componentID string, waitAck bool) (io.ReadWriteCloser, error) {
	// open the rpc stream
	rpcStream, err := rpcCaller(ctx)
	if err != nil {
		return nil, err
	}

	// write the component id
	err = rpcStream.Send(&RpcStreamPacket{
		Body: &RpcStreamPacket_Init{
			Init: &RpcStreamInit{
				ComponentId: componentID,
			},
		},
	})
	if err != nil {
		_ = rpcStream.Close()
		return nil, err
	}

	// wait for ack
	if waitAck {
		pkt, err := rpcStream.Recv()
		if err == nil {
			switch b := pkt.GetBody().(type) {
			case *RpcStreamPacket_Ack:
				if errStr := b.Ack.GetError(); errStr != "" {
					err = errors.Errorf("remote: %s", errStr)
				}
			default:
				err = errors.New("expected ack packet")
			}
		}
		if err != nil {
			_ = rpcStream.Close()
			return nil, err
		}
	}

	// ready
	rw := NewRpcStreamReadWriter(rpcStream)
	return rw, nil
}

// NewRpcStreamOpenStream constructs an OpenStream function with a RpcStream.
//
// if waitAck is set, OpenStream waits for acknowledgment from the remote.
func NewRpcStreamOpenStream[T RpcStream](rpcCaller RpcStreamCaller[T], componentID string, waitAck bool) srpc.OpenStreamFunc {
	return func(ctx context.Context, msgHandler srpc.PacketHandler, closeHandler srpc.CloseHandler) (srpc.Writer, error) {
		// open the stream
		rw, err := OpenRpcStream(ctx, rpcCaller, componentID, waitAck)
		if err != nil {
			return nil, err
		}

		// construct the packet conn
		prw := srpc.NewPacketReadWriter(rw)

		// start the read pump
		go prw.ReadPump(msgHandler, closeHandler)

		// return the writer
		return prw, nil
	}
}

// NewRpcStreamClient constructs a Client which opens streams with a RpcStream.
//
// if waitAck is set, OpenStream waits for acknowledgment from the remote.
func NewRpcStreamClient[T RpcStream](rpcCaller RpcStreamCaller[T], componentID string, waitAck bool) srpc.Client {
	openStream := NewRpcStreamOpenStream(rpcCaller, componentID, waitAck)
	return srpc.NewClient(openStream)
}

// HandleRpcStream handles an incoming RPC stream (remote is the initiator).
func HandleRpcStream(stream RpcStream, getter RpcStreamGetter) error {
	// Read the "init" packet.
	initPkt, err := stream.Recv()
	if err != nil {
		return err
	}
	initInner, ok := initPkt.GetBody().(*RpcStreamPacket_Init)
	if !ok || initInner.Init == nil {
		return errors.New("expected init packet")
	}
	componentID := initInner.Init.GetComponentId()
	if componentID == "" {
		return errors.New("invalid init packet: empty component id")
	}

	// lookup the server for this component id
	ctx := stream.Context()
	mux, muxRel, err := getter(ctx, componentID)
	if err == nil && mux == nil {
		err = errors.New("no server for that component")
	}
	if mux != nil && muxRel != nil {
		defer muxRel()
	}

	// send ack
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	sendErr := stream.Send(&RpcStreamPacket{
		Body: &RpcStreamPacket_Ack{
			Ack: &RpcAck{Error: errStr},
		},
	})
	if err != nil {
		return err
	}
	if sendErr != nil {
		return sendErr
	}

	// handle the rpc
	srw := NewRpcStreamReadWriter(stream)
	prw := srpc.NewPacketReadWriter(srw)
	serverRPC := srpc.NewServerRPC(ctx, mux, prw)
	go prw.ReadPump(serverRPC.HandlePacket, serverRPC.HandleStreamClose)
	return serverRPC.Wait(ctx)
}

// RpcStreamReadWriter reads and writes a buffered RpcStream.
type RpcStreamReadWriter struct {
	// stream is the RpcStream
	stream RpcStream
	// buf is the incoming data buffer
	buf bytes.Buffer
}

// NewRpcStreamReadWriter constructs a new read/writer.
func NewRpcStreamReadWriter(stream RpcStream) *RpcStreamReadWriter {
	return &RpcStreamReadWriter{stream: stream}
}

// Write writes a packet to the writer.
func (r *RpcStreamReadWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	err = r.stream.Send(&RpcStreamPacket{
		Body: &RpcStreamPacket_Data{
			Data: p,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Read reads a packet from the writer.
func (r *RpcStreamReadWriter) Read(p []byte) (n int, err error) {
	readBuf := p
	for len(readBuf) != 0 && err == nil {
		var rn int

		// if the buffer has data, read from it.
		if r.buf.Len() != 0 {
			rn, err = r.buf.Read(readBuf)
		} else {
			if n != 0 {
				// if we read data to p already, return now.
				break
			}

			var pkt *RpcStreamPacket
			pkt, err = r.stream.Recv()
			if err != nil {
				break
			}

			if errStr := pkt.GetAck().GetError(); errStr != "" {
				return n, errors.New(errStr)
			}

			data := pkt.GetData()
			if len(data) == 0 {
				continue
			}

			// read as much as possible directly to the output
			copy(readBuf, data)
			if len(data) > len(readBuf) {
				// we read some of the data, buffer the rest.
				rn = len(readBuf)
				_, _ = r.buf.Write(data[rn:]) // never returns an error
			} else {
				// we read all of data
				rn = len(data)
			}
		}

		// advance readBuf by rn
		n += rn
		readBuf = readBuf[rn:]
	}
	return n, err
}

// Close closes the packet rw.
func (r *RpcStreamReadWriter) Close() error {
	return r.stream.Close()
}

// _ is a type assertion
var _ io.ReadWriteCloser = (*RpcStreamReadWriter)(nil)
