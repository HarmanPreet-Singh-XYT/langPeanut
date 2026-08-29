package agents

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/platforms"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// ContextAgent performs semantic disambiguation and contextual key naming
type ContextAgent struct{}

func NewContextAgent() *ContextAgent {
	return &ContextAgent{}
}

// DisambiguationMap handles common polysemous words
var domainKeywords = map[string]string{
	"flight":   "travel",
	"ticket":   "travel",
	"airline":  "travel",
	"hotel":    "travel",
	"room":     "travel",
	"cart":     "ecommerce",
	"checkout": "ecommerce",
	"price":    "ecommerce",
	"order":    "ecommerce",
	"product":  "ecommerce",
	"author":   "library",
	"chapter":  "library",
	"isbn":     "library",
	"page":     "library",
	"read":     "library",
}

// DisambiguateAndEnhance processes candidate strings and improves key naming and classification
func (ca *ContextAgent) DisambiguateAndEnhance(candidates []types.StringCandidate) ([]types.StringCandidate, error) {
	// 1. Group candidates by file to aggregate sibling strings
	byFile := make(map[string][]int)
	for i, c := range candidates {
		byFile[c.FilePath] = append(byFile[c.FilePath], i)
	}

	for filePath, indices := range byFile {
		var fileSiblings []string
		for _, idx := range indices {
			fileSiblings = append(fileSiblings, candidates[idx].CleanValue)
		}

		domain := ca.inferDomain(filePath, fileSiblings)

		for _, idx := range indices {
			c := &candidates[idx]
			c.SiblingStrings = fileSiblings

			// Check for ambiguous short words
			lower := strings.ToLower(c.CleanValue)
			switch lower {
			case "book":
				if domain == "travel" {
					c.Key = "reserveFlightBtn"
					c.Explanation = "Disambiguated 'Book' as verb (reserve ticket) in travel context"
					c.Confidence = 0.99
				} else if domain == "library" {
					c.Key = "readingBookTitle"
					c.Explanation = "Disambiguated 'Book' as noun (physical book) in library context"
					c.Confidence = 0.99
				} else {
					c.Key = "bookAction"
				}
			case "order":
				if domain == "ecommerce" {
					c.Key = "placeOrderBtn"
					c.Explanation = "Disambiguated 'Order' as commerce checkout action"
				}
			case "back":
				c.Key = "goBackBtn"
			case "save":
				c.Key = "saveChangesBtn"
			case "close":
				c.Key = "closeModalBtn"
			}

			// Ensure key uniqueness within the file/context
			componentName := getComponentNameFromFile(c.FilePath)
			if !strings.HasPrefix(strings.ToLower(c.Key), strings.ToLower(componentName)) && len(c.Key) < 15 {
				c.Key = platforms.ToCamelCase(fmt.Sprintf("%s %s", componentName, c.Key))
			}
		}
	}

	return candidates, nil
}

func (ca *ContextAgent) inferDomain(filePath string, siblings []string) string {
	lowerPath := strings.ToLower(filePath)
	for kw, domain := range domainKeywords {
		if strings.Contains(lowerPath, kw) {
			return domain
		}
	}

	// Check siblings
	for _, s := range siblings {
		lowerS := strings.ToLower(s)
		for kw, domain := range domainKeywords {
			if strings.Contains(lowerS, kw) {
				return domain
			}
		}
	}

	return "general"
}

func getComponentNameFromFile(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	// Remove common suffixes like .screen, .view, _modal
	name = strings.ReplaceAll(name, ".screen", "")
	name = strings.ReplaceAll(name, ".view", "")
	name = strings.ReplaceAll(name, "_screen", "")
	name = strings.ReplaceAll(name, "_view", "")
	name = strings.ReplaceAll(name, "_modal", "")
	return platforms.ToCamelCase(name)
}
