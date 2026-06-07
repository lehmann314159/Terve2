from fastapi import FastAPI
from pydantic import BaseModel
from .analysis import analyze_text

app = FastAPI()


class AnalyzeRequest(BaseModel):
    text: str


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/analyze")
def analyze(req: AnalyzeRequest):
    return analyze_text(req.text)
