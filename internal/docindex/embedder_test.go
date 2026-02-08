package docindex

import (
	"context"
	"math"
	"testing"
)

func TestKeywordEmbedder_Embed(t *testing.T) {
	embedder := NewKeywordEmbedder(128)

	vec, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(vec) != 128 {
		t.Errorf("Embed() vector length = %d; want 128", len(vec))
	}

	// Check that vector is normalized (L2 norm ≈ 1.0)
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	if diff := math.Abs(float64(norm) - 1.0); diff > 0.001 {
		t.Errorf("Embed() vector L2 norm = %f; want ≈1.0", norm)
	}
}

func TestKeywordEmbedder_EmptyText(t *testing.T) {
	embedder := NewKeywordEmbedder(64)

	vec, err := embedder.Embed(context.Background(), "")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(vec) != 64 {
		t.Errorf("Embed() vector length = %d; want 64", len(vec))
	}

	// All zeros for empty text
	for i, v := range vec {
		if v != 0 {
			t.Errorf("Embed() vec[%d] = %f; want 0 for empty text", i, v)
			break
		}
	}
}

func TestKeywordEmbedder_EmbedBatch(t *testing.T) {
	embedder := NewKeywordEmbedder(64)

	texts := []string{"hello world", "foo bar baz", "testing embeddings"}
	vecs, err := embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(vecs) != 3 {
		t.Fatalf("EmbedBatch() returned %d vectors; want 3", len(vecs))
	}

	for i, vec := range vecs {
		if len(vec) != 64 {
			t.Errorf("EmbedBatch() vec[%d] length = %d; want 64", i, len(vec))
		}
	}
}

func TestKeywordEmbedder_Dimension(t *testing.T) {
	embedder := NewKeywordEmbedder(256)
	if embedder.Dimension() != 256 {
		t.Errorf("Dimension() = %d; want 256", embedder.Dimension())
	}
}

func TestKeywordEmbedder_SimilarTexts(t *testing.T) {
	embedder := NewKeywordEmbedder(256)
	ctx := context.Background()

	v1, _ := embedder.Embed(ctx, "the quick brown fox jumps over the lazy dog")
	v2, _ := embedder.Embed(ctx, "the quick brown fox leaps over the lazy dog")
	v3, _ := embedder.Embed(ctx, "machine learning and artificial intelligence")

	sim12 := CosineSimilarity(v1, v2)
	sim13 := CosineSimilarity(v1, v3)

	// Similar texts should have higher similarity than dissimilar
	if sim12 <= sim13 {
		t.Errorf("Similar texts similarity (%f) should be > dissimilar (%f)", sim12, sim13)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float32
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{1, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{0, 1, 0},
			want: 0.0,
		},
		{
			name: "empty vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
		{
			name: "different lengths",
			a:    []float32{1, 0},
			b:    []float32{1, 0, 0},
			want: 0.0,
		},
		{
			name: "zero vector",
			a:    []float32{0, 0, 0},
			b:    []float32{1, 0, 0},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if diff := math.Abs(float64(got - tt.want)); diff > 0.001 {
				t.Errorf("CosineSimilarity() = %f; want %f", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeEmbedding(t *testing.T) {
	original := []float32{1.0, -2.5, 3.14, 0.0, -0.001}

	encoded := EncodeEmbedding(original)
	if len(encoded) != len(original)*4 {
		t.Fatalf("EncodeEmbedding() length = %d; want %d", len(encoded), len(original)*4)
	}

	decoded := DecodeEmbedding(encoded)
	if len(decoded) != len(original) {
		t.Fatalf("DecodeEmbedding() length = %d; want %d", len(decoded), len(original))
	}

	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("round-trip vec[%d] = %f; want %f", i, decoded[i], original[i])
		}
	}
}

func TestDecodeEmbedding_InvalidLength(t *testing.T) {
	// Not a multiple of 4
	result := DecodeEmbedding([]byte{1, 2, 3})
	if result != nil {
		t.Errorf("DecodeEmbedding() should return nil for invalid length; got %v", result)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"Hello World", []string{"hello", "world"}},
		{"foo-bar baz", []string{"foo", "bar", "baz"}},
		{"test123", []string{"test123"}},
		{"", nil},
		{"   ", nil},
	}

	for _, tt := range tests {
		got := tokenize(tt.text)
		if len(got) != len(tt.want) {
			t.Errorf("tokenize(%q) = %v; want %v", tt.text, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q; want %q", tt.text, i, got[i], tt.want[i])
			}
		}
	}
}
