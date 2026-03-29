package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/avdo/goeoserv/internal/config"
	"github.com/avdo/goeoserv/internal/db"
	"github.com/avdo/goeoserv/internal/player"
	"github.com/avdo/goeoserv/internal/protocol"
	"github.com/avdo/goeoserv/internal/world"
	"github.com/ethanmoffat/eolib-go/v3/protocol/net/server"
	"github.com/gorilla/websocket"
)

type Server struct {
	cfg      *config.Config
	db       *db.Database
	world    *world.World
	listener net.Listener
	mux      *protocolMux

	mu           sync.Mutex
	players      map[int]*player.Player
	nextPlayerID int
	ipConnCount  map[string]int
	ipLastConn   map[string]time.Time
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func New(cfg *config.Config, database *db.Database, w *world.World) *Server {
	return &Server{
		cfg:          cfg,
		db:           database,
		world:        w,
		players:      make(map[int]*player.Player),
		nextPlayerID: 1,
		ipConnCount:  make(map[string]int),
		ipLastConn:   make(map[string]time.Time),
	}
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Server.Host, s.cfg.Server.Port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("binding to %s: %w", addr, err)
	}

	s.mux = newProtocolMux(s.listener)

	go s.serveWebSocket(ctx, s.mux.HTTPListener())
	go s.acceptTCPLoop(ctx, s.mux.TCPListener())

	slog.Info("listening (TCP/WebSocket)", "addr", addr)
	return nil
}

func (s *Server) serveWebSocket(ctx context.Context, l net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.allowConnection(ip) {
			http.Error(w, "too many connections", http.StatusTooManyRequests)
			return
		}

		wsConn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Debug("websocket upgrade failed", "ip", ip, "err", err)
			return
		}

		wsConn.SetReadLimit(68000)
		wrappedConn := protocol.NewWebSocketConn(wsConn)
		s.addPlayer(ctx, wrappedConn, ip, "WebSocket")
	})

	srv := &http.Server{Handler: mux}
	_ = srv.Serve(l)
}

func (s *Server) acceptTCPLoop(ctx context.Context, l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("accept error", "err", err)
				continue
			}
		}

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !s.allowConnection(ip) {
			_ = conn.Close()
			continue
		}

		wrappedConn := protocol.NewTCPConn(conn)
		s.addPlayer(ctx, wrappedConn, ip, "TCP")
	}
}

func (s *Server) addPlayer(ctx context.Context, conn *protocol.Conn, ip string, connType string) {
	s.mu.Lock()
	s.ipConnCount[ip]++
	s.ipLastConn[ip] = time.Now()
	playerID := s.nextPlayerID
	s.nextPlayerID++

	p := player.New(playerID, conn, ip, s.cfg, s.db)
	p.World = s.world
	s.players[playerID] = p
	connCount := len(s.players)
	s.mu.Unlock()

	slog.Info("connection accepted",
		"type", connType,
		"addr", conn.RemoteAddr(),
		"id", playerID,
		"connections", fmt.Sprintf("%d/%d", connCount, s.cfg.Server.MaxConnections),
	)

	go func() {
		p.Run(ctx)
		s.removePlayer(playerID, ip)
	}()
}

func (s *Server) allowConnection(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.players) >= s.cfg.Server.MaxConnections {
		slog.Warn("server full, rejecting connection", "ip", ip)
		return false
	}

	if s.cfg.Server.MaxConnectionsPerIP > 0 && s.ipConnCount[ip] >= s.cfg.Server.MaxConnectionsPerIP {
		slog.Warn("too many connections from IP", "ip", ip, "count", s.ipConnCount[ip])
		return false
	}

	if s.cfg.Server.IPReconnectLimit > 0 {
		if lastConn, ok := s.ipLastConn[ip]; ok {
			if time.Since(lastConn) < time.Duration(s.cfg.Server.IPReconnectLimit)*time.Second {
				slog.Warn("reconnecting too quickly", "ip", ip)
				return false
			}
		}
	}

	return true
}

func (s *Server) removePlayer(playerID int, ip string) {
	s.mu.Lock()
	p := s.players[playerID]
	delete(s.players, playerID)
	s.ipConnCount[ip]--
	if s.ipConnCount[ip] <= 0 {
		delete(s.ipConnCount, ip)
	}
	s.mu.Unlock()

	if p != nil {
		if err := p.SaveCharacter(); err != nil {
			slog.Error("failed to save character on disconnect", "id", playerID, "err", err)
		}
		if p.MapID > 0 {
			s.world.LeaveMap(p.MapID, playerID)
		}
		if p.AccountID > 0 {
			s.world.RemoveLoggedInAccount(p.AccountID)
		}
	}

	slog.Info("connection closed",
		"id", playerID,
		"connections", fmt.Sprintf("%d/%d", len(s.players), s.cfg.Server.MaxConnections),
	)
}

// RunPingLoop sends periodic pings and disconnects unresponsive players.
func (s *Server) RunPingLoop(ctx context.Context) {
	if s.cfg.Server.PingRate <= 0 {
		return
	}
	pingInterval := time.Duration(s.cfg.Server.PingRate) * time.Second
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			type pingTarget struct {
				player *player.Player
				id     int
			}
			targets := make([]pingTarget, 0, len(s.players))
			for id, p := range s.players {
				targets = append(targets, pingTarget{player: p, id: id})
			}
			s.mu.Unlock()

			for _, t := range targets {
				now := time.Now()
				seq1, seq2, seqStart := protocol.GeneratePingSequenceBytes()
				switch t.player.Bus.StartPing(now, pingInterval, seqStart) {
				case protocol.PingTimedOut:
					slog.Info("player timed out (no pong)", "id", t.id)
					t.player.Close()
					continue
				case protocol.PingAwaitingPong:
					continue
				}

				if err := t.player.Bus.SendPacket(&server.ConnectionPlayerServerPacket{
					Seq1: seq1,
					Seq2: seq2,
				}); err != nil {
					slog.Debug("player ping send failed", "id", t.id, "err", err)
					t.player.Close()
				}
			}
		}
	}
}

// RunSaveLoop periodically saves all online characters.
func (s *Server) RunSaveLoop(ctx context.Context) {
	if s.cfg.Server.SaveRate <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(s.cfg.Server.SaveRate) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			players := make([]*player.Player, 0, len(s.players))
			for _, p := range s.players {
				players = append(players, p)
			}
			s.mu.Unlock()

			if len(players) == 0 {
				continue
			}

			saved := 0
			for _, p := range players {
				if err := p.SaveCharacter(); err != nil {
					slog.Error("auto-save failed", "id", p.ID, "err", err)
				} else if p.CharacterID != nil {
					saved++
				}
			}
			if saved > 0 {
				slog.Info("auto-save complete", "saved", saved)
			}
		}
	}
}

func (s *Server) Shutdown() {
	if s.mux != nil {
		_ = s.mux.Close()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.players {
		p.Close()
	}
}
