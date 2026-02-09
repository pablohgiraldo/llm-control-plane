package rag

import "context"

// Retriever fetches relevant context from a knowledge base.
type Retriever interface {
	Retrieve(ctx context.Context, query string, opts RetrievalOptions) ([]Document, error)
}

// RetrievalOptions configures retrieval behavior.
type RetrievalOptions struct {
	TopK      int
	Threshold float64
	Filters   map[string]string
}

// Document represents a retrieved knowledge base entry.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]interface{}
	Score    float64
}

// Embedder generates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}
