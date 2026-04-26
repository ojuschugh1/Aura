// Package wiki implements a persistent, LLM-maintained knowledge base.
//
// Inspired by Karpathy's "LLM Wiki" pattern, this package provides the
// infrastructure for building personal knowledge bases where the LLM
// incrementally builds and maintains a structured, interlinked collection
// of pages backed by SQLite.
//
// Architecture (three layers):
//
//   - Raw sources: immutable documents ingested into the wiki. The LLM reads
//     from them but never modifies them.
//   - Wiki pages: LLM-generated markdown pages — summaries, entity pages,
//     concept pages, comparisons, synthesis. The LLM owns this layer entirely.
//   - Index + Log: a catalog of all pages for fast navigation, and a
//     chronological record of all wiki operations.
//
// Core operations:
//
//   - Ingest: process a new source, extract key information, create/update pages.
//   - Query: search the wiki index, read relevant pages, synthesize answers.
//   - Lint: health-check for orphans, contradictions, stale pages, missing refs.
//
// All data is stored in the same SQLite database used by the rest of Aura,
// keeping the local-first, single-binary philosophy intact.
package wiki
