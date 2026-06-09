"""
Spanish morphological analysis pipeline.

Tool layers:
  1. spaCy es_core_news_lg — lemma, POS, morphological features
  2. Enclitic decomposer — splits fused verb+clitic tokens (e.g. "dímelo")
"""

import spacy

# Load once at module init — not per request.
nlp = spacy.load("es_core_news_lg")


# ---------------------------------------------------------------------------
# Enclitic decomposer
# ---------------------------------------------------------------------------

# Object/reflexive clitics that fuse to verbs in Spanish.
# Ordered longest-first so "melo" is tried before "me" and "lo".
_CLITIC_PATTERNS = [
    ("selos", ["se", "los"]),
    ("selas", ["se", "las"]),
    ("melos", ["me", "los"]),
    ("melas", ["me", "las"]),
    ("noslo", ["nos", "lo"]),
    ("nosla", ["nos", "la"]),
    ("melo",  ["me", "lo"]),
    ("mela",  ["me", "la"]),
    ("telo",  ["te", "lo"]),
    ("tela",  ["te", "la"]),
    ("selo",  ["se", "lo"]),
    ("sela",  ["se", "la"]),
    ("nos",   ["nos"]),
    ("les",   ["les"]),
    ("los",   ["los"]),
    ("las",   ["las"]),
    ("me",    ["me"]),
    ("te",    ["te"]),
    ("se",    ["se"]),
    ("le",    ["le"]),
    ("lo",    ["lo"]),
    ("la",    ["la"]),
    ("os",    ["os"]),
]

# Minimum characters left after stripping clitics (avoids "ha" → "h" + ["a"]).
_MIN_BASE = 2


def _suffix_strip(word):
    """
    Try stripping a known clitic suffix from *word* (lowercased).
    Returns (base, [clitics]) or None.
    """
    w = word.lower()
    for suffix, clitics in _CLITIC_PATTERNS:
        if w.endswith(suffix) and len(w) - len(suffix) >= _MIN_BASE:
            base = w[:-len(suffix)]
            return base, clitics
    return None


def _decompose_enclitic(token):
    """
    Detect a fused verb+clitic token and return (base_lemma, [clitics]).
    Returns None if the token is not a fusion.

    Strategy 1 — spaCy space-in-lemma signal (reliable for words in the model):
        "dímelo" → lemma "decir me lo" → ("decir", ["me", "lo"])

    Strategy 2 — suffix stripping fallback (catches words the model doesn't know):
        "dimelo" → strip "melo" → base "di" → re-lemmatise via spaCy
        Activated only for VERB tokens where spaCy gave no space-in-lemma.
    """
    # Strategy 1
    if " " in token.lemma_:
        parts = token.lemma_.split()
        return parts[0], parts[1:]

    # Strategy 2 — only for verbs
    if token.pos_ != "VERB":
        return None

    result = _suffix_strip(token.text)
    if result is None:
        return None

    base, clitics = result
    # Re-run spaCy on the isolated base to get a proper lemma.
    base_doc = nlp(base)
    base_lemma = base_doc[0].lemma_ if base_doc else base
    return base_lemma, clitics


# ---------------------------------------------------------------------------
# Main analysis entry point
# ---------------------------------------------------------------------------

def analyze_text(text: str) -> list[dict]:
    """
    Analyze Spanish text and return a list of token dicts matching the
    sidecar contract expected by internal/language/spanish/driver.go:

        {"surface": str, "lemma": str, "pos": str, "features": [str]}

    Only alpha tokens are included (punctuation skipped).
    Features are spaCy morph fields formatted as Key=Value strings,
    optionally enriched by mlconjug3 for verb tokens.
    """
    doc = nlp(text)
    tokens = []

    for token in doc:
        if not token.is_alpha:
            continue

        spacy_features = list(token.morph)

        decomposed = _decompose_enclitic(token)
        if decomposed:
            base_verb, clitics = decomposed
            tokens.append({
                "surface": token.text,
                "lemma":   base_verb,
                "pos":     "verb",
                "features": spacy_features,
            })
            for clitic in clitics:
                tokens.append({
                    "surface": clitic,
                    "lemma":   clitic,
                    "pos":     "pron",
                    "features": [],
                })
        else:
            tokens.append({
                "surface": token.text,
                "lemma":   token.lemma_,
                "pos":     token.pos_.lower(),
                "features": spacy_features,
            })

    return tokens
