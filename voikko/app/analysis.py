"""
analysis.py — Finnish morphological analysis functions using Voikko.

Ported from chat.py (lines 78-232). These functions wrap libvoikko to provide
clean, English-labeled morphological analysis of Finnish words.
"""

import libvoikko

voikko = libvoikko.Voikko("fi")

# Voikko returns raw Finnish grammar tag names. We translate them to readable English.
case_map = {
    "nimento": "nominative", "omanto": "genitive", "kohdanto": "accusative",
    "osanto": "partitive", "sisaolento": "inessive", "sisaeronto": "elative",
    "sisatulento": "illative", "ulkoolento": "adessive", "ulkoeronto": "ablative",
    "ulkotulento": "allative", "olento": "essive", "muunto": "translative",
    "keinonto": "instructive", "vajanto": "abessive", "seuranto": "comitative",
}

number_map = {
    "yksikkö": "singular", "monikko": "plural",
    "singular": "singular", "plural": "plural",
}


def analyze_word(word: str) -> list:
    """Full morphological analysis of a single Finnish word."""
    raw_analyses = voikko.analyze(word)
    if not raw_analyses:
        return []

    results = []
    for raw in raw_analyses:
        wc = raw.get("CLASS", raw.get("WCLASS", ""))
        lemma = raw.get("BASEFORM", "")
        case_raw = raw.get("SIJAMUOTO", "")
        num_raw = raw.get("NUMBER", "")

        entry = {
            "lemma": lemma,
            "word_class": wc.lower() if wc else "unknown",
            "case": case_map.get(case_raw.lower(), case_raw.lower()) if case_raw else None,
            "number": number_map.get(num_raw.lower(), num_raw.lower()) if num_raw else None,
            "person": raw.get("PERSON", None),
            "tense": raw.get("TENSE", None),
            "mood": raw.get("MOOD", None),
            "possessive": raw.get("POSSESSIVE", None),
        }
        entry = {k: v for k, v in entry.items() if v is not None}
        results.append(entry)

    return results


def validate_word(word: str) -> dict:
    """Check if a Finnish word is valid and return its analyses."""
    analyses = analyze_word(word)
    return {
        "word": word,
        "valid": voikko.spell(word),
        "analyses": analyses,
        "ambiguous": len(analyses) > 1,
    }


def validate_sentence(sentence: str) -> dict:
    """Tokenize a Finnish sentence and validate/analyze every word."""
    tokens = voikko.tokens(sentence)
    result_tokens = []

    for token in tokens:
        text = token.tokenText

        if token.tokenType == libvoikko.Token.WORD:
            valid = voikko.spell(text)
            analyses = analyze_word(text)
            result_tokens.append({
                "token": text,
                "type": "word",
                "valid": valid,
                "analyses": analyses[:2],
            })
        else:
            result_tokens.append({
                "token": text,
                "type": "punctuation" if token.tokenType == libvoikko.Token.PUNCTUATION else "other",
            })

    invalid = [t["token"] for t in result_tokens
               if t.get("type") == "word" and not t.get("valid")]

    return {
        "sentence": sentence,
        "tokens": result_tokens,
        "invalid_words": invalid,
        "all_valid": len(invalid) == 0,
    }


def get_vowel_harmony(word: str) -> dict:
    """Determine vowel harmony class (front/back/neutral/mixed)."""
    front = set("äöy")
    back = set("aou")
    wl = word.lower()

    fc = sum(1 for c in wl if c in front)
    bc = sum(1 for c in wl if c in back)

    if fc > 0 and bc == 0:
        harmony = "front"
    elif bc > 0 and fc == 0:
        harmony = "back"
    elif fc > 0 and bc > 0:
        harmony = "mixed"
    else:
        harmony = "neutral"

    return {
        "word": word,
        "harmony": harmony,
        "suffix_vowel_a": "ä" if harmony == "front" else "a",
        "suffix_vowel_o": "ö" if harmony == "front" else "o",
        "suffix_vowel_u": "y" if harmony == "front" else "u",
    }


def get_suggestions(word: str, max_suggestions: int = 5) -> dict:
    """Get spelling suggestions for a potentially malformed word."""
    return {
        "word": word,
        "valid": voikko.spell(word),
        "suggestions": voikko.suggest(word)[:max_suggestions] or [],
    }


def validate_words_batch(words: list) -> list:
    """Validate a list of words in one call."""
    return [
        {
            "word": w,
            "valid": voikko.spell(w),
            "analyses": analyze_word(w)[:1],
        }
        for w in words
    ]
