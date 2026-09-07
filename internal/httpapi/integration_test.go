//go:build integration

package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 集成层：通过真实 TCP 监听与 http.Client 走完整请求链路（中间件、编码、状态码）。
// 引入数据库等外部依赖后，本层测试连接真实依赖，运行方式为 make test-integration。
func TestIntegration_HealthzAndPing(t *testing.T) {
	srv := httptest.NewServer(newTestHandler())
	t.Cleanup(srv.Close)

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantKey    string
		wantValue  string
	}{
		{name: "healthz", path: "/healthz", wantStatus: http.StatusOK, wantKey: "status", wantValue: "ok"},
		{name: "ping", path: "/v1/ping", wantStatus: http.StatusOK, wantKey: "message", wantValue: "pong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.Client().Get(srv.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body[tc.wantKey] != tc.wantValue {
				t.Errorf("body[%q] = %q, want %q", tc.wantKey, body[tc.wantKey], tc.wantValue)
			}
		})
	}
}

func TestIntegration_Notes(t *testing.T) {
	srv := httptest.NewServer(newTestHandler())
	t.Cleanup(srv.Close)
	client := srv.Client()
	notesURL := srv.URL + "/v1/notes"

	status, raw := doJSON(t, client, http.MethodPost, notesURL, `{"title":"买菜","content":"鸡蛋、牛奶"}`)
	if status != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d (body %s)", status, http.StatusCreated, raw)
	}
	created := decodeNote(t, raw)
	if !noteIDPattern.MatchString(created.ID) {
		t.Fatalf("id = %q, want match %s", created.ID, noteIDPattern)
	}

	status, raw = doJSON(t, client, http.MethodGet, notesURL+"/"+created.ID, "")
	if status != http.StatusOK {
		t.Fatalf("GET status = %d, want %d (body %s)", status, http.StatusOK, raw)
	}
	if got := decodeNote(t, raw); got != created {
		t.Errorf("GET = %+v, want %+v", got, created)
	}

	status, raw = doJSON(t, client, http.MethodGet, notesURL+"/n_000000000000", "")
	if e := assertErrorEnvelope(t, status, raw, http.StatusNotFound, "not_found"); e.Message != "note not found" {
		t.Errorf("error.message = %q, want %q", e.Message, "note not found")
	}

	status, raw = doJSON(t, client, http.MethodPost, notesURL, `{"title":"   "}`)
	if e := assertErrorEnvelope(t, status, raw, http.StatusBadRequest, "invalid_argument"); !strings.Contains(e.Message, "title") {
		t.Errorf("error.message = %q, want it to mention title", e.Message)
	}
}

func doJSON(t *testing.T, client *http.Client, method, url, body string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	return resp.StatusCode, raw
}
