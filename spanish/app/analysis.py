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


def _decompose_enclitic(token):
    """
    Detect a fused verb+clitic token via spaCy's space-in-lemma signal.
    e.g. "dímelo" → lemma "decir me lo" → base verb "decir", clitics ["me", "lo"]

    Returns (base_verb_lemma, [clitic, ...]) or None if not a fusion.
    """
    if " " not in token.lemma_:
        return None
    parts = token.lemma_.split()
    return parts[0], parts[1:]


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
