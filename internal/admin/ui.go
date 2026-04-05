package admin

import (
	"log/slog"
	"net/http"
)

type uiData struct {
	Items    any
	Page     int
	Size     int
	Total    int
	Query    string
	HasPrev  bool
	HasNext  bool
	PrevPage int
	NextPage int
}

func newUIData(items any, page, size, total int, q string) uiData {
	return uiData{
		Items:    items,
		Page:     page,
		Size:     size,
		Total:    total,
		Query:    q,
		HasPrev:  page > 0,
		HasNext:  (page+1)*size < total,
		PrevPage: page - 1,
		NextPage: page + 1,
	}
}

func (s *Server) handleUIDrops(w http.ResponseWriter, r *http.Request) {
	page, size, q := pageParams(r)
	items, total := getDropsData(page, size, q)
	if err := s.tmpl.ExecuteTemplate(w, "drops", newUIData(items, page, size, total, q)); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) handleUITalk(w http.ResponseWriter, r *http.Request) {
	page, size, q := pageParams(r)
	items, total := getTalkData(page, size, q)
	if err := s.tmpl.ExecuteTemplate(w, "talk", newUIData(items, page, size, total, q)); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) handleUIInns(w http.ResponseWriter, r *http.Request) {
	page, size, q := pageParams(r)
	items, total := getInnsData(page, size, q)
	if err := s.tmpl.ExecuteTemplate(w, "inns", newUIData(items, page, size, total, q)); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) handleUIShops(w http.ResponseWriter, r *http.Request) {
	page, size, q := pageParams(r)
	items, total := getShopsData(page, size, q)
	if err := s.tmpl.ExecuteTemplate(w, "shops", newUIData(items, page, size, total, q)); err != nil {
		slog.Error("template execute error", "err", err)
	}
}

func (s *Server) handleUIMasters(w http.ResponseWriter, r *http.Request) {
	page, size, q := pageParams(r)
	items, total := getMastersData(page, size, q)
	if err := s.tmpl.ExecuteTemplate(w, "masters", newUIData(items, page, size, total, q)); err != nil {
		slog.Error("template execute error", "err", err)
	}
}
