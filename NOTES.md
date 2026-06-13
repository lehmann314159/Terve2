# Terve2 Session Notes

## What We Built

### Two-Column Reading Layout
- Split the book reader into Finnish (left, 3fr) + analysis (right, 2fr) columns
- Analysis panel is sticky — stays in view while scrolling
- Wide monitors get near-full-width layout (`max-width: 98%`)

### Per-Word Flashcard Buttons
- Green ＋ / red ✕ buttons on each row of the morphological analysis table
- Buttons appear only when logged in
- HTMX `hx-swap="outerHTML"` swaps just the button cell on save/remove — no page reload
- Handlers: `POST /flashcards/save-word`, `POST /flashcards/remove-word`
- DB: `LookupUserCardIDsByLemmas()` pre-loads saved state so buttons render correctly on load

### CEFR Difficulty Ratings
- `difficulty` column added to `books` table (migration runs at startup, idempotent)
- Difficulty assigned via Ollama at import time — samples first 800 chars, asks for A1–C2 rating
- Displayed as color-coded badge on book cards (A1=green → C2=purple)
- Seed books have hardcoded difficulties; backfill runs at startup for existing rows with `difficulty = ''`

### Expanded Book Library
Seeded 20 Finnish books from Project Gutenberg, organized by CEFR level:

| Level | Books |
|-------|-------|
| A1 | Topelius *Lukemisia lapsille* vols 1–4, 6–8 (7 vols) |
| A2 | Härkönen folk tales (trolls, animals), Grimm fairy tales (Finnish), Andersen fairy tales (Finnish) |
| B1 | *Liisan seikkailut ihmemaassa* (Alice in Wonderland), *Pekka Poikanen* (Peter Pan) |
| B2 | Larin-Kyösti *Joulu-yön tarina*, Rauanheimo *Aamusta iltaan*, Juhani Aho *Yksin* |
| C1 | Minna Canth *Anna Liisa* + *Työmiehen vaimo*, Runeberg *Vänrikki Stoolin tarinat*, Aleksis Kivi *Seitsemän veljestä* |

Books are embedded into the binary via `//go:embed bookdata/*.txt`.

### LLM Benchmark Harness (`cmd/benchmark/`)
- Standalone Go program, also built into the Docker image
- Flags: `-models` (comma-separated), `-ollama`, `-voikko`, `-runs`
- **Warmup pass**: runs first test case cold then warm; estimates model-switch overhead
- **8 Finnish test cases**: single word → inflected noun → verb → partitive → short phrase → longer phrase → tricky morphology → compound word
- Uses Ollama's `eval_count` / `eval_duration` fields for accurate tok/s (not wall-clock)
- Multi-model comparison table with `◀ fastest` / `◀ lowest latency` markers
- Per-test tok/s grid across models
- Full structured responses printed at the end (think blocks stripped) for manual quality review
- Run via: `docker exec terve ./benchmark -models model1,model2`

---

## Model Benchmark Findings

### Hardware
**ASUS GX10** — NVIDIA GB10 Blackwell chip, 128 GB unified RAM (CPU + GPU share the same pool). Larger models fit entirely in GPU memory, so inference is fast and scales well with model size.

### Models Tested

| Model | Avg ms | Avg tok/s | Notes |
|-------|--------|-----------|-------|
| qwen2.5:32b-instruct-q4_K_M | ~8,300 | ~9.8 | **Current production model. Best balance.** |
| qwen2.5:32b-instruct-q8 | ~13,200 | ~6.1 | 37% slower, ~87s cold-start, identical quality |
| qwen3:30b-a3b | fast | high | Failed longer phrase (translated only "leipää" instead of full sentence) |
| qwen3:32b | ~42,000 | — | Highest quality, too slow for interactive use |
| llama4:scout | — | — | Crashed Ollama (67 GB exceeded available memory during run) |

### Models to Drop from Future Runs

| Model | Reason |
|-------|--------|
| `llama4:scout` | Crashed Ollama — 67 GB exceeds available memory headroom |
| `qwen2.5:32b-instruct-q8` | 37% slower, 9× worse cold-start, zero quality improvement over Q4 |
| `qwen3:32b` | ~42s average latency — too slow for interactive use; quality not worth it |
| `qwen3:30b-a3b` | Failed longer phrase test (translated only last word); `/no_think` is now added to all prompts — worth a retest before permanently dropping |

### Q4 vs Q8 Verdict
**Stick with Q4.** Across all 8 test cases:
- Translations: word-for-word identical
- Morphological breakdowns: identical
- Explanations: near-identical (minor rephrasing only)

At 32B parameters, quantization level does not meaningfully affect linguistic knowledge or explanation quality. The Q8 penalty (60% more latency, 9× worse cold-start, more RAM pressure) is not justified.

### What Actually Moves Quality
1. Prompt engineering (highest leverage, zero cost)
2. Larger model — qwen2.5:72b if memory allows
3. Qwen3 with `/no_think` — disables chain-of-thought, faster responses *(added to all prompts)*
4. A fine-tuned linguistic model (unlikely to exist for Finnish)

---

## Known Issues / Bugs

- `qwen3:30b-a3b` fails on the "longer phrase" test case — translates only the last word instead of the full phrase. Possibly a prompt parsing issue, not a fundamental capability gap. Worth investigating.
- Production server had a `permission denied` issue with `docker compose` (even with sudo). Unresolved — may need `aa-remove-unknown` or a Docker daemon restart.

---

## Future Steps

### Spanish Expansion

#### Read
- [ ] **Paste-your-own-text smoke test**: verify end-to-end in Spanish — select text → analysis panel → AI explanation. Should work already via the driver, but not confirmed.
- [ ] **Spanish article source**: the RSS feed (`articles.go`) is hardcoded to YLE Selkosuomi (Finnish). Options: (a) find an equivalent simplified-Spanish RSS feed and wire it in when `lang=es`, or (b) hide the RSS panel for ES users and rely on paste-text + books (simpler, acceptable short-term).
- [ ] **Reading page subtitle/hint text**: currently says "load Finnish articles from YLE..." — make it language-aware or generic.

#### Quiz
- [x] **Quiz hub subtitle**: language-aware ("Finnish" / "Spanish").
- [x] **Hide case-id quiz for Spanish**: hidden in hub + handler-level guard.
- [x] **form-english handler**: ported to `driverFor(r).Analyze()` for morphology fallback — works for any language.
- [x] **Finnish-only guards**: `case-id`, `declension`, `conjugation`, `cloze`, `sentence-translation` page and question handlers all guard with `finnishOnly`/`finnishOnlyPartial` (redirect or error partial). Hub hides these cards for Spanish.
- [x] **Cloze / sentence-translation for Spanish**: `ollama.GenerateSentences` now accepts `language string`; `sentence_cache` table has a `lang` column (migration); DB functions scoped by lang; quiz handlers pass `driver.Language()`. Both quiz types now available for Spanish.
- [x] **Conjugation for Spanish**: `GenerateSpanishVerbParadigm` added (present/preterite/imperfect, 18 forms); `spanishVerbFormKeys` + `verbFormKeysForLang(lang)`; paradigm DB scoped by lang; ConjugationQuestion uses driver.Analyze for verb detection; Finnish-only guard removed.
- [ ] **Declension for Spanish**: Finnish case declension concept doesn't apply. Consider a Spanish gender/number agreement quiz instead.

---

### High Priority
- [ ] **Automated quality judge**: pipe benchmark responses to Claude API for scoring (accuracy, grammar, usefulness). Discussed, deferred — likely Claude Sonnet as judge via API.
- [ ] **Investigate qwen3:30b-a3b prompt failure**: add `/no_think` flag to system prompt, retest longer phrase. If it passes, this model is worth considering (3B active params = very fast).
- [ ] **Fix production permission denied**: investigate Docker socket permissions on GX10.

### Medium Priority
- [ ] **Flashcards review UI**: actual SRS review flow (show front, flip, grade). DB schema is ready (`user_cards` table exists), handlers not yet built.
- [ ] **Favorites**: bookmark books or chapters for quick return.
- [ ] **Reading progress**: track last-read chapter per user.

### Low Priority / Ideas
- [ ] **qwen2.5:72b benchmark**: the GX10 should handle it — worth a quick run to see if quality improves enough to justify the latency increase.
- [ ] **Difficulty auto-assignment accuracy**: manually verify a sample of Ollama-assigned CEFR ratings against known difficulty levels.
- [ ] **English translation inline**: show English translation in parentheses next to the Finnish word in the analysis panel (was discussed, not implemented).

---

## Eval Harness Expansion (2026-04-02 discussion)

### Goal
Add automated quality scoring to the benchmark — currently it only measures latency/tok/s. Responses are printed for manual review with no structured quality signal.

### Proposed: Claude Sonnet as quality judge
- After all model runs complete, pipe each `(test case, response)` pair to Claude API (Sonnet)
- Judge scores on: translation accuracy, grammatical explanation correctness, conciseness (1–5), format compliance (TRANSLATION:/EXPLANATION: present)
- Output: composite quality score per model per test case — enables quality vs. speed tradeoff analysis

### Proposed: Ground-truth expected answers
- Add `expected` fields to `testCase` struct (expected translation + key morphological facts)
- Judge compares model response against expected — makes scoring objective rather than subjective
- Higher-leverage than pure LLM-as-judge alone

### Test coverage gaps identified
- No genitive case test
- No allative / ablative (directional cases)
- Nothing sourced from actual book text (all cases are constructed examples)

### Pending model retests
- `qwen3:30b-a3b`: `/no_think` was added after the longer-phrase failure — retest that single case before dropping the model (3B active params = very fast if it works)
- `qwen2.5:72b`: GX10 should handle it — one run to check quality vs. latency tradeoff

### Implementation decision pending
- Does the judge call Claude API directly from the benchmark binary, or output a structured file for a separate tool?

---

## Infrastructure Notes

- **Volume mounts** added to `docker-compose.yml`: CSS and templates are live-mounted, so changes are instant locally without a rebuild.
- **Benchmark binary** is baked into the Docker image — run it on the GX10 via `docker exec terve ./benchmark [flags]`.
- **Voikko** is internal-only (no exposed port); benchmark must run inside the Docker network.
