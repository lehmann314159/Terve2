# Terve — Finnish Reading Comprehension App: Design Document

**Status:** CONCEPT — pre-development  
**Date:** March 2026  
**Depends on:** EXP-007 (Voikko MCP architecture, validated)

---

## Overview

A web application for learning Finnish through native texts. Two integrated
sections: a **Reading** section that surfaces real Finnish content at the
user's level and supports interactive phrase analysis, and a **Lessons**
section that provides structured grammar instruction and flashcards driven
by what the user encounters in reading.

The user starts at zero (A1) and progresses through CEFR levels as
vocabulary and grammar confidence grows.

---

## CEFR Level Framework

CEFR (Common European Framework of Reference) provides the complexity
scale for both text selection and lesson content.

| Level | Description | Finnish grammar scope | Text source |
|-------|-------------|----------------------|-------------|
| A1 | Complete beginner | Nominative, basic verb conjugation (present), SVO sentences | YLE Selkosuomi (simple) |
| A2 | Elementary | Partitive, accusative, basic locative cases (inessive/illative/adessive) | YLE Selkosuomi (full) |
| B1 | Intermediate | All 15 cases, perfect tense, basic modal constructions | YLE regular news |
| B2 | Upper-intermediate | Possessive suffixes, passive voice, conditional, rare cases | Helsingin Sanomat |
| C1 | Advanced | Full case system mastery, aspect nuance, idiomatic usage | Literature, HS long reads |

The user's current level is stored and used to filter text sources and
determine which lesson content is relevant.

---

## Architecture

```
Browser (Go templates + htmx frontend)
    ↓ HTTP
Go backend (REST API + template rendering)
    ├── Text fetcher (YLE/HS RSS/scraper by CEFR level)
    ├── Voikko service (morphological analysis — Python sidecar, persistent)
    ├── Ollama client (LLM explanations — Qwen2.5 72B)
    ├── User model (SQLite — progress, flashcards, seen texts)
    └── Lesson engine (deterministic grammar tables + LLM examples)
```

**Language choice rationale:**
- Frontend: Go templates + htmx. Most interactions are server-driven (click →
  server responds → DOM swap), which htmx handles cleanly. No build pipeline,
  no npm, one language throughout. One small vanilla JS module (~50 lines)
  handles multi-word phrase selection via the browser Selection API, firing an
  htmx request on mouseup.
- Backend: Go (fits existing GX10 project patterns, good for REST APIs)
- Voikko sidecar: Python (official bindings; called via HTTP from Go backend)
- The Voikko Python process from EXP-007 becomes a persistent FastAPI/Flask
  service inside Docker Compose. Spawn-per-request would cost ~250–350ms per
  click on the GX10 — acceptable in isolation but noticeable in a reading flow.

**Infrastructure:** Follows existing GX10 Docker + Caddy pattern.
Subdomain: `terve.verynormalserver.com` (repurposing the existing terve slot
once the old app is retired or merged).

---

## Reading Section

### Text Sourcing

Texts are fetched from Finnish-language sources filtered by CEFR level.
Each text is stored locally on first fetch to avoid repeated network calls
and to build a personal corpus over time.

**Source mapping:**
- A1/A2: YLE Selkosuomi RSS — `https://yle.fi/uutiset/osasto/selkouutiset/`
- B1: YLE news RSS — `https://yle.fi/uutiset/rss/uutiset.rss`
- B2/C1: Helsingin Sanomat (public articles)

A text is pre-screened by the backend before display: Voikko runs
`validate_sentence` on each sentence and computes a complexity score
(proportion of non-nominative forms, number of unique cases present,
presence of possessive suffixes). Texts that score outside the target
CEFR band for the user's level are skipped.

### Display

Text is displayed sentence by sentence, rendered as individually
selectable tokens. The user can:

1. **Click a single word** — triggers word-level analysis
2. **Click and drag across multiple words** — triggers phrase-level analysis
3. **Click a sentence** — triggers full sentence breakdown
4. **Type their own English translation** of a sentence and submit it

The selection model uses the browser's native Selection API, which handles
multi-word phrases naturally. The selected text is sent to the backend as a
string; the backend calls Voikko on each token and the LLM for the
explanation.

### Analysis Response

When the user selects text, a panel slides in (or appears below the text)
showing:

- Each word's lemma, case/tense/mood (from Voikko — deterministic)
- An English explanation of what the form means and why it's used here
  (from LLM, grounded in Voikko output)
- For phrases: how the words relate to each other grammatically
- A **"Add to flashcards"** button
- A **"This is a gap for me"** button that tags the construction for
  lesson generation

### User Translation Mode

The user can toggle a sentence into "translation mode":
- The sentence is dimmed
- A text input appears
- The user types their English translation and submits
- The backend sends both the Finnish sentence and the user's attempt to
  the LLM, which identifies what was correct, what was missed, and what
  was wrong — without revealing the full translation upfront
- Gaps identified here are automatically tagged for lesson and flashcard
  generation

---

## Lesson Section

### Structure

Lessons are organized by CEFR level and grammar topic. Content has two layers:

**Deterministic layer (no LLM):**
- Case tables (all 15 cases, suffix forms, vowel harmony variants)
- Verb conjugation paradigms (present, past, perfect, conditional)
- Consonant gradation tables
- These are pre-built and served directly — fast, reliable, always correct

**LLM layer:**
- Contextual explanations of *why* a rule works
- Example sentences generated on demand
- Explanations grounded in words/constructions the user has actually
  encountered in reading (personalized examples)

### Lesson Generation from Reading

When the user tags a construction as a gap (either explicitly or via
translation mode), the backend:

1. Identifies the grammar category from the Voikko analysis
   (e.g. "inessive plural with possessive suffix")
2. Checks whether a lesson for that category exists at the user's level
3. If yes: surfaces it with the user's actual sentence as the example
4. If no: generates a focused mini-lesson using the LLM, grounded in
   the specific construction encountered

This means the lesson section grows organically from reading rather than
being a fixed curriculum the user works through linearly.

### Flashcards

Each flashcard stores:
- The Finnish word or phrase (front)
- The English meaning (back)
- The source sentence (context, shown on flip)
- The Voikko morphological analysis (shown on flip as grammar annotation)
- CEFR level tag
- Spaced repetition metadata (times seen, times correct, next due date)

Spaced repetition uses a simple SM-2 algorithm — well-understood, easy to
implement in Go, no external dependency needed.

Flashcard review is a separate mode in the lesson section. Cards due for
review are surfaced first; new cards from recent reading are introduced
gradually.

---

## User Model (SQLite)

```
texts         — fetched texts, CEFR level, source, date, seen flag
selections    — each word/phrase the user selected, with analysis
flashcards    — cards with SRS metadata
gaps          — tagged constructions driving lesson generation
user_level    — current CEFR level per skill (reading, grammar)
progress      — texts completed, flashcards mastered, lessons viewed
```

Starting state: user level A1, empty flashcard deck, no seen texts.
The app works immediately from first launch.

---

## Division of Labor: Voikko vs. LLM

| Task | Handled by | Reason |
|------|-----------|--------|
| Word validity | Voikko | Deterministic |
| Lemma extraction | Voikko | Deterministic |
| Case/tense/mood identification | Voikko | Deterministic |
| Possessive suffix identification | Voikko | Deterministic |
| Vowel harmony | Voikko | Deterministic |
| Text complexity scoring | Voikko | Computed from analysis |
| Case tables, conjugation paradigms | Pre-built data | Deterministic |
| Consonant gradation tables | Pre-built data | Deterministic |
| Contextual explanation (why) | LLM | Requires reasoning |
| Translation attempt evaluation | LLM | Requires reasoning |
| Mini-lesson generation | LLM | Requires generation |
| Personalized examples | LLM | Requires generation |

The LLM never makes morphological claims from its own knowledge — it only
explains and contextualizes what Voikko has already determined.

---

## Open Questions

- [x] **OQ-A:** How to handle texts that mix CEFR levels within a single
      article? **Decision: whole-article score.** Score the article once and
      assign a single level. Sentence-by-sentence scoring can be added in
      Phase 5 polish if mixed-level content proves to be a real problem in
      practice.

- [x] **OQ-B:** YLE Selkosuomi RSS may not provide enough A1-level content
      volume for sustained daily use. **Decision: research first, then build
      with multiple sources in mind.** Identified candidates beyond YLE
      Selkosuomi:
      - FluencyDrop (fluencydrop.com/stories/finnish) — CEFR-tagged graded
        stories A1–C2, free web, scrapeable. AI-generated but explicitly
        level-labeled; good programmatic fallback.
      - Lingua.com (lingua.com/finnish/reading/) — small corpus of genuine
        A1 texts. Low volume but real content.
      - Sano suomeksi (sanosuomeksi.com) — A1–B2 short reading texts
        organized by level.
      Phase 1 uses YLE Selkosuomi only. FluencyDrop is the Phase 5 fallback
      if Selkosuomi volume proves insufficient.

- [x] **OQ-C:** Phrase selection boundary detection — when the user selects
      part of a noun phrase, should the app expand to the full phrase?
      **Decision: offer expansion as a suggestion.** Voikko detects that the
      selection is partial; the full phrase is highlighted with a subtle
      "Expand?" affordance. User keeps control; smart hint is available.

- [x] **OQ-D:** Should the Voikko sidecar be a persistent HTTP service
      (always running, fast) or spawned per request (simpler, slower)?
      **Decision: persistent HTTP service.** Spawn cost on the GX10 is
      ~250–350ms (Python startup + FST load) — under the one-second threshold
      but noticeable on every word click. Persistent sidecar in Docker Compose
      alongside the main app.

- [x] **OQ-E:** Model for translation evaluation — the LLM needs to assess
      a learner's English translation of a Finnish sentence without being
      overly strict (word-for-word) or too lenient. **Decision: test both
      approaches in Phase 3 and decide from real usage.** Two candidates:
      (1) Simple rubric — "correct meaning conveyed" only, lenient on
      word-for-word mapping. (2) Structured rubric — meaning conveyed +
      key grammar elements reflected + missing nuances noted. Implement
      both as system prompt variants; Mike evaluates against actual Finnish
      comprehension (near-fluency in Spanish gives a reference point for
      what useful feedback feels like).

---

## Development Phases

**Phase 1 — Core reading loop (MVP)**
- Backend: text fetcher (YLE Selkosuomi only), Voikko sidecar HTTP service,
  single analysis endpoint
- Frontend: text display with single-word click analysis, analysis panel
- No user model yet — stateless

**Phase 2 — Phrase selection + persistence**
- Multi-word selection via Selection API
- SQLite user model: seen texts, selections log
- Flashcard creation from selections

**Phase 3 — Translation mode + lesson generation**
- User translation input and LLM evaluation
- Gap tagging
- Basic lesson surfacing (pre-built grammar tables)

**Phase 4 — Spaced repetition + CEFR progression**
- SM-2 flashcard review
- CEFR level tracking and text difficulty filtering
- LLM-generated mini-lessons from tagged gaps

**Phase 5 — Polish**
- Multiple text sources (YLE regular, HS)
- Smart phrase boundary expansion
- Progress visualisation
