package docindex

import (
	"context"
	"encoding/binary"
	"math"
)

// Embedder produces vector embeddings from text
type Embedder interface {
	// Embed returns a float32 vector for the given text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch returns embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding vector dimension
	Dimension() int
}

// KeywordEmbedder is a simple TF-based embedder that does not require
// an external API. It uses term frequency vectors for basic similarity.
// This serves as the default when no LLM embedding provider is configured.
type KeywordEmbedder struct {
	dimension int
	vocab     map[string]int
}

// NewKeywordEmbedder creates a keyword-based embedder with a fixed vocabulary size
func NewKeywordEmbedder(dimension int) *KeywordEmbedder {
	return &KeywordEmbedder{
		dimension: dimension,
		vocab:     make(map[string]int),
	}
}

// Dimension returns the embedding vector dimension
func (e *KeywordEmbedder) Dimension() int {
	return e.dimension
}

// Embed produces a hash-based embedding for keyword matching
func (e *KeywordEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return e.embedText(text), nil
}

// EmbedBatch embeds multiple texts
func (e *KeywordEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = e.embedText(text)
	}
	return result, nil
}

func (e *KeywordEmbedder) embedText(text string) []float32 {
	vec := make([]float32, e.dimension)
	words := tokenize(text)
	if len(words) == 0 {
		return vec
	}

	// Hash each word to a bucket and accumulate
	for _, word := range words {
		bucket := hashString(word) % uint32(e.dimension)
		vec[bucket] += 1.0
	}

	// L2 normalize
	normalize(vec)
	return vec
}

// CosineSimilarity computes cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := float32(math.Sqrt(float64(normA)) * math.Sqrt(float64(normB)))
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// EncodeEmbedding serializes a float32 vector to bytes for SQLite BLOB storage
func EncodeEmbedding(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// DecodeEmbedding deserializes bytes to a float32 vector
func DecodeEmbedding(data []byte) []float32 {
	if len(data)%4 != 0 {
		return nil
	}
	vec := make([]float32, len(data)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vec
}

// normalize L2-normalizes a vector in place
func normalize(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(float64(sum)))
	for i := range vec {
		vec[i] /= norm
	}
}

// tokenize splits text into lowercase word tokens
func tokenize(text string) []string {
	var words []string
	word := make([]byte, 0, 32)
	for i := 0; i < len(text); i++ {
		c := text[i]
		if isAlphaNum(c) {
			if c >= 'A' && c <= 'Z' {
				c += 32 // lowercase
			}
			word = append(word, c)
		} else if len(word) > 0 {
			words = append(words, string(word))
			word = word[:0]
		}
	}
	if len(word) > 0 {
		words = append(words, string(word))
	}
	return words
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// hashString produces a simple hash of a string
func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}
