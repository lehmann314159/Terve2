// benchmark measures latency and throughput of the Voikko + Ollama stack
// for a fixed set of Finnish test phrases, across one or more models.
//
// Usage:
//
//	go run ./cmd/benchmark [flags]
//
// Flags:
//
//	-models   Comma-separated list of Ollama model names
//	          (default: $OLLAMA_MODEL or qwen2.5:32b-instruct-q4_K_M)
//	-ollama   Ollama base URL (default: $OLLAMA_URL or http://localhost:11434)
//	-voikko   Voikko base URL (default: $VOIKKO_URL or http://localhost:8000)
//	-runs     Repetitions per test case per model (results are averaged)
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lehmann314159/terve2/internal/ollama"
	"github.com/lehmann314159/terve2/internal/voikko"
)

// testCase is one Finnish phrase to analyze and explain.
type testCase struct {
	label   string
	text    string
	context string
}

var testCases = []testCase{
	{
		label:   "single common word",
		text:    "olen",
		context: "Minä olen opiskelija.",
	},
	{
		label:   "inflected noun",
		text:    "talossa",
		context: "Hän asuu talossa.",
	},
	{
		label:   "past tense verb",
		text:    "juoksin",
		context: "Juoksin kotiin nopeasti.",
	},
	{
		label:   "partitive object",
		text:    "kahvia",
		context: "Juon kahvia joka aamu.",
	},
	{
		label:   "short phrase",
		text:    "minä olen kotona",
		context: "Minä olen kotona tänään.",
	},
	{
		label:   "longer phrase",
		text:    "Hän meni kauppaan ostamaan leipää",
		context: "Hän meni kauppaan ostamaan leipää ja maitoa.",
	},
	{
		label:   "tricky morphology",
		text:    "kävelemässä",
		context: "Hän on kävelemässä puistossa.",
	},
	{
		label:   "compound word",
		text:    "kirjastokortti",
		context: "Tarvitsen kirjastokortin lainaamista varten.",
	},
}

// ollamaMetrics holds timing data returned by the Ollama API.
type ollamaMetrics struct {
	Response           string `json:"response"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`        // nanoseconds
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"` // nanoseconds
	TotalDuration      int64  `json:"total_duration"`       // nanoseconds
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

// result holds the outcome of one test case for one model.
type result struct {
	testCase
	model        string
	voikkoMs     float64
	ollamaMs     float64
	promptTokens int
	evalTokens   int
	tokensPerSec float64
	response     string
	err          error
}

// modelSummary holds averaged stats for one model.
type modelSummary struct {
	model        string
	avgOllamaMs  float64
	avgTps       float64
	results      []result
}

func main() {
	modelsFlag := flag.String("models", envOr("OLLAMA_MODEL", "qwen2.5:32b-instruct-q4_K_M"), "Comma-separated list of model names")
	ollamaURL := flag.String("ollama", envOr("OLLAMA_URL", "http://localhost:11434"), "Ollama base URL")
	voikkoURL := flag.String("voikko", envOr("VOIKKO_URL", "http://localhost:8000"), "Voikko base URL")
	runs := flag.Int("runs", 1, "Repetitions per test case (results are averaged)")
	flag.Parse()

	models := splitModels(*modelsFlag)

	vc := voikko.NewClient(*voikkoURL)
	httpClient := &http.Client{Timeout: 300 * time.Second}

	fmt.Printf("Models: %s\n", strings.Join(models, ", "))
	fmt.Printf("Ollama: %s\n", *ollamaURL)
	fmt.Printf("Voikko: %s\n", *voikkoURL)
	fmt.Printf("Runs:   %d per test case per model\n\n", *runs)

	// Check Voikko once
	fmt.Print("Checking Voikko... ")
	if _, err := vc.AnalyzeWord("talo"); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK")

	// Check each model is available
	for _, m := range models {
		fmt.Printf("Checking %-40s ", m+"...")
		if err := checkOllama(*ollamaURL, m, httpClient); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
	}
	fmt.Println()

	// Run all models
	var summaries []modelSummary
	for _, m := range models {
		fmt.Printf("── Model: %s ──\n", m)
		summary := runModel(m, vc, *ollamaURL, httpClient, *runs)
		summaries = append(summaries, summary)
		fmt.Println()
	}

	printComparison(summaries)
	printPreviews(summaries)
}

func runModel(model string, vc *voikko.Client, ollamaURL string, hc *http.Client, runs int) modelSummary {
	var results []result
	var totalOllamaMs, totalTps float64
	var count int

	for _, tc := range testCases {
		fmt.Printf("  %-34s ", tc.label+"...")

		var sumVoikkoMs, sumOllamaMs float64
		var sumPromptTokens, sumEvalTokens int
		var lastResponse string
		var lastErr error

		for i := 0; i < runs; i++ {
			r := runTestCase(tc, vc, ollamaURL, model, hc)
			if r.err != nil {
				lastErr = r.err
				break
			}
			sumVoikkoMs += r.voikkoMs
			sumOllamaMs += r.ollamaMs
			sumPromptTokens += r.promptTokens
			sumEvalTokens += r.evalTokens
			lastResponse = r.response
		}

		if lastErr != nil {
			fmt.Printf("ERROR: %v\n", lastErr)
			results = append(results, result{testCase: tc, model: model, err: lastErr})
			continue
		}

		r := result{
			testCase:     tc,
			model:        model,
			voikkoMs:     sumVoikkoMs / float64(runs),
			ollamaMs:     sumOllamaMs / float64(runs),
			promptTokens: sumPromptTokens / runs,
			evalTokens:   sumEvalTokens / runs,
			tokensPerSec: float64(sumEvalTokens) / (sumOllamaMs / 1000.0),
			response:     lastResponse,
		}
		results = append(results, r)
		fmt.Printf("%6.0fms  %5.1f tok/s\n", r.ollamaMs, r.tokensPerSec)
		totalOllamaMs += r.ollamaMs
		totalTps += r.tokensPerSec
		count++
	}

	if count > 0 {
		fmt.Printf("  %-34s %6.0fms  %5.1f tok/s\n", "AVERAGE",
			totalOllamaMs/float64(count), totalTps/float64(count))
	}

	return modelSummary{
		model:       model,
		avgOllamaMs: totalOllamaMs / float64(max(count, 1)),
		avgTps:      totalTps / float64(max(count, 1)),
		results:     results,
	}
}

func printComparison(summaries []modelSummary) {
	if len(summaries) < 2 {
		return
	}

	sep := strings.Repeat("─", 72)
	fmt.Printf("%s\n", sep)
	fmt.Println("Model Comparison Summary")
	fmt.Printf("%s\n\n", sep)

	// Header
	fmt.Printf("%-44s  %10s  %10s\n", "Model", "Avg ms", "Avg tok/s")
	fmt.Printf("%-44s  %10s  %10s\n", strings.Repeat("-", 44), "----------", "----------")

	// Find best for highlighting
	var bestTps, bestMs float64
	for i, s := range summaries {
		if i == 0 || s.avgTps > bestTps {
			bestTps = s.avgTps
		}
		if i == 0 || s.avgOllamaMs < bestMs {
			bestMs = s.avgOllamaMs
		}
	}

	for _, s := range summaries {
		tpsMarker := ""
		msMarker := ""
		if s.avgTps == bestTps {
			tpsMarker = " ◀ fastest"
		}
		if s.avgOllamaMs == bestMs {
			msMarker = " ◀ lowest latency"
		}
		fmt.Printf("%-44s  %9.0fms  %8.1f%s%s\n",
			s.model, s.avgOllamaMs, s.avgTps, tpsMarker, msMarker)
	}

	// Per-test comparison table
	fmt.Printf("\n%s\n", sep)
	fmt.Println("Tokens/sec by test case")
	fmt.Printf("%s\n\n", sep)

	// Column header
	fmt.Printf("%-32s", "Test Case")
	for _, s := range summaries {
		name := s.model
		if len(name) > 14 {
			// shorten to last segment after ":"
			if i := strings.LastIndex(name, ":"); i >= 0 {
				name = name[i+1:]
			}
			if len(name) > 14 {
				name = name[:14]
			}
		}
		fmt.Printf("  %10s", name)
	}
	fmt.Println()
	fmt.Printf("%-32s", strings.Repeat("-", 32))
	for range summaries {
		fmt.Printf("  %10s", "----------")
	}
	fmt.Println()

	for i, tc := range testCases {
		fmt.Printf("%-32s", tc.label)
		for _, s := range summaries {
			if i < len(s.results) && s.results[i].err == nil {
				fmt.Printf("  %9.1f ", s.results[i].tokensPerSec)
			} else {
				fmt.Printf("  %10s", "ERR")
			}
		}
		fmt.Println()
	}

	fmt.Printf("%-32s", "AVERAGE")
	for _, s := range summaries {
		fmt.Printf("  %9.1f ", s.avgTps)
	}
	fmt.Println()
	fmt.Printf("\n%s\n\n", sep)
}

func printPreviews(summaries []modelSummary) {
	if len(summaries) == 0 {
		return
	}

	sep := strings.Repeat("═", 72)
	fmt.Printf("%s\n", sep)
	fmt.Println("FULL RESPONSES — paste below this line to Claude for quality analysis")
	fmt.Printf("%s\n\n", sep)

	for i, tc := range testCases {
		fmt.Printf("━━━ Test case %d: %s ━━━\n", i+1, tc.label)
		fmt.Printf("Finnish:  %s\n", tc.text)
		fmt.Printf("Context:  %s\n\n", tc.context)

		for _, s := range summaries {
			if i >= len(s.results) {
				continue
			}
			r := s.results[i]
			fmt.Printf("  Model: %s\n", s.model)
			if r.err != nil {
				fmt.Printf("  ERROR: %v\n\n", r.err)
				continue
			}
			// Strip <think>...</think> block if present so the judge
			// sees only the final answer, same as what the app uses.
			response := stripThinking(r.response)
			translation, explanation := ollama.ParseResponse(response)
			fmt.Printf("  TRANSLATION: %s\n", translation)
			fmt.Printf("  EXPLANATION: %s\n\n", explanation)
		}
	}
}

func runTestCase(tc testCase, vc *voikko.Client, ollamaURL, model string, hc *http.Client) result {
	r := result{testCase: tc, model: model}

	// --- Voikko ---
	t0 := time.Now()
	sv, err := vc.ValidateSentence(tc.text)
	r.voikkoMs = float64(time.Since(t0).Milliseconds())
	if err != nil {
		r.err = fmt.Errorf("voikko: %w", err)
		return r
	}

	// --- Build prompt ---
	prompt := ollama.BuildPrompt(tc.text, tc.context, sv.Tokens)

	// --- Ollama ---
	req := generateRequest{
		Model:  model,
		Prompt: prompt,
		System: ollama.SystemPrompt,
		Stream: false,
	}
	body, _ := json.Marshal(req)

	t1 := time.Now()
	resp, err := hc.Post(ollamaURL+"/api/generate", "application/json", bytes.NewReader(body))
	r.ollamaMs = float64(time.Since(t1).Milliseconds())
	if err != nil {
		r.err = fmt.Errorf("ollama request: %w", err)
		return r
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		r.err = fmt.Errorf("ollama read: %w", err)
		return r
	}

	var metrics ollamaMetrics
	if err := json.Unmarshal(raw, &metrics); err != nil {
		r.err = fmt.Errorf("ollama parse: %w", err)
		return r
	}

	r.promptTokens = metrics.PromptEvalCount
	r.evalTokens = metrics.EvalCount
	if metrics.EvalDuration > 0 {
		r.tokensPerSec = float64(metrics.EvalCount) / (float64(metrics.EvalDuration) / 1e9)
	}
	r.response = metrics.Response
	return r
}

// stripThinking removes <think>...</think> blocks that Qwen3 models emit
// before their actual response.
func stripThinking(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s, "</think>")
		if end == -1 {
			s = s[:start]
			break
		}
		s = s[:start] + s[end+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

func checkOllama(baseURL, model string, hc *http.Client) error {
	body, _ := json.Marshal(map[string]string{"name": model})
	resp, err := hc.Post(baseURL+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d — is the model pulled?", resp.StatusCode)
	}
	return nil
}

func splitModels(s string) []string {
	var models []string
	for _, m := range strings.Split(s, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}
	return models
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
