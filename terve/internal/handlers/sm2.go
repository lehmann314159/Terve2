package handlers

import (
	"math"
	"time"
)

// ReviewResult holds the updated SM-2 fields after a review.
type ReviewResult struct {
	EaseFactor   float64
	IntervalDays int
	Repetitions  int
	NextReview   time.Time
}

// SM2 computes the next review state using the SM-2 algorithm.
// quality: 1=Again, 3=Hard, 4=Good, 5=Easy
func SM2(easeFactor float64, intervalDays, repetitions, quality int) ReviewResult {
	now := time.Now()

	if quality < 3 {
		// Failed — reset
		return ReviewResult{
			EaseFactor:   easeFactor,
			IntervalDays: 1,
			Repetitions:  0,
			NextReview:   now.AddDate(0, 0, 1),
		}
	}

	// Update ease factor: EF' = EF + (0.1 - (5-q) * (0.08 + (5-q)*0.02))
	q := float64(quality)
	ef := easeFactor + (0.1 - (5-q)*(0.08+(5-q)*0.02))
	ef = math.Max(ef, 1.3)

	var interval int
	reps := repetitions + 1
	switch {
	case reps == 1:
		interval = 1
	case reps == 2:
		interval = 6
	default:
		interval = int(math.Round(float64(intervalDays) * ef))
	}

	return ReviewResult{
		EaseFactor:   ef,
		IntervalDays: interval,
		Repetitions:  reps,
		NextReview:   now.AddDate(0, 0, interval),
	}
}
