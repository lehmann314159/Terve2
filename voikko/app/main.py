"""
main.py — FastAPI service wrapping Voikko morphological analysis.

Provides HTTP endpoints for Finnish word/sentence analysis.
Runs as a sidecar container alongside the Go backend.
"""

from fastapi import FastAPI
from pydantic import BaseModel

from .analysis import (
    analyze_word,
    validate_word,
    validate_sentence,
    get_vowel_harmony,
    get_suggestions,
    validate_words_batch,
)

app = FastAPI(title="Voikko Finnish Analysis Service")


# --- Request models ---

class WordRequest(BaseModel):
    word: str

class SentenceRequest(BaseModel):
    sentence: str

class SuggestionsRequest(BaseModel):
    word: str
    max_suggestions: int = 5

class BatchRequest(BaseModel):
    words: list[str]


# --- Endpoints ---

@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/analyze")
def analyze(req: WordRequest):
    return analyze_word(req.word)


@app.post("/validate")
def validate(req: WordRequest):
    return validate_word(req.word)


@app.post("/validate-sentence")
def validate_sent(req: SentenceRequest):
    return validate_sentence(req.sentence)


@app.post("/vowel-harmony")
def vowel_harmony(req: WordRequest):
    return get_vowel_harmony(req.word)


@app.post("/suggestions")
def suggestions(req: SuggestionsRequest):
    return get_suggestions(req.word, req.max_suggestions)


@app.post("/validate-batch")
def validate_batch(req: BatchRequest):
    return validate_words_batch(req.words)
