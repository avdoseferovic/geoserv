package protocol

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ethanmoffat/eolib-go/v3/data"
	"github.com/gorilla/websocket"
)

// WebSocketConn is the interface required for WebSocket connections.
// This allows both gorilla/websocket connections and custom implementations
// (like hijacked HTTP connections) to work with the protocol layer.
type WebSocketConn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (messageType int, p []byte, err error)
	SetWriteDeadline(time.Time) error
	WriteMessage(messageType int, data []byte) error
	Close() error
	RemoteAddr() net.Addr
}

const writeDeadline = 10 * time.Second

// ConnType distinguishes TCP from WebSocket connections.
type ConnType int

const (
	ConnTCP ConnType = iota
	ConnWebSocket
)

// Conn wraps either a raw TCP connection or a WebSocket connection,
// providing a unified interface for reading and writing EO packets.
type Conn struct {
	connType ConnType
	tcp      net.Conn
	ws       WebSocketConn
	wsMu     sync.Mutex // websocket writes must be serialized
}

// NewTCPConn wraps a raw TCP connection.
func NewTCPConn(conn net.Conn) *Conn {
	return &Conn{connType: ConnTCP, tcp: conn}
}

// NewWebSocketConn wraps a WebSocket connection.
// It accepts any WebSocketConn interface implementation, including
// gorilla/websocket connections and custom implementations.
func NewWebSocketConn(ws WebSocketConn) *Conn {
	return &Conn{connType: ConnWebSocket, ws: ws}
}

// ReadPacket reads a single EO packet.
// For TCP: reads the 2-byte EO length prefix, then the packet body.
// For WebSocket: reads one binary message containing only the EO packet body.
// Returns the raw packet bytes (action + family + payload), without the length prefix.
func (c *Conn) ReadPacket() ([]byte, error) {
	switch c.connType {
	case ConnTCP:
		_ = c.tcp.SetReadDeadline(time.Time{}) // no deadline; ping/pong handles liveness

		// Read 2-byte length prefix
		lengthBuf := make([]byte, 2)
		if _, err := io.ReadFull(c.tcp, lengthBuf); err != nil {
			return nil, err
		}

		// Detect TLS ClientHello (0x16 0x03) — client is trying SSL on a plain TCP port
		if lengthBuf[0] == 0x16 && lengthBuf[1] == 0x03 {
			return nil, fmt.Errorf("TLS handshake detected on plain TCP port (client may need WebSocket port)")
		}

		packetLength := data.DecodeNumber(lengthBuf)
		if packetLength < 2 || packetLength > 65535 {
			return nil, fmt.Errorf("invalid packet length: %d", packetLength)
		}
		packetBuf := make([]byte, packetLength)
		if _, err := io.ReadFull(c.tcp, packetBuf); err != nil {
			return nil, err
		}
		return packetBuf, nil

	case ConnWebSocket:
		_ = c.ws.SetReadDeadline(time.Time{})
		msgType, msg, err := c.ws.ReadMessage()
		if err != nil {
			return nil, err
		}
		if msgType != websocket.BinaryMessage {
			return nil, fmt.Errorf("expected binary message, got type %d", msgType)
		}
		// Web clients send the 2-byte EO length prefix inside the WebSocket
		// message. Strip it so the caller gets only action + family + payload,
		// matching the TCP path.
		if len(msg) >= 4 {
			msg = msg[2:]
		}
		if len(msg) < 2 { // at least action + family
			return nil, fmt.Errorf("websocket message too short: %d bytes", len(msg))
		}
		return msg, nil

	default:
		return nil, fmt.Errorf("unknown connection type")
	}
}

// WritePacket writes a fully assembled packet with the 2-byte EO length prefix prepended.
// For TCP: writes the full EO packet as-is.
// For WebSocket: writes the full EO packet including the length prefix, matching
// the web client convention where both send and receive include the prefix.
func (c *Conn) WritePacket(buf []byte) error {
	switch c.connType {
	case ConnTCP:
		_ = c.tcp.SetWriteDeadline(time.Now().Add(writeDeadline))
		_, err := c.tcp.Write(buf)
		return err

	case ConnWebSocket:
		if len(buf) < 4 {
			return fmt.Errorf("websocket packet too short: %d bytes", len(buf))
		}

		c.wsMu.Lock()
		defer c.wsMu.Unlock()
		_ = c.ws.SetWriteDeadline(time.Now().Add(writeDeadline))
		return c.ws.WriteMessage(websocket.BinaryMessage, buf)

	default:
		return fmt.Errorf("unknown connection type")
	}
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	switch c.connType {
	case ConnTCP:
		return c.tcp.Close()
	case ConnWebSocket:
		return c.ws.Close()
	default:
		return nil
	}
}

// RemoteAddr returns the remote address string.
func (c *Conn) RemoteAddr() string {
	switch c.connType {
	case ConnTCP:
		return c.tcp.RemoteAddr().String()
	case ConnWebSocket:
		return c.ws.RemoteAddr().String()
	default:
		return "unknown"
	}
}
