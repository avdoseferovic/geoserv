package admin

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
	pubdata "github.com/avdoseferovic/geoserv/internal/pub"
)

//go:embed static/* templates/*
var staticFS embed.FS

type PlayerCounter interface {
	OnlinePlayerCount() int
}

type Server struct {
	cfg       *config.Config
	world     PlayerCounter
	startTime time.Time
	srv       *http.Server
	tmpl      *template.Template
}

func New(cfg *config.Config, world PlayerCounter) *Server {
	return &Server{
		cfg:       cfg,
		world:     world,
		startTime: time.Now(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	tmpl, err := template.ParseFS(staticFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}
	s.tmpl = tmpl

	mux := http.NewServeMux()

	// UI routes
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /ui/dashboard", s.handleDashboard)
	mux.HandleFunc("GET /ui/drops", s.handleUIDrops)
	mux.HandleFunc("GET /ui/drops/{id}/edit", s.handleUIDropsEdit)
	mux.HandleFunc("POST /ui/drops/{id}", s.handleUIDropsPost)
	mux.HandleFunc("DELETE /ui/drops/{id}", s.handleUIDropsDelete)

	mux.HandleFunc("GET /ui/talk", s.handleUITalk)
	mux.HandleFunc("GET /ui/talk/{id}/edit", s.handleUITalkEdit)
	mux.HandleFunc("POST /ui/talk/{id}", s.handleUITalkPost)
	mux.HandleFunc("DELETE /ui/talk/{id}", s.handleUITalkDelete)

	mux.HandleFunc("GET /ui/inns", s.handleUIInns)
	mux.HandleFunc("GET /ui/inns/{id}/edit", s.handleUIInnsEdit)
	mux.HandleFunc("POST /ui/inns/{id}", s.handleUIInnsPost)
	mux.HandleFunc("DELETE /ui/inns/{id}", s.handleUIInnsDelete)

	mux.HandleFunc("GET /ui/shops", s.handleUIShops)
	mux.HandleFunc("GET /ui/shops/{id}/edit", s.handleUIShopsEdit)
	mux.HandleFunc("POST /ui/shops/{id}", s.handleUIShopsPost)
	mux.HandleFunc("DELETE /ui/shops/{id}", s.handleUIShopsDelete)

	mux.HandleFunc("GET /ui/masters", s.handleUIMasters)
	mux.HandleFunc("GET /ui/masters/{id}/edit", s.handleUIMastersEdit)
	mux.HandleFunc("POST /ui/masters/{id}", s.handleUIMastersPost)
	mux.HandleFunc("DELETE /ui/masters/{id}", s.handleUIMastersDelete)

	// API lookup routes
	mux.HandleFunc("GET /api/items", handleGetItemList)
	mux.HandleFunc("GET /api/npcs", handleGetNpcList)
	mux.HandleFunc("GET /api/vendors", handleGetVendorList)

	// Static files
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("embedding static files: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	addr := fmt.Sprintf("0.0.0.0:%s", s.cfg.Admin.Port)
	s.srv = &http.Server{Addr: addr, Handler: mux}

	go func() {
		slog.Info("admin panel listening", "addr", addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin panel error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
	}()

	return nil
}

type statsResponse struct {
	Uptime        string `json:"uptime"`
	OnlinePlayers int    `json:"online_players"`
	Items         int    `json:"items"`
	Npcs          int    `json:"npcs"`
	Spells        int    `json:"spells"`
	Classes       int    `json:"classes"`
	DropTables    int    `json:"drop_tables"`
	TalkTables    int    `json:"talk_tables"`
	Inns          int    `json:"inns"`
	Shops         int    `json:"shops"`
	SkillMasters  int    `json:"skill_masters"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()
	if err := s.tmpl.ExecuteTemplate(w, "base", stats); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()
	if err := s.tmpl.ExecuteTemplate(w, "content", stats); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) getStats() statsResponse {
	stats := statsResponse{
		Uptime:        time.Since(s.startTime).Truncate(time.Second).String(),
		OnlinePlayers: s.world.OnlinePlayerCount(),
		Items:         pubdata.EifLength(),
		Npcs:          pubdata.EnfLength(),
		Spells:        pubdata.EsfLength(),
		Classes:       pubdata.EcfLength(),
	}
	if pubdata.DropDB != nil {
		stats.DropTables = len(pubdata.DropDB.Npcs)
	}
	if pubdata.TalkDB != nil {
		stats.TalkTables = len(pubdata.TalkDB.Npcs)
	}
	if pubdata.InnDB != nil {
		stats.Inns = len(pubdata.InnDB.Inns)
	}
	if pubdata.ShopFileDB != nil {
		stats.Shops = len(pubdata.ShopFileDB.Shops)
	}
	if pubdata.SkillMasterDB != nil {
		stats.SkillMasters = len(pubdata.SkillMasterDB.SkillMasters)
	}
	return stats
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode error", "err", err)
	}
}
