package wirenet

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/mediabuyerbot/go-wirenet/pb"

	"github.com/google/uuid"

	"github.com/hashicorp/yamux"
)

const (
	ClientSide Role = 1
	ServerSide Role = 2
)

type Role int

func (s Role) String() (side string) {
	switch s {
	case ClientSide:
		side = "client side"
	case ServerSide:
		side = "server side"
	default:
		side = "unknown"
	}
	return side
}

type (
	SessionHook    func(Session)
	RetryPolicy    func(min, max time.Duration, attemptNum int) time.Duration
	Handler        func(context.Context, Stream)
	Sessions       map[uuid.UUID]Session
	TokenValidator func(streamName string, id Identification, token Token) error
)

type Wire interface {
	Sessions() Sessions
	Session(sessionID uuid.UUID) (Session, error)
	Mount(name string, h Handler)
	Stream(name string) (Stream, error)
	Close() error
	Connect() error
}

type wire struct {
	addr string

	readTimeout      time.Duration
	writeTimeout     time.Duration
	sessCloseTimeout time.Duration

	role          Role
	openSessHook  SessionHook
	closeSessHook SessionHook
	onConnect     func(io.Closer)
	transportConf *yamux.Config

	closed      bool
	conn        bool
	connCounter int
	closeCh     chan chan *ShutdownError
	waitCh      chan struct{}

	retryWaitMin time.Duration
	retryWaitMax time.Duration
	retryMax     int
	retryPolicy  RetryPolicy

	sessions    Sessions
	streamIndex map[string]Session

	token          Token
	verifyToken    TokenValidator
	identification Identification

	tlsConfig *tls.Config

	handlers map[string]Handler
	mu       sync.RWMutex
}

func newWire(addr string, role Role, opts ...Option) (Wire, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(addr) == 0 {
		return nil, ErrAddrEmpty
	}
	wire := &wire{
		addr:     addr,
		handlers: make(map[string]Handler),

		readTimeout:      DefaultReadTimeout,
		writeTimeout:     DefaultWriteTimeout,
		sessCloseTimeout: DefaultSessionCloseTimeout,

		sessions:    make(Sessions),
		streamIndex: make(map[string]Session),

		role:          role,
		openSessHook:  func(Session) {},
		closeSessHook: func(Session) {},
		closeCh:       make(chan chan *ShutdownError),
		onConnect:     func(_ io.Closer) {},

		retryMax:     DefaultRetryMax,
		retryWaitMin: DefaultRetryWaitMin,
		retryWaitMax: DefaultRetryWaitMax,
		retryPolicy:  DefaultRetryPolicy,

		transportConf: &yamux.Config{
			AcceptBacklog:          DefaultAcceptBacklog,
			EnableKeepAlive:        DefaultEnableKeepAlive,
			KeepAliveInterval:      DefaultKeepAliveInterval,
			ConnectionWriteTimeout: DefaultWriteTimeout,
			MaxStreamWindowSize:    DefaultAcceptBacklog * 1024,
			LogOutput:              os.Stderr,
		},
	}
	for _, opt := range opts {
		opt(wire)
	}
	return wire, nil
}

func Server(addr string, opts ...Option) (Wire, error) {
	return newWire(addr, ServerSide, opts...)
}

func Client(addr string, opts ...Option) (Wire, error) {
	return newWire(addr, ClientSide, opts...)
}

func (w *wire) Session(sid uuid.UUID) (Session, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	sess, found := w.sessions[sid]
	if !found {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

func (w *wire) Stream(name string) (Stream, error) {
	w.mu.RLock()
	if len(w.sessions) == 0 || w.closed {
		w.mu.RUnlock()
		return nil, ErrSessionClosed
	}
	sess, found := w.streamIndex[name]
	if !found {
		w.mu.RUnlock()
		return nil, ErrStreamNotFound
	}
	w.mu.RUnlock()
	return sess.OpenStream(name)
}

func (w *wire) Sessions() Sessions {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessions
}

func (w *wire) Connect() (err error) {
	switch w.role {
	case ClientSide:
		err = w.acceptClient()
	case ServerSide:
		err = w.acceptServer()
	}
	return err
}

func (w *wire) Mount(name string, h Handler) {
	w.handlers[name] = h
}

func (w *wire) isClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed
}

func (w *wire) isConnOk() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.conn
}

func (w *wire) setConnFlag(flag bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn = flag
}

func (w *wire) Close() (err error) {
	closeup := func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
	}

	if !w.isConnOk() {
		closeup()
		return nil
	}

	if w.isClosed() {
		return ErrWireClosed
	}

	closeup()

	return w.close()
}

func (w *wire) close() (err error) {
	errCh := make(chan *ShutdownError)
	w.closeCh <- errCh
	shutErr := <-errCh
	if shutErr.IsFilled() {
		err = shutErr
	}
	return err
}

func (w *wire) acceptClient() (err error) {
	tryClose := func(closer io.Closer) {
		w.setConnFlag(false)
		closer.Close()
	}
	for {
		attemptNum := w.connCounter
		if attemptNum >= w.retryMax || w.isClosed() {
			break
		}

		w.connCounter++
		w.setConnFlag(false)

		conn, err := w.dial()
		if err != nil {
			log.Println("dial error", err)
			retryWait := w.retryPolicy(
				w.retryWaitMin,
				w.retryWaitMax,
				attemptNum)

			timeout := time.Now().Add(retryWait)
			for {
				if (timeout.Unix() <= time.Now().Unix()) || w.isClosed() {
					break
				}
				time.Sleep(time.Second)
			}
			continue
		}

		w.setConnFlag(true)
		w.connCounter = 0

		wrapConn, serveErr := yamux.Client(conn, w.transportConf)
		if serveErr != nil {
			tryClose(conn)
			return serveErr
		}

		sid, remoteStreamNames, sErr := w.openSession(wrapConn)
		if sErr != nil {
			tryClose(conn)
			return sErr
		}

		w.waitCh = make(chan struct{})
		w.closeCh = make(chan chan *ShutdownError)

		go w.shutdown(wrapConn)
		go w.onConnect(w)

		openSession(sid, w.identification, wrapConn, w, remoteStreamNames)

		<-w.waitCh
	}
	return err
}

func (w *wire) listen() (listener net.Listener, err error) {
	if w.tlsConfig != nil {
		listener, err = tls.Listen("tcp", w.addr, w.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", w.addr)
	}
	return listener, err
}

func (w *wire) dial() (conn net.Conn, err error) {
	if w.tlsConfig != nil {
		conn, err = tls.Dial("tcp", w.addr, w.tlsConfig)
	} else {
		conn, err = net.Dial("tcp", w.addr)
	}
	return
}

func (w *wire) acceptServer() (err error) {
	listener, err := w.listen()
	if err != nil {
		return err
	}
	defer func() {
		listener.Close()
		w.setConnFlag(false)
	}()

	go w.shutdown(listener)
	go w.onConnect(w)

	w.setConnFlag(true)

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if !w.isClosed() {
				err = acceptErr
			}
			break
		}

		// close all new connections
		if w.isClosed() {
			_ = conn.Close()
			continue
		}

		wrapConn, serveErr := yamux.Server(conn, w.transportConf)
		if serveErr != nil {
			err = serveErr
			break
		}

		sid, id, localStreamNames, sErr := w.confirmSession(wrapConn, w.verifyToken)
		if sErr != nil {
			conn.Close()
			continue
		}

		openSession(sid, id, wrapConn, w, localStreamNames)
	}
	return err
}

func (w *wire) shutdown(conn io.Closer) {
	errCh, ok := <-w.closeCh
	if !ok {
		return
	}

	shutdownErr := NewShutdownError()
	w.mu.RLock()
	sessions := make([]Session, 0, len(w.sessions))
	for _, sess := range w.sessions {
		sessions = append(sessions, sess)
	}
	w.mu.RUnlock()

	var err error
	for _, sess := range sessions {
		err = sess.Close()
		if err != nil && err != ErrSessionClosed {
			shutdownErr.Add(err)
		}
	}

	if err := conn.Close(); err != nil {
		shutdownErr.Add(err)
	}

	if w.role == ClientSide {
		close(w.waitCh)
	}

	errCh <- shutdownErr
}

func (w *wire) hasSession(s Session) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, found := w.sessions[s.ID()]
	return found
}

func (w *wire) registerSession(s Session) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sessions[s.ID()] = s
	for _, streamName := range s.StreamNames() {
		w.streamIndex[streamName] = s
	}
}

func (w *wire) unregisterSession(s Session) {
	var sl int
	w.mu.Lock()
	for _, streamName := range s.StreamNames() {
		delete(w.streamIndex, streamName)
	}
	delete(w.sessions, s.ID())
	sl = len(w.sessions)
	w.mu.Unlock()

	forcedCloseClient := w.role == ClientSide && sl == 0 && !w.isClosed()
	if forcedCloseClient {
		w.close()
	}
}

func (w *wire) openSession(conn *yamux.Session) (sid uuid.UUID, rsn []string, err error) {
	stream, err := conn.OpenStream()
	if err != nil {
		return sid, rsn, err
	}
	defer stream.Close()

	if err := stream.SetDeadline(deadline(w)); err != nil {
		return sid, rsn, err
	}

	sid = uuid.New()
	sessID, err := sid.MarshalBinary()
	if err != nil {
		return sid, rsn, err
	}

	remoteStreamNames, err := openSessionRequest(
		stream,
		sessID,
		w.token,
		w.identification,
		localStreamNames(w.handlers))
	if err != nil {
		return sid, nil, err
	}

	return sid, remoteStreamNames, nil
}

func (w *wire) confirmSession(conn *yamux.Session, tv TokenValidator) (sid uuid.UUID, id Identification, lsn []string, err error) {
	stream, err := conn.AcceptStream()
	if err != nil {
		return sid, nil, nil, err
	}
	defer stream.Close()

	if err := stream.SetDeadline(deadline(w)); err != nil {
		return sid, nil, lsn, err
	}

	return confirmSessionRequest(
		stream,
		w.verifyToken,
		localStreamNames(w.handlers),
	)
}

func validateRole(r Role) error {
	switch r {
	case ClientSide, ServerSide:
		return nil
	default:
		return ErrUnknownRole
	}
}

func localStreamNames(m map[string]Handler) (names []string) {
	names = make([]string, 0, len(m))
	for n, _ := range m {
		names = append(names, n)
	}
	return
}

func deadline(w *wire) time.Time {
	return time.Now().Add(w.transportConf.ConnectionWriteTimeout)
}

func confirmSessionRequest(conn *yamux.Stream, fn TokenValidator, localStreamNames []string) (sid uuid.UUID, id Identification, lsn []string, err error) {
	frm, err := newDecoder(conn).Decode()
	if err != nil {
		return sid, nil, nil, err
	}
	var req pb.OpenSessionRequest
	if err := proto.Unmarshal(frm.Payload(), &req); err != nil {
		return sid, nil, nil, err
	}

	resp := &pb.OpenSessionResponse{
		Sid:               req.Sid,
		RemoteStreamNames: localStreamNames,
	}
	if fn != nil {
		if err := fn("confirmSession", req.Identification, req.Token); err != nil {
			resp.Err = err.Error()
		}
	}
	p, err := proto.Marshal(resp)
	if err != nil {
		return sid, nil, nil, err
	}
	if err := newEncoder(conn).Encode(confSessType, "confirmSession", p); err != nil {
		return sid, nil, nil, err
	}
	sid, err = uuid.FromBytes(req.Sid)
	return sid, req.Identification, req.LocalStreamNames, err
}

func openSessionRequest(conn *yamux.Stream, sessID []byte, token Token, id Identification, localStreamNames []string) ([]string, error) {
	payload, err := proto.Marshal(&pb.OpenSessionRequest{
		Sid:              sessID,
		Token:            token,
		LocalStreamNames: localStreamNames,
		Identification:   id,
	})
	if err != nil {
		return nil, err
	}
	if err := newEncoder(conn).Encode(openSessTyp, "openSession", payload); err != nil {
		return nil, err
	}
	frm, err := newDecoder(conn).Decode()
	if err != nil {
		return nil, err
	}
	var resp pb.OpenSessionResponse
	if err := proto.Unmarshal(frm.Payload(), &resp); err != nil {
		return nil, err
	}
	if len(resp.Err) > 0 {
		return nil, errors.New(resp.Err)
	}
	return resp.RemoteStreamNames, nil
}
