package web_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/yhlooo/motif/pkg/gitrepo"
	"github.com/yhlooo/motif/pkg/review"
	"github.com/yhlooo/motif/pkg/store"
	"github.com/yhlooo/motif/pkg/web"
)

func TestReviewAPICommentsAndLocalRequestProtection(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%s %v", b, e)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Reviewer")
	git("config", "user.email", "r@example.com")
	path := filepath.Join(root, "a.go")
	if e := os.WriteFile(path, []byte("old\n"), 0644); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "base")
	git("switch", "-c", "feature")
	if e := os.WriteFile(path, []byte("new\nsecond\n"), 0644); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "change")
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	w, e := r.Identity(ctx)
	if e != nil {
		t.Fatal(e)
	}
	service := &review.Service{Repo: r, Worktree: w}
	handler := web.Handler(service, "127.0.0.1:3939")
	token := ""
	request := func(method, path string, body any, requestToken, origin, host string) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		if body != nil {
			if e := json.NewEncoder(&b).Encode(body); e != nil {
				t.Fatal(e)
			}
		}
		req := httptest.NewRequest(method, "http://"+host+path, &b)
		req.Header.Set("X-Motif-Token", requestToken)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		return request(method, path, body, token, "", "127.0.0.1:3939")
	}
	rec := call("GET", "/api/bootstrap", nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var boot struct {
		Token string `json:"token"`
	}
	if e = json.Unmarshal(rec.Body.Bytes(), &boot); e != nil {
		t.Fatal(e)
	}
	token = boot.Token
	for _, rec := range []*httptest.ResponseRecorder{request("POST", "/api/code-reviews", nil, "", "", "127.0.0.1:3939"), request("POST", "/api/code-reviews", nil, token, "https://evil.example", "127.0.0.1:3939"), request("GET", "/api/bootstrap", nil, "", "", "evil.example:3939")} {
		if rec.Code != http.StatusForbidden {
			t.Fatalf("unsafe request accepted: %d %s", rec.Code, rec.Body.String())
		}
	}
	rec = call("POST", "/api/code-reviews/preview", map[string]string{"source": "feature", "target": "main"})
	var p review.Preview
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &p) != nil {
		t.Fatal(rec.Body.String())
	}
	rec = call("POST", "/api/code-reviews", map[string]any{"title": "API Review", "preview": p})
	var cr store.Review
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &cr) != nil {
		t.Fatal(rec.Body.String())
	}
	item := cr.Items[0].ID
	prefix := "/api/code-reviews/" + cr.ID
	rec = call("GET", prefix+"/diff?first="+item+"&last="+item, nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("+new")) {
		t.Fatal("diff", rec.Body.String())
	}
	input := map[string]any{"first": item, "last": item, "path": base64.StdEncoding.EncodeToString([]byte("a.go")), "side": "new", "start": 1, "end": 2, "body": "<script>alert(1)</script>", "view_version": cr.Version}
	rec = call("POST", prefix+"/threads", input)
	var threads []store.Thread
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &threads) != nil || len(threads) != 1 {
		t.Fatal("thread", rec.Body.String())
	}
	if threads[0].Comments[0].Author != "Reviewer" {
		t.Fatal("author metadata")
	}
	rec = call("POST", "/api/comment-threads/"+threads[0].ID+"/replies", map[string]any{"body": "reply", "view_version": cr.Version})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	input["end"] = 100
	rec = call("POST", prefix+"/threads", input)
	if rec.Code == 200 {
		t.Fatal("out-of-range comment was accepted")
	}
	input["end"] = 2
	input["side"] = "wrong"
	rec = call("POST", prefix+"/threads", input)
	if rec.Code == 200 {
		t.Fatal("invalid side was accepted")
	}
	rec = call("PATCH", prefix+"/items/"+item+"/decision", map[string]any{"decision": "approved", "view_version": cr.Version})
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"status":"approved"`)) {
		t.Fatal("approve", rec.Body.String())
	}
	// Reads keep the published range until an explicit refresh publishes a new version.
	if e = os.WriteFile(path, []byte("new\nsecond\nthird\n"), 0644); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "advance source")
	rec = call("GET", prefix, nil)
	var published store.Review
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &published) != nil || published.Version != cr.Version {
		t.Fatal("GET unexpectedly refreshed", rec.Body.String())
	}
	rec = call("POST", prefix+"/refresh", nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = call("PATCH", prefix+"/items/"+item+"/decision", map[string]any{"decision": "approved", "view_version": cr.Version})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale approval accepted: %d %s", rec.Code, rec.Body.String())
	}
	input["side"] = "new"
	input["end"] = 2
	rec = call("POST", prefix+"/threads", input)
	if rec.Code != http.StatusConflict {
		t.Fatal("stale comment accepted", rec.Body.String())
	}
	rec = call("GET", prefix, nil)
	var refreshed store.Review
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &refreshed) != nil || refreshed.Version <= cr.Version || refreshed.Status != "in_review" || len(refreshed.Commits) != 2 {
		t.Fatal("source not refreshed", rec.Body.String())
	}
	rec = call("GET", prefix+"/diff?first="+item+"&last="+item+"&view_version="+strconv.FormatInt(cr.Version, 10), nil)
	if rec.Code != http.StatusConflict {
		t.Fatal("stale diff read mixed view versions", rec.Body.String())
	}
	rec = call("GET", "/", nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("Motif")) {
		t.Fatal("embedded UI missing")
	}

	// Deleting both tracking branches only prevents updates, never reading or
	// reviewing the last published commit range.
	git("switch", "--detach")
	git("branch", "-D", "feature", "main")
	rec = call("POST", prefix+"/refresh", nil)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"updated":false`)) {
		t.Fatal("missing branch", rec.Body.String())
	}
	for _, path := range []string{"/api/code-reviews", prefix, prefix + "/diff?first=" + refreshed.Items[0].ID + "&last=" + refreshed.Items[0].ID, prefix + "/threads"} {
		rec = call("GET", path, nil)
		if rec.Code != 200 {
			t.Fatal("saved CR unavailable", path, rec.Body.String())
		}
	}
	rec = call("PATCH", prefix+"/decisions", map[string]any{"items": []string{refreshed.Items[0].ID}, "decision": "approved", "view_version": refreshed.Version})
	if rec.Code != 200 {
		t.Fatal("cannot review saved range", rec.Body.String())
	}
	rec = call("POST", prefix+"/threads", map[string]any{"first": refreshed.Items[0].ID, "last": refreshed.Items[0].ID, "path": base64.StdEncoding.EncodeToString([]byte("a.go")), "side": "new", "start": 1, "end": 1, "body": "saved range comment", "view_version": refreshed.Version})
	if rec.Code != 200 {
		t.Fatal("cannot comment saved range", rec.Body.String())
	}
	rec = call("POST", "/api/comment-threads/"+threads[0].ID+"/replies", map[string]any{"body": "obsolete reply", "view_version": cr.Version})
	if rec.Code != http.StatusConflict {
		t.Fatal("stale reply accepted", rec.Body.String())
	}
	if e = s.DeleteRepository(ctx, w.RepositoryID); e != nil {
		t.Fatal("project deletion with review/comment relations", e)
	}
}
