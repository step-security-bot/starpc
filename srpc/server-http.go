package srpc

import (
	"context"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

// HTTPServer implements the SRPC server.
type HTTPServer struct {
	mux  Mux
	srpc *Server
	path string
}

// NewHTTPServer builds a http server / handler.
// if path is empty, serves on all routes.
func NewHTTPServer(mux Mux, path string) (*HTTPServer, error) {
	return &HTTPServer{
		mux:  mux,
		srpc: NewServer(mux),
		path: path,
	}, nil
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.path != "" && r.URL.Path != s.path {
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		logrus.Error(err.Error())
		return
	}
	defer c.Close(websocket.StatusInternalError, "closed")

	ctx := r.Context()
	wsConn, err := NewWebSocketConn(ctx, c, true)
	if err != nil {
		logrus.Error(err.Error())
		c.Close(websocket.StatusInternalError, err.Error())
		return
	}

	// handle incoming streams
	for {
		strm, err := wsConn.AcceptStream()
		if err != nil {
			if err != io.EOF && err != context.Canceled {
				logrus.Error(err.Error())
				c.Close(websocket.StatusInternalError, err.Error())
			}
			return
		}
		go func() {
			err := s.srpc.HandleConn(ctx, strm)
			if err != nil && err != io.EOF && err != context.Canceled {
				logrus.Error(err.Error())
			}
		}()
	}
}
