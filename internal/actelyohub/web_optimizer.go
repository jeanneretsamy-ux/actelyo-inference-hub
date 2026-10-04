package actelyohub

// Local, measured sampling optimization. No model downloads or reloads: the
// existing LM Studio instance, GPU placement and parallelism stay intact.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type optimizerSample struct {
	Task    string  `json:"task"`
	Answer  string  `json:"answer"`
	Valid   bool    `json:"valid"`
	Seconds float64 `json:"seconds"`
	Tokens  int     `json:"tokens"`
	Error   string  `json:"error,omitempty"`
}
type optimizerProfile struct {
	Name            string            `json:"name"`
	Temperature     float64           `json:"temperature"`
	Passed          int               `json:"passed"`
	TokensPerSecond float64           `json:"tokens_per_second"`
	Samples         []optimizerSample `json:"samples"`
}
type optimizerReport struct {
	ID          string             `json:"id"`
	State       string             `json:"state"`
	Progress    string             `json:"progress"`
	Objective   string             `json:"objective"`
	Model       string             `json:"model"`
	Context     int                `json:"context"`
	Parallel    int                `json:"parallel"`
	FreeRAM     int                `json:"free_ram_mb"`
	Profiles    []optimizerProfile `json:"profiles"`
	Recommended int                `json:"recommended"`
	Applied     bool               `json:"applied"`
	CanRestore  bool               `json:"can_restore"`
	Error       string             `json:"error,omitempty"`
	Warning     string             `json:"warning"`
}
type optimizerBackup struct {
	Config                                        map[string]string
	PresetID, PresetName, PresetContent, Expected string
	ExpectedPresetContent                         string
}

var hubOptimizer = struct {
	sync.Mutex
	busy   bool
	cancel context.CancelFunc
	report optimizerReport
}{report: optimizerReport{State: "idle", Recommended: -1}}

func optimizerOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "/v1" && u.Path != "/v1/") || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("LM Studio doit être une API locale /v1 sans identifiants ni paramètres")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if !(ip != nil && ip.IsLoopback()) && host != "localhost" {
		return "", fmt.Errorf("l'optimisation est réservée à LM Studio sur cet ordinateur")
	}
	// Use a literal address rather than resolving localhost via DNS.
	if host == "localhost" {
		u.Host = net.JoinHostPort("127.0.0.1", u.Port())
		if u.Port() == "" {
			u.Host = "127.0.0.1"
		}
	}
	u.Path = ""
	return strings.TrimRight(u.String(), "/"), nil
}
func optimizerRequest(ctx context.Context, origin, key, path string, body any, dst any) error {
	var data []byte
	method := http.MethodGet
	if body != nil {
		data, _ = json.Marshal(body)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, origin+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirection refusée") }, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("LM Studio ne répond pas : %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LM Studio a refusé le test (HTTP %d)", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(dst)
}
func optimizerInventory(ctx context.Context, cfg map[string]string) (int, int, error) {
	origin, err := optimizerOrigin(cfg["EXTERNAL_URL"])
	if err != nil {
		return 0, 0, err
	}
	var inventory struct {
		Models []struct {
			Loaded []struct {
				ID     string `json:"id"`
				Config struct {
					Context  int `json:"context_length"`
					Parallel int `json:"parallel"`
				} `json:"config"`
			} `json:"loaded_instances"`
		} `json:"models"`
	}
	if err = optimizerRequest(ctx, origin, cfg["EXTERNAL_KEY"], "/api/v1/models", nil, &inventory); err != nil {
		return 0, 0, err
	}
	for _, model := range inventory.Models {
		for _, instance := range model.Loaded {
			if instance.ID == cfg["EXTERNAL_MODEL"] && instance.Config.Context > 0 {
				return instance.Config.Context, instance.Config.Parallel, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("le modèle du hub n'est pas chargé dans LM Studio")
}
func handleOptimizer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	hubOptimizer.Lock()
	defer hubOptimizer.Unlock()
	if !hubOptimizer.busy {
		getJSON(bkState, "optimizer_report", &hubOptimizer.report)
	}
	sendJSON(w, 200, hubOptimizer.report)
}
func optimizerBusy() bool { hubOptimizer.Lock(); defer hubOptimizer.Unlock(); return hubOptimizer.busy }
func handleOptimizerRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var body struct {
		Objective string `json:"objective"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body) != nil || (body.Objective != "balanced" && body.Objective != "speed") {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "objectif requis : balanced ou speed"})
		return
	}
	hubOptimizer.Lock()
	defer hubOptimizer.Unlock()
	if hubOptimizer.busy || conv.isGenerating() {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "attendez la fin de la génération ou du test en cours"})
		return
	}
	var previous optimizerReport
	getJSON(bkState, "optimizer_report", &previous)
	if previous.CanRestore {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "restaurez les réglages précédents avant une nouvelle comparaison"})
		return
	}
	cfg := ReadConfig()
	if cfg["EXTERNAL"] != "1" || cfg["EXTERNAL_MODEL"] == "" {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "sélectionnez un preset LM Studio local"})
		return
	}
	if _, err := optimizerOrigin(cfg["EXTERNAL_URL"]); err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	hubOptimizer.busy = true
	hubOptimizer.cancel = cancel
	hubOptimizer.report = optimizerReport{ID: strconv.FormatInt(time.Now().UnixNano(), 10), State: "running", Progress: "Vérification du modèle local…", Objective: body.Objective, Model: cfg["EXTERNAL_MODEL"], Recommended: -1, Profiles: []optimizerProfile{}, Warning: "Tests fictifs de calcul, extraction et absence d'information, répétés deux fois. La qualité juridique, le GPU et le débit en concurrence ne sont pas évalués."}
	backup := optimizerBackup{Config: cfg, PresetID: activePresetID(), PresetName: activePresetName(), Expected: formatEnv(cfg)}
	if backup.PresetID != "" {
		backup.PresetContent, _ = ReadPreset(backup.PresetID)
		backup.ExpectedPresetContent = backup.PresetContent
	}
	if err := putJSON(bkState, "optimizer_backup", backup); err != nil {
		cancel()
		hubOptimizer.busy = false
		sendJSON(w, 500, map[string]any{"ok": false, "error": "sauvegarde impossible"})
		return
	}
	report := hubOptimizer.report
	go runOptimizer(ctx, cancel, cfg, report)
	sendJSON(w, 202, report)
}
func handleOptimizerCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	hubOptimizer.Lock()
	defer hubOptimizer.Unlock()
	if hubOptimizer.cancel != nil {
		hubOptimizer.cancel()
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}
func selectOptimizerProfile(profiles []optimizerProfile, objective string) int {
	best := -1
	for i, p := range profiles {
		if p.Passed != 6 || len(p.Samples) != 6 || p.TokensPerSecond <= 0 {
			continue
		}
		if best == -1 {
			best = i
			continue
		}
		b := profiles[best]
		if objective == "speed" {
			if p.TokensPerSecond > b.TokensPerSecond {
				best = i
			}
		} else if p.TokensPerSecond > b.TokensPerSecond*1.1 || (p.TokensPerSecond >= b.TokensPerSecond*.9 && p.Temperature < b.Temperature) {
			best = i
		}
	}
	return best
}
func optimizerValid(task, answer string) bool {
	s := strings.TrimSpace(answer)
	switch task {
	case "calcul":
		return s == "42"
	case "extraction":
		var doc struct {
			Client  string `json:"client"`
			Montant int    `json:"montant"`
		}
		return json.Unmarshal([]byte(s), &doc) == nil && doc.Client == "ALPHA" && doc.Montant == 120
	case "information_absente":
		return strings.ToUpper(s) == "INFORMATION ABSENTE"
	}
	return false
}
func runOptimizer(ctx context.Context, cancel context.CancelFunc, cfg map[string]string, report optimizerReport) {
	defer cancel()
	defer func() {
		hubOptimizer.Lock()
		defer hubOptimizer.Unlock()
		hubOptimizer.busy = false
		hubOptimizer.cancel = nil
		hubOptimizer.report = report
		_ = putJSON(bkState, "optimizer_report", report)
	}()
	contextLength, parallel, err := optimizerInventory(ctx, cfg)
	if err != nil {
		report.State = "failed"
		report.Error = err.Error()
		return
	}
	report.Context = contextLength
	report.Parallel = parallel
	used, total := ramUsageMB()
	report.FreeRAM = total - used
	if report.FreeRAM < 1024 {
		report.Warning += " Mémoire libre inférieure à 1 Go : fermez les applications inutiles avant des documents longs."
	}
	origin, _ := optimizerOrigin(cfg["EXTERNAL_URL"])
	baseline := .7
	if v, err := strconv.ParseFloat(cfg["TEMP"], 64); err == nil {
		baseline = v
	}
	profiles := []optimizerProfile{{Name: "Actuel", Temperature: baseline}, {Name: "Précis", Temperature: .1}, {Name: "Souple", Temperature: .3}}
	tasks := []struct{ Name, Prompt string }{{"calcul", "Combien font 6 multiplié par 7 ? Réponds uniquement avec le nombre."}, {"extraction", "Extrait ce texte fictif en JSON uniquement avec les clés client et montant : Client ALPHA. Montant 120 euros. Le montant doit être un nombre."}, {"information_absente", "Document fictif : le contrat est signé par ALPHA. Quelle est la date de signature ? Si elle ne figure pas dans le document, réponds uniquement INFORMATION ABSENTE."}}
	// Warm up once, excluded from timing; never autoload an absent instance.
	var warm any
	if err := optimizerRequest(ctx, origin, cfg["EXTERNAL_KEY"], "/v1/chat/completions", map[string]any{"model": report.Model, "messages": []map[string]string{{"role": "user", "content": "Réponds OK"}}, "max_tokens": 4, "temperature": .1}, &warm); err != nil {
		report.State = "failed"
		report.Error = err.Error()
		return
	}
	for round := 0; round < 2; round++ {
		for i := range profiles {
			for _, task := range tasks {
				if ctx.Err() != nil {
					report.State = "cancelled"
					report.Error = "Tests interrompus. La configuration n'a pas été modifiée."
					return
				}
				report.Progress = fmt.Sprintf("%s — %s — passage %d/2", profiles[i].Name, task.Name, round+1)
				// Publish an independent copy: polling must not race with profile updates.
				encoded, _ := json.Marshal(report)
				var snapshot optimizerReport
				_ = json.Unmarshal(encoded, &snapshot)
				hubOptimizer.Lock()
				hubOptimizer.report = snapshot
				hubOptimizer.Unlock()
				payload := map[string]any{"model": report.Model, "messages": []map[string]string{{"role": "system", "content": readSysPrompt()}, {"role": "user", "content": task.Prompt}}, "max_tokens": 96, "stream": false, "temperature": profiles[i].Temperature}
				applySamplingFrom(payload, cfg)
				payload["temperature"] = profiles[i].Temperature
				var response struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
						Finish string `json:"finish_reason"`
					} `json:"choices"`
					Usage struct {
						Completion int `json:"completion_tokens"`
					} `json:"usage"`
				}
				start := time.Now()
				err := optimizerRequest(ctx, origin, cfg["EXTERNAL_KEY"], "/v1/chat/completions", payload, &response)
				sample := optimizerSample{Task: task.Name, Seconds: time.Since(start).Seconds(), Tokens: response.Usage.Completion}
				if err != nil {
					sample.Error = err.Error()
				} else if len(response.Choices) > 0 {
					sample.Answer = response.Choices[0].Message.Content
					sample.Valid = response.Choices[0].Finish != "length" && optimizerValid(task.Name, sample.Answer)
				}
				if sample.Valid {
					profiles[i].Passed++
				}
				profiles[i].Samples = append(profiles[i].Samples, sample)
				var seconds float64
				var tokens int
				for _, s := range profiles[i].Samples {
					seconds += s.Seconds
					tokens += s.Tokens
				}
				if seconds > 0 {
					profiles[i].TokensPerSecond = float64(tokens) / seconds
				}
				report.Profiles = profiles
			}
		}
	}
	if formatEnv(ReadConfig()) != formatEnv(cfg) {
		report.State = "failed"
		report.Error = "La configuration a changé pendant les tests. Relancez la comparaison."
		return
	}
	if length, concurrency, err := optimizerInventory(ctx, cfg); err != nil || length != report.Context || concurrency != report.Parallel {
		report.State = "failed"
		report.Error = "Le chargement LM Studio a changé pendant les tests. Relancez la comparaison."
		return
	}
	report.Recommended = selectOptimizerProfile(profiles, report.Objective)
	if report.Recommended < 0 {
		report.State = "failed"
		report.Error = "Aucun profil n'a réussi les six vérifications. Réglages conservés."
		return
	}
	report.State = "completed"
	report.Progress = "Comparaison terminée. Réglages d'origine conservés jusqu'à l'application."
}
func handleOptimizerApply(w http.ResponseWriter, r *http.Request)   { optimizerConfigure(w, r, false) }
func handleOptimizerRestore(w http.ResponseWriter, r *http.Request) { optimizerConfigure(w, r, true) }
func optimizerConfigure(w http.ResponseWriter, r *http.Request, restore bool) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	hubOptimizer.Lock()
	defer hubOptimizer.Unlock()
	fail := func(message string) { sendJSON(w, 409, map[string]any{"ok": false, "error": message}) }
	if hubOptimizer.busy || conv.isGenerating() {
		fail("attendez la fin du test ou de la génération")
		return
	}
	var report optimizerReport
	var backup optimizerBackup
	if !getJSON(bkState, "optimizer_report", &report) || !getJSON(bkState, "optimizer_backup", &backup) {
		fail("aucune comparaison enregistrée")
		return
	}
	if formatEnv(ReadConfig()) != backup.Expected {
		fail("les réglages ont été modifiés depuis la comparaison ; aucune modification automatique")
		return
	}
	if restore && !report.CanRestore {
		fail("aucun réglage à restaurer")
		return
	}
	if backup.PresetID != "" {
		content, err := ReadPreset(backup.PresetID)
		if err != nil || activePresetID() != backup.PresetID || content != backup.ExpectedPresetContent {
			fail("le preset a été modifié depuis la comparaison ; aucune modification automatique")
			return
		}
	}
	if !restore && (report.State != "completed" || report.Recommended < 0 || report.Recommended >= len(report.Profiles) || report.Applied) {
		fail("aucun profil mesuré à appliquer")
		return
	}
	if !restore {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		length, _, err := optimizerInventory(ctx, ReadConfig())
		if err != nil || length != report.Context {
			fail("le chargement LM Studio a changé ; relancez les tests")
			return
		}
	}
	next := parseEnv(backup.Expected)
	if restore {
		next = backup.Config
	} else {
		next["TEMP"] = strconv.FormatFloat(report.Profiles[report.Recommended].Temperature, 'f', -1, 64)
		next["CTX"] = strconv.Itoa(report.Context)
	}
	content := formatEnv(next)
	if restore {
		content = backup.PresetContent
	}
	if backup.PresetID != "" {
		if _, err := SavePreset(backup.PresetID, backup.PresetName, content); err != nil {
			fail("enregistrement du preset impossible")
			return
		}
	}
	if err := WriteConfig(next); err != nil {
		if backup.PresetID != "" {
			_, _ = SavePreset(backup.PresetID, backup.PresetName, backup.PresetContent)
		}
		fail("enregistrement de la configuration impossible")
		return
	}
	if backup.PresetID != "" {
		_ = putStr(bkState, "active_preset", backup.PresetID)
	}
	backup.Expected = formatEnv(next)
	if backup.PresetID != "" {
		backup.ExpectedPresetContent, _ = ReadPreset(backup.PresetID)
	}
	report.Applied = !restore
	report.CanRestore = !restore
	if restore {
		report.State = "restored"
		report.Progress = "Réglages précédents restaurés."
	} else {
		report.Progress = "Profil appliqué et contexte synchronisé avec LM Studio. Aucun redémarrage nécessaire."
	}
	_ = putJSON(bkState, "optimizer_backup", backup)
	_ = putJSON(bkState, "optimizer_report", report)
	hubOptimizer.report = report
	sendJSON(w, 200, report)
}
