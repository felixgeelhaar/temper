package docindex

import (
	"context"
	"sort"
)

// SearchResult represents a search result with similarity score
type SearchResult struct {
	SectionID  int64   `json:"section_id"`
	DocumentID string  `json:"document_id"`
	Heading    string  `json:"heading"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
}

// Retriever performs similarity search over indexed document sections
type Retriever struct {
	index    *Index
	embedder Embedder
}

// NewRetriever creates a new retriever
func NewRetriever(index *Index, embedder Embedder) *Retriever {
	return &Retriever{
		index:    index,
		embedder: embedder,
	}
}

// Search performs a top-K similarity search for the given query
func (r *Retriever) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	// Embed the query
	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Load all sections with embeddings
	sections, err := r.index.ListAllSectionsWithEmbeddings()
	if err != nil {
		return nil, err
	}

	// Compute similarity for each section
	var results []SearchResult
	for _, section := range sections {
		if section.Embedding == nil {
			continue
		}
		sectionVec := DecodeEmbedding(section.Embedding)
		if sectionVec == nil {
			continue
		}

		score := CosineSimilarity(queryVec, sectionVec)
		results = append(results, SearchResult{
			SectionID:  section.ID,
			DocumentID: section.DocumentID,
			Heading:    section.Heading,
			Content:    section.Content,
			Score:      score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// SearchWithThreshold returns results above a minimum similarity threshold
func (r *Retriever) SearchWithThreshold(ctx context.Context, query string, topK int, minScore float32) ([]SearchResult, error) {
	results, err := r.Search(ctx, query, 0) // Get all
	if err != nil {
		return nil, err
	}

	// Filter by threshold
	var filtered []SearchResult
	for _, res := range results {
		if res.Score >= minScore {
			filtered = append(filtered, res)
		}
	}

	if topK > 0 && len(filtered) > topK {
		filtered = filtered[:topK]
	}

	return filtered, nil
}
