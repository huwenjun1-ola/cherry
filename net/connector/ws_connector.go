package cherryConnector

import (
	"github.com/cherry-game/cherry/error"
	"github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/packet"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"time"
)

type WSConnector struct {
	address           string
	listener          net.Listener
	up                *websocket.Upgrader
	certFile          string
	keyFile           string
	onConnectListener []cherryFacade.OnConnectListener
}

func NewWS(address string) *WSConnector {
	if address == "" {
		cherryLogger.Warn("create websocket fail. address is null.")
		return nil
	}

	ws := &WSConnector{
		address:           address,
		onConnectListener: make([]cherryFacade.OnConnectListener, 0),
	}
	return ws
}

func NewWebsocketLTS(address, certFile, keyFile string) *WSConnector {
	if address == "" {
		cherryLogger.Warn("create websocket fail. address is null.")
		return nil
	}

	if certFile == "" || keyFile == "" {
		cherryLogger.Warn("create websocket fail. certFile or keyFile is null.")
		return nil
	}

	w := &WSConnector{
		address:           address,
		certFile:          certFile,
		keyFile:           keyFile,
		onConnectListener: make([]cherryFacade.OnConnectListener, 0),
	}

	return w
}

func (w *WSConnector) OnStart() {
	if len(w.onConnectListener) < 1 {
		panic("onConnectListener() not set.")
	}

	var err error
	w.listener, err = GetNetListener(w.address, w.certFile, w.keyFile)
	if err != nil {
		cherryLogger.Fatalf("Failed to listen: %s", err.Error())
	}

	if w.certFile == "" || w.keyFile == "" {
		cherryLogger.Infof("websocket connector listening at address ws://%s", w.address)
	} else {
		cherryLogger.Infof("websocket connector listening at address wss://%s", w.address)
		cherryLogger.Infof("certFile = %s", w.certFile)
		cherryLogger.Infof("keyFile = %s", w.keyFile)
	}

	w.up = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     CheckOrigin,
	}

	defer func() {
		if err := w.listener.Close(); err != nil {
			cherryLogger.Error(err)
		}
	}()

	err = http.Serve(w.listener, w)
	if err != nil {
		cherryLogger.Error(err)
	}
}

func (w *WSConnector) OnConnect(listener ...cherryFacade.OnConnectListener) {
	w.onConnectListener = listener
}

func (w *WSConnector) OnStop() {
}

//ServerHTTP server.Handler
func (w *WSConnector) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	wsConn, err := w.up.Upgrade(rw, r, nil)
	if err != nil {
		cherryLogger.Infof("Upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error())
		return
	}

	conn, err := NewWSConn(wsConn)
	if err != nil {
		cherryLogger.Errorf("Failed to create new ws connection: %s", err.Error())
		return
	}

	//new goroutine process socket connection
	go w.processNewConn(conn)
}

func (w *WSConnector) processNewConn(conn cherryFacade.INetConn) {
	for _, listener := range w.onConnectListener {
		listener(conn)
	}
}

// WSConn is an adapter to t.INetConn, which implements all t.INetConn
// interface base on *websocket.INetConn
type WSConn struct {
	conn   *websocket.Conn
	typ    int // message type
	reader io.Reader
}

// NewWSConn return an initialized *WSConn
func NewWSConn(conn *websocket.Conn) (*WSConn, error) {
	c := &WSConn{conn: conn}
	return c, nil
}

// GetNextMessage reads the next message available in the stream
func (c *WSConn) GetNextMessage() (b []byte, err error) {
	_, msgBytes, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if len(msgBytes) < cherryPacket.HeadLength {
		return nil, cherryError.PacketInvalidHeader
	}

	header := msgBytes[:cherryPacket.HeadLength]

	msgSize, _, err := cherryPacket.ParseHeader(header)
	if err != nil {
		return nil, err
	}

	dataLen := len(msgBytes[cherryPacket.HeadLength:])

	if dataLen < msgSize || dataLen > msgSize {
		return nil, cherryError.PacketMsgSmallerThanExpected
	}

	return msgBytes, err
}

func (c *WSConn) Read(b []byte) (int, error) {
	if c.reader == nil {
		t, r, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		c.typ = t
		c.reader = r
	}
	n, err := c.reader.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	} else if err == io.EOF {
		_, r, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		c.reader = r
	}

	return n, nil
}

func (c *WSConn) Write(b []byte) (int, error) {
	err := c.conn.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *WSConn) Close() error {
	return c.conn.Close()
}

func (c *WSConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *WSConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *WSConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	return c.SetWriteDeadline(t)
}

func (c *WSConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WSConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
