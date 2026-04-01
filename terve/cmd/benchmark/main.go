// benchmark measures latency and throughput of the Voikko + Ollama stack
// for a fixed set of Finnish test phrases.
//
// Usage:
//
//	go run ./cmd/benchmark [flags]
//
// Flags:
//
//	-model    Ollama model name (default: $OLLAMA_MODEL or qwen2.5:32b-instruct-q4_K_M)
//	-ollama   Ollama base URL   (default: $OLLAMA_URL   or http://localhost:11434)
//	-voikko   Voikko base URL   (default: $VOIKKO_URL   or http://localhost:8000)
//	-runs     Repetitions per test case (default: 1)
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
	Response          string  `json:"response"`
	EvalCount         int     `json:"eval_count"`
	EvalDuration      int64   `json:"eval_duration"`       // nanoseconds
	PromptEvalCount   int     `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"` // nanoseconds
	TotalDuration     int64   `json:"total_duration"`       // nanoseconds
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

// result holds the outcome of one test case run.
type result struct {
	testCase
	voikkoMs    float64
	ollamaMs    float64
	promptTokens int
	evalTokens  int
	tokensPerSec float64
	response    string
	err         error
}

func main() {
	model := flag.String("model", envOr("OLLAMA_MODEL", "qwen2.5:32b-instruct-q4_K_M"), "Ollama model name")
	ollamaURL := flag.String("ollama", envOr("OLLAMA_URL", "http://localhost:11434"), "Ollama base URL")
	voikkoURL := flag.String("voikko", envOr("VOIKKO_URL", "http://localhost:8000"), "Voikko base URL")
	runs := flag.Int("runs", 1, "Repetitions per test case (results are averaged)")
	flag.Parse()

	vc := voikko.NewClient(*voikkoURL)
	httpClient := &http.Client{Timeout: 300 * time.Second}

	fmt.Printf("Model:  %s\n", *model)
	fmt.Printf("Ollama: %s\n", *ollamaURL)
	fmt.Printf("Voikko: %s\n", *voikkoURL)
	fmt.Printf("Runs:   %d per test case\n\n", *runs)

	// Check connectivity
	fmt.Print("Checking Voikko... ")
	if _, err := vc.AnalyzeWord("talo"); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK")

	fmt.Print("Checking Ollama... ")
	if err := checkOllama(*ollamaURL, *model, httpClient); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK\n")

	var results []result

	for _, tc := range testCases {
		fmt.Printf("  Running: %s (%q)...", tc.label, tc.text)

		var totalVoikkoMs, totalOllamaMs float64
		var totalPromptTokens, totalEvalTokens int
		var lastResponse string

		for i := 0; i < *runs; i++ {
			r := runTestCase(tc, vc, *ollamaURL, *model, httpClient)
			if r.err != nil {
				fmt.Printf(" ERROR: %v\n", r.err)
				results = append(results, r)
				goto next
			}
			totalVoikkoMs += r.voikkoMs
			totalOllamaMs += r.ollamaMs
			totalPromptTokens += r.promptTokens
			totalEvalTokens += r.evalTokens
			lastResponse = r.response
		}

		results = append(results, result{
			testCase:     tc,
			voikkoMs:     totalVoikkoMs / float64(*runs),
			ollamaMs:     totalOllamaMs / float64(*runs),
			promptTokens: totalPromptTokens / *runs,
			evalTokens:   totalEvalTokens / *runs,
			tokensPerSec: float64(totalEvalTokens) / (totalOllamaMs / 1000.0),
			response:     lastResponse,
		})
		fmt.Printf(" done (%.0f ms)\n", totalOllamaMs/float64(*runs))
	next:
	}

	printResults(results, *model)
}

func runTestCase(tc testCase, vc *voikko.Client, ollamaURL, model string, hc *http.Client) result {
	r := result{testCase: tc}

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

func printResults(results []result, model string) {
	sep := strings.Repeat("─", 100)
	fmt.Printf("\n%s\n", sep)
	fmt.Printf("Results for model: %s\n", model)
	fmt.Printf("%s\n\n", sep)

	fmt.Printf("%-32s  %8s  %8s  %8s  %8s  %9s\n",
		"Test Case", "Voikko", "Ollama", "P.Tokens", "E.Tokens", "Tok/sec")
	fmt.Printf("%-32s  %8s  %8s  %8s  %8s  %9s\n",
		strings.Repeat("-", 32), "--------", "--------", "--------", "--------", "---------")

	var totalOllama, totalTps float64
	var count int

	for _, r := range results {
		if r.err != nil {
			fmt.Printf("%-32s  ERROR: %v\n", r.label, r.err)
			continue
		}
		fmt.Printf("%-32s  %7.0fms  %7.0fms  %8d  %8d  %8.1f\n",
			r.label, r.voikkoMs, r.ollamaMs, r.promptTokens, r.evalTokens, r.tokensPerSec)
		totalOllama += r.ollamaMs
		totalTps += r.tokensPerSec
		count++
	}

	if count > 0 {
		fmt.Printf("\n%-32s  %8s  %7.0fms  %8s  %8s  %8.1f\n",
			"AVERAGE", "", totalOllama/float64(count), "", "", totalTps/float64(count))
	}

	fmt.Printf("\n%s\n\n", sep)

	// Show response snippets
	fmt.Println("Response previews (last run):")
	fmt.Println()
	for _, r := range results {
		if r.err != nil {
			continue
		}
		preview := r.response
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		fmt.Printf("  [%s]\n  %s\n\n", r.label, preview)
	}
}

func checkOllama(baseURL, model string, hc *http.Client) error {
	body, _ := json.Marshal(map[string]string{"name": model})
	resp, err := hc.Post(baseURL+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
