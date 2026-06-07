"""
Spanish morphological analysis pipeline.

Tool layers:
  1. spaCy es_core_news_lg — lemma, POS, morphological features
  2. Enclitic decomposer — splits fused verb+clitic tokens (e.g. "dímelo")
  3. mlconjug3 — enriches verb conjugation features missing from spaCy
"""

import spacy
from mlconjug3 import Conjugator

# Load once at module init — not per request.
nlp = spacy.load("es_core_news_lg")
conjugator = Conjugator(language="es")


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
# mlconjug3 enrichment
# ---------------------------------------------------------------------------

_MOOD_MAP = {
    "Indicativo":  "Ind",
    "Subjuntivo":  "Sub",
    "Imperativo":  "Imp",
    "Condicional": "Cnd",
    "Infinitivo":  "Inf",
    "Gerundio":    "Ger",
    "Participio":  "Part",
}

_TENSE_MAP = {
    "Presente":                      "Pres",
    "Pretérito perfecto compuesto":  "Perf",
    "Pretérito imperfecto":          "Imp",
    "Pretérito indefinido":          "Past",
    "Pretérito pluscuamperfecto":    "Pqp",
    "Futuro":                        "Fut",
    "Condicional":                   "Cnd",
}

_PERSON_MAP = {
    "yo":                      "1",
    "tú":                      "2",
    "él/ella/usted":           "3",
    "nosotros/-as":            "1",
    "nosotros":                "1",
    "vosotros/-as":            "2",
    "vosotros":                "2",
    "ellos/ellas/ustedes":     "3",
}


def _enrich_verb_features(surface, lemma, spacy_features):
    """
    Use mlconjug3 to find the conjugated form matching `surface` and add
    any Mood/Tense/Person features that spaCy left unset.
    """
    try:
        conjugated = conjugator.conjugate(lemma)
        if conjugated is None:
            return spacy_features

        surface_lower = surface.lower()
        existing_keys = {f.split("=")[0] for f in spacy_features}

        for mood_name, tenses in conjugated.conjug_info.items():
            for tense_name, persons in tenses.items():
                if not isinstance(persons, dict):
                    continue
                for person_key, form in persons.items():
                    if not form or form.lower() != surface_lower:
                        continue
                    # Found match — fill in missing features
                    extra = []
                    if "Mood" not in existing_keys and mood_name in _MOOD_MAP:
                        extra.append(f"Mood={_MOOD_MAP[mood_name]}")
                    if "Tense" not in existing_keys and tense_name in _TENSE_MAP:
                        extra.append(f"Tense={_TENSE_MAP[tense_name]}")
                    if "Person" not in existing_keys and person_key in _PERSON_MAP:
                        extra.append(f"Person={_PERSON_MAP[person_key]}")
                    return spacy_features + extra
    except Exception:
        pass
    return spacy_features


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
            features = _enrich_verb_features(token.text, base_verb, spacy_features)
            tokens.append({
                "surface": token.text,
                "lemma":   base_verb,
                "pos":     "verb",
                "features": features,
            })
            for clitic in clitics:
                tokens.append({
                    "surface": clitic,
                    "lemma":   clitic,
                    "pos":     "pron",
                    "features": [],
                })
        else:
            pos = token.pos_.lower()
            features = spacy_features
            if pos == "verb":
                features = _enrich_verb_features(token.text, token.lemma_, spacy_features)
            tokens.append({
                "surface": token.text,
                "lemma":   token.lemma_,
                "pos":     pos,
                "features": features,
            })

    return tokens
