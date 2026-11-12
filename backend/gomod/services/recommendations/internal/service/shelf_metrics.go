package service

import (
	recsdto "goshop/services/recommendations/internal/dto"
)

// categoryCoverage is how many distinct categories the shelf spans.
//
// It is the metric that makes this bug visible in a log: a shelf of twelve
// "Picked for you" cards drawn from two categories is two distinct answers, not
// twelve. Zero for an empty shelf.
func categoryCoverage(items []recsdto.RecommendationItem) int {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := item.CategoryName
		if key == "" {
			key = item.CategoryUUID
		}
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

// brandCount is how many distinct brands the shelf spans, read the way the
// ranking grouped them.
func brandCount(items []recsdto.RecommendationItem) int {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[firstWord(item.Name)] = struct{}{}
	}
	return len(seen)
}

// intraListSimilarity approximates how alike a shelf's items are.
//
// Product vectors do not survive hydration (the card does not need them), so
// this uses the one consideration the ranking recorded: the cosine similarity
// each item scored against the visitor's profile. Two items that both score
// near the same profile point are near each other, so the shelf-average of the
// squared scores rises with a shelf of near-duplicates and falls with a mixed
// one. Exists to be watched over time, not compared across shelf kinds.
func intraListSimilarity(items []recsdto.RecommendationItem) float64 {
	var sum float64
	n := 0
	for _, item := range items {
		if item.Similarity <= 0 {
			continue
		}
		sum += item.Similarity * item.Similarity
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
