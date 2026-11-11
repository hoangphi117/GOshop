package dto

import (
	"goshop/services/catalog/internal/repository"
)

// FAQDTO is the public shape of a knowledge-base entry. The assistant reads
// question/answer verbatim, so both stay plain strings with no markdown.
type FAQDTO struct {
	UUID     string   `json:"faq_uuid"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Tags     []string `json:"tags,omitempty"`
}

func MapFAQsToDTOs(items []repository.FAQ) []FAQDTO {
	out := make([]FAQDTO, 0, len(items))
	for _, item := range items {
		out = append(out, FAQDTO{
			UUID:     item.UUID.String(),
			Question: item.Question,
			Answer:   item.Answer,
			Tags:     item.Tags,
		})
	}
	return out
}
