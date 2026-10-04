package actelyohub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOptimizerMeasuredRunAndApply(t *testing.T) {
	testHome(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/models" {
			_, _ = w.Write([]byte(`{"models":[{"loaded_instances":[{"id":"test-model","config":{"context_length":16384,"parallel":1}}]}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		var body struct {
			Temperature float64 `json:"temperature"`
			Messages    []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests++
		text := body.Messages[len(body.Messages)-1].Content
		answer := "OK"
		if strings.Contains(text, "multiplié") {
			answer = "42"
		} else if strings.Contains(text, "Extrait") {
			answer = `{"client":"ALPHA","montant":120}`
		} else if strings.Contains(text, "date de signature") {
			answer = "INFORMATION ABSENTE"
		}
		if body.Temperature == .3 {
			answer = "incorrect"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}, "finish_reason": "stop"}}, "usage": map[string]int{"completion_tokens": 4}})
	}))
	defer server.Close()
	cfg := map[string]string{"EXTERNAL": "1", "EXTERNAL_URL": server.URL + "/v1", "EXTERNAL_MODEL": "test-model", "CTX": "8192"}
	if err := WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handleOptimizerRun(rr, httptest.NewRequest("POST", "/api/optimizer/run", strings.NewReader(`{"objective":"balanced"}`)))
	if rr.Code != 202 {
		t.Fatal(rr.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for optimizerBusy() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if optimizerBusy() {
		t.Fatal("job did not finish")
	}
	var report optimizerReport
	getJSON(bkState, "optimizer_report", &report)
	if report.State != "completed" || requests != 19 || report.Profiles[2].Passed != 0 || report.Recommended == 2 {
		t.Fatalf("bad comparison %#v requests=%d", report, requests)
	}
	if formatEnv(ReadConfig()) != formatEnv(cfg) {
		t.Fatal("benchmark modified configuration")
	}
	rr = httptest.NewRecorder()
	handleOptimizerApply(rr, httptest.NewRequest("POST", "/api/optimizer/apply", strings.NewReader("{}")))
	if rr.Code != 200 || ReadConfig()["CTX"] != "16384" || ReadConfig()["TEMP"] == "" {
		t.Fatalf("profile not applied %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handleOptimizerRestore(rr, httptest.NewRequest("POST", "/api/optimizer/restore", strings.NewReader("{}")))
	if rr.Code != 200 || formatEnv(ReadConfig()) != formatEnv(cfg) {
		t.Fatalf("round trip failed %s", rr.Body.String())
	}
}

func TestOptimizerLocalOnly(t *testing.T) {
	for _, bad := range []string{"https://api.openai.com/v1", "http://192.168.1.2/v1", "http://127.0.0.1:1234/v1?key=secret", "http://user:secret@localhost:1234/v1", "file:///v1", "http://localhost:1234/foo"} {
		if _, err := optimizerOrigin(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	for _, good := range []string{"http://127.0.0.1:1234/v1", "http://localhost:1234/v1", "http://[::1]:1234/v1/"} {
		if _, err := optimizerOrigin(good); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := optimizerOrigin("http://localhost/v1"); err != nil || got != "http://127.0.0.1" {
		t.Fatalf("%s %v", got, err)
	}
}
func TestOptimizerRejectsFastWrongAnswers(t *testing.T) {
	samples := make([]optimizerSample, 6)
	profiles := []optimizerProfile{{Temperature: .7, Passed: 5, TokensPerSecond: 100, Samples: samples}, {Temperature: .1, Passed: 6, TokensPerSecond: 10, Samples: samples}, {Temperature: .3, Passed: 6, TokensPerSecond: 10.5, Samples: samples}}
	if selectOptimizerProfile(profiles, "balanced") != 1 || selectOptimizerProfile(profiles, "speed") != 2 {
		t.Fatal("quality gate or objective ignored")
	}
	if selectOptimizerProfile(profiles[:1], "balanced") != -1 {
		t.Fatal("failed profile recommended")
	}
}
func TestOptimizerValidators(t *testing.T) {
	for _, tc := range []struct {
		task, answer string
		valid        bool
	}{{"calcul", "42", true}, {"calcul", "42 euros", false}, {"extraction", `{"client":"ALPHA","montant":120}`, true}, {"extraction", `{"client":"ALPHA","montant":"120"}`, false}, {"information_absente", "INFORMATION ABSENTE", true}, {"information_absente", "Le 1er janvier", false}} {
		if optimizerValid(tc.task, tc.answer) != tc.valid {
			t.Fatal(tc)
		}
	}
}
func TestOptimizerNeverFollowsRedirect(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalled = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	var result any
	if optimizerRequest(context.Background(), server.URL, "secret", "/v1/models", nil, &result) == nil || targetCalled {
		t.Fatal("redirect leaked request")
	}
}
func TestOptimizerRestoresAndRejectsChangedConfig(t *testing.T) {
	testHome(t)
	hubOptimizer.Lock()
	hubOptimizer.busy = false
	hubOptimizer.Unlock()
	cfg := map[string]string{"EXTERNAL": "1", "EXTERNAL_URL": "http://127.0.0.1:1234/v1", "EXTERNAL_MODEL": "legalya", "CTX": "8192", "TEMP": ".7"}
	original := map[string]string{"EXTERNAL": "1", "EXTERNAL_URL": "http://127.0.0.1:1234/v1", "EXTERNAL_MODEL": "legalya", "CTX": "4096"}
	if err := WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := putJSON(bkState, "optimizer_backup", optimizerBackup{Config: original, Expected: formatEnv(cfg)}); err != nil {
		t.Fatal(err)
	}
	if err := putJSON(bkState, "optimizer_report", optimizerReport{State: "completed", Applied: true, CanRestore: true}); err != nil {
		t.Fatal(err)
	}
	if err := SetConfigKey("CTX", "16384"); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handleOptimizerRestore(rr, httptest.NewRequest("POST", "/api/optimizer/restore", strings.NewReader("{}")))
	if rr.Code != 409 || ReadConfig()["CTX"] != "16384" {
		t.Fatal("overwrote manual edits")
	}
	if err := WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	handleOptimizerRestore(rr, httptest.NewRequest("POST", "/api/optimizer/restore", strings.NewReader("{}")))
	var report optimizerReport
	_ = json.Unmarshal(rr.Body.Bytes(), &report)
	if rr.Code != 200 || report.State != "restored" || report.CanRestore || formatEnv(ReadConfig()) != formatEnv(original) {
		t.Fatalf("restore failed %s", rr.Body.String())
	}
}
