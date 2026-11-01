// Package web serves the local, single-user review application.
package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yhlooo/motif/pkg/review"
	"github.com/yhlooo/motif/pkg/store"
)

//go:embed assets/*
var assets embed.FS

type server struct {
	service     *review.Service
	token, host string
	mux         *http.ServeMux
}

func Handler(service *review.Service, host string) http.Handler {
	s := &server{service: service, host: host, token: store.ID(), mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/bootstrap", s.bootstrap)
	s.mux.HandleFunc("GET /api/code-reviews", s.list)
	s.mux.HandleFunc("POST /api/code-reviews/preview", s.preview)
	s.mux.HandleFunc("POST /api/code-reviews", s.create)
	s.mux.HandleFunc("GET /api/code-reviews/{id}", s.get)
	s.mux.HandleFunc("GET /api/code-reviews/{id}/diff", s.diff)
	s.mux.HandleFunc("PATCH /api/code-reviews/{id}/items/{item}/decision", s.decision)
	s.mux.HandleFunc("GET /api/code-reviews/{id}/threads", s.threads)
	s.mux.HandleFunc("POST /api/code-reviews/{id}/threads", s.thread)
	s.mux.HandleFunc("POST /api/comment-threads/{id}/replies", s.reply)
	s.mux.HandleFunc("GET /api/patches/{id}", s.patch)
	sub, _ := fs.Sub(assets, "assets")
	s.mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != s.host {
			http.Error(w, "invalid local host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+s.host {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Motif-Token")), []byte(s.token)) != 1 {
				http.Error(w, "invalid local request token", http.StatusForbidden)
				return
			}
		}
		s.mux.ServeHTTP(w, r)
	})
}
func Serve(ctx context.Context, service *review.Service, address string, out io.Writer) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("web must bind to a loopback IP address")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	hostport := listener.Addr().String()
	srv := &http.Server{Handler: Handler(service, hostport), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	fmt.Fprintf(out, "Motif review: http://%s\n", hostport)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(c)
		case <-done:
		}
	}()
	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func send(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, err)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		fail(w, fmt.Errorf("expected a single JSON object"))
		return false
	}
	return true
}
func (s *server) rid() string { return s.service.Worktree.RepositoryID }
func (s *server) bootstrap(w http.ResponseWriter, r *http.Request) {
	branches, err := s.service.Repo.Branches(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	send(w, map[string]any{"token": s.token, "repository": s.service.Repo.Root, "branches": branches})
}
func (s *server) list(w http.ResponseWriter, r *http.Request) {
	rs, err := s.service.Repo.Store.Reviews(s.rid())
	if err != nil {
		fail(w, err)
		return
	}
	send(w, rs)
}
func (s *server) preview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if !decode(w, r, &in) {
		return
	}
	p, err := s.service.Preview(r.Context(), in.Source, in.Target)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, p)
}
func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title   string         `json:"title"`
		Preview review.Preview `json:"preview"`
	}
	if !decode(w, r, &in) {
		return
	}
	cr, err := s.service.Create(r.Context(), in.Preview, in.Title)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, cr)
}
func (s *server) get(w http.ResponseWriter, r *http.Request) {
	cr, err := s.service.Repo.Store.Review(s.rid(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	send(w, cr)
}
func (s *server) diff(w http.ResponseWriter, r *http.Request) {
	cr, err := s.service.Repo.Store.Review(s.rid(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	b, a, err := review.Selection(cr, r.URL.Query().Get("first"), r.URL.Query().Get("last"))
	if err != nil {
		fail(w, err)
		return
	}
	d, err := s.service.Repo.Diff(r.Context(), b, a)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, d)
}
func (s *server) decision(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Decision string `json:"decision"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.service.Repo.Store.Decision(r.Context(), s.rid(), r.PathValue("id"), r.PathValue("item"), in.Decision); err != nil {
		fail(w, err)
		return
	}
	s.get(w, r)
}
func (s *server) threads(w http.ResponseWriter, r *http.Request) {
	ts, err := s.service.Repo.Store.Threads(s.rid(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	send(w, ts)
}
func (s *server) author(r *http.Request) string {
	a := s.service.Repo.Config(r.Context(), "user.name")
	if a == "" {
		a = "本地用户"
	}
	return a
}
func (s *server) thread(w http.ResponseWriter, r *http.Request) {
	var in struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Path  string `json:"path"`
		Side  string `json:"side"`
		Start int    `json:"start"`
		End   int    `json:"end"`
		Body  string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	cr, err := s.service.Repo.Store.Review(s.rid(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	t := store.Thread{First: in.First, Last: in.Last, Path: in.Path, Side: in.Side, Start: in.Start, End: in.End}
	if err = s.service.AddThread(r.Context(), cr, t, in.Body, s.author(r)); err != nil {
		fail(w, err)
		return
	}
	s.threads(w, r)
}
func (s *server) reply(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Body) == "" {
		fail(w, fmt.Errorf("reply body is required"))
		return
	}
	if err := s.service.Repo.Store.Reply(r.Context(), s.rid(), r.PathValue("id"), in.Body, s.author(r)); err != nil {
		fail(w, err)
		return
	}
	send(w, map[string]bool{"ok": true})
}
func (s *server) patch(w http.ResponseWriter, r *http.Request) {
	events, err := s.service.Repo.Store.RepositoryEvents(s.rid())
	if err != nil {
		fail(w, err)
		return
	}
	for _, e := range events {
		if e.ID != r.PathValue("id") {
			continue
		}
		b, err := s.service.Repo.Store.Snapshot(e.BeforeID)
		if err != nil {
			fail(w, err)
			return
		}
		a, err := s.service.Repo.Store.Snapshot(e.AfterID)
		if err != nil {
			fail(w, err)
			return
		}
		d, err := s.service.Repo.Diff(r.Context(), b.State, a.State)
		if err != nil {
			fail(w, err)
			return
		}
		send(w, map[string]any{"event": e, "diff": d})
		return
	}
	fail(w, store.ErrNotFound)
}
