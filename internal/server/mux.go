package server

import (
	"bufio"
	"net"
	"sync"
)

// protocolMux splits a single net.Listener into two: one for HTTP connections
// (WebSocket upgrades) and one for everything else (raw EO protocol).
// It peeks the first byte of each connection to decide the route.
type protocolMux struct {
	root net.Listener
	http *chanListener
	tcp  *chanListener
	once sync.Once
}

func newProtocolMux(l net.Listener) *protocolMux {
	m := &protocolMux{
		root: l,
		http: newChanListener(l.Addr()),
		tcp:  newChanListener(l.Addr()),
	}
	go m.serve()
	return m
}

func (m *protocolMux) serve() {
	for {
		conn, err := m.root.Accept()
		if err != nil {
			m.http.closeWithErr(err)
			m.tcp.closeWithErr(err)
			return
		}

		br := bufio.NewReader(conn)
		b, err := br.Peek(1)
		if err != nil {
			_ = conn.Close()
			continue
		}

		pc := &peekedConn{Conn: conn, reader: br}

		// HTTP methods start with an uppercase ASCII letter (G for GET, P for POST/PUT, etc.).
		// EO protocol first byte is an encoded length which is never a valid ASCII letter.
		if b[0] >= 'A' && b[0] <= 'Z' {
			m.http.ch <- pc
		} else {
			m.tcp.ch <- pc
		}
	}
}

// HTTPListener returns the listener that receives HTTP connections.
func (m *protocolMux) HTTPListener() net.Listener { return m.http }

// TCPListener returns the listener that receives raw EO protocol connections.
func (m *protocolMux) TCPListener() net.Listener { return m.tcp }

func (m *protocolMux) Close() error {
	return m.root.Close()
}

// peekedConn wraps a net.Conn with a buffered reader that preserves peeked bytes.
type peekedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *peekedConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

// chanListener is a net.Listener backed by a channel of connections.
type chanListener struct {
	ch     chan net.Conn
	addr   net.Addr
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	closed bool
	err    error
}

func newChanListener(addr net.Addr) *chanListener {
	return &chanListener{
		ch:   make(chan net.Conn, 16),
		addr: addr,
		done: make(chan struct{}),
	}
}

func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ch:
		return conn, nil
	case <-l.done:
		l.mu.Lock()
		err := l.err
		l.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, net.ErrClosed
	}
}

func (l *chanListener) Addr() net.Addr { return l.addr }

func (l *chanListener) Close() error {
	l.closeWithErr(nil)
	return nil
}

func (l *chanListener) closeWithErr(err error) {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.err = err
		l.mu.Unlock()
		close(l.done)
	})
}
