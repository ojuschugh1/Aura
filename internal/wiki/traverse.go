package wiki

import (
	"fmt"
	"strings"

	"github.com/ojuschugh1/aura/pkg/types"
)

// TraceResult holds the shortest path between two wiki pages.
type TraceResult struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Hops  int      `json:"hops"`
	Path  []string `json:"path"`
	Found bool     `json:"found"`
}

// TracePath finds the shortest path between two pages using BFS over the
// bidirectional link graph. Unlike directed graph traversal, this treats
// all wiki links as two-way connections — if A links to B, you can walk
// from B back to A. This matches how knowledge actually flows.
func (e *Engine) TracePath(fromSlug, toSlug string) (*TraceResult, error) {
	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, err
	}

	// Build bidirectional adjacency list.
	adj := make(map[string][]string)
	slugExists := make(map[string]bool)
	for _, p := range pages {
		slugExists[p.Slug] = true
		for _, link := range p.LinksSlugs {
			adj[p.Slug] = append(adj[p.Slug], link)
			adj[link] = append(adj[link], p.Slug) // bidirectional
		}
	}

	if !slugExists[fromSlug] {
		return nil, fmt.Errorf("page %q not found", fromSlug)
	}
	if !slugExists[toSlug] {
		return nil, fmt.Errorf("page %q not found", toSlug)
	}

	// BFS.
	visited := map[string]bool{fromSlug: true}
	parent := map[string]string{}
	queue := []string{fromSlug}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == toSlug {
			// Reconstruct path.
			path := []string{toSlug}
			for path[len(path)-1] != fromSlug {
				path = append(path, parent[path[len(path)-1]])
			}
			// Reverse.
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return &TraceResult{
				From:  fromSlug,
				To:    toSlug,
				Hops:  len(path) - 1,
				Path:  path,
				Found: true,
			}, nil
		}

		for _, neighbor := range adj[current] {
			if !visited[neighbor] && slugExists[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	return &TraceResult{From: fromSlug, To: toSlug, Found: false}, nil
}

// NearbyResult holds pages within N hops of a given page.
type NearbyResult struct {
	Center string          `json:"center"`
	Depth  int             `json:"depth"`
	Pages  []NearbyEntry   `json:"pages"`
}

// NearbyEntry is a page found near the center, with its distance.
type NearbyEntry struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Distance int    `json:"distance"`
	Relation string `json:"relation"` // "links_to", "linked_from", or "mutual"
}

// Nearby returns all pages within `depth` hops of the given page.
// Default depth is 2 — enough to see the immediate neighborhood without
// pulling in the entire wiki.
func (e *Engine) Nearby(slug string, depth int) (*NearbyResult, error) {
	if depth <= 0 {
		depth = 2
	}

	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, err
	}

	// Build adjacency and page lookup.
	pageMap := make(map[string]*types.WikiPage)
	outbound := make(map[string]map[string]bool)
	inbound := make(map[string]map[string]bool)

	for _, p := range pages {
		pageMap[p.Slug] = p
		if outbound[p.Slug] == nil {
			outbound[p.Slug] = make(map[string]bool)
		}
		for _, link := range p.LinksSlugs {
			outbound[p.Slug][link] = true
			if inbound[link] == nil {
				inbound[link] = make(map[string]bool)
			}
			inbound[link][p.Slug] = true
		}
	}

	if pageMap[slug] == nil {
		return nil, fmt.Errorf("page %q not found", slug)
	}

	// BFS with distance tracking.
	type bfsNode struct {
		slug string
		dist int
	}
	visited := map[string]int{slug: 0}
	queue := []bfsNode{{slug, 0}}
	var entries []NearbyEntry

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.dist >= depth {
			continue
		}

		// Explore outbound links.
		for neighbor := range outbound[current.slug] {
			if _, seen := visited[neighbor]; !seen && pageMap[neighbor] != nil {
				visited[neighbor] = current.dist + 1
				queue = append(queue, bfsNode{neighbor, current.dist + 1})
			}
		}
		// Explore inbound links (bidirectional).
		for neighbor := range inbound[current.slug] {
			if _, seen := visited[neighbor]; !seen && pageMap[neighbor] != nil {
				visited[neighbor] = current.dist + 1
				queue = append(queue, bfsNode{neighbor, current.dist + 1})
			}
		}
	}

	for s, dist := range visited {
		if s == slug {
			continue
		}
		p := pageMap[s]
		if p == nil {
			continue
		}

		// Determine relationship direction.
		linksTo := outbound[slug][s]
		linkedFrom := inbound[slug][s]
		relation := "connected"
		if linksTo && linkedFrom {
			relation = "mutual"
		} else if linksTo {
			relation = "links_to"
		} else if linkedFrom {
			relation = "linked_from"
		}

		entries = append(entries, NearbyEntry{
			Slug:     s,
			Title:    p.Title,
			Category: p.Category,
			Distance: dist,
			Relation: relation,
		})
	}

	return &NearbyResult{
		Center: slug,
		Depth:  depth,
		Pages:  entries,
	}, nil
}

// ContextResult provides a full 360° view of a page — everything you need
// to understand its place in the wiki.
type ContextResult struct {
	Page       *types.WikiPage   `json:"page"`
	InboundLinks  []ContextLink  `json:"inbound_links"`
	OutboundLinks []ContextLink  `json:"outbound_links"`
	Sources       []ContextSource `json:"sources"`
	RecentLog     []*types.WikiLogEntry `json:"recent_activity"`
	Confidence    float64         `json:"confidence"`
	WordCount     int             `json:"word_count"`
	IsHub         bool            `json:"is_hub"`
}

// ContextLink is a linked page with its title and category.
type ContextLink struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Category string `json:"category"`
}

// ContextSource is a backing source for the page.
type ContextSource struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// Context returns a full 360° view of a page: what links to it, what it
// links to, which sources back it, recent activity, and whether it's a hub.
func (e *Engine) Context(slug string) (*ContextResult, error) {
	page, err := e.store.GetPage(slug)
	if err != nil {
		return nil, fmt.Errorf("page %q not found", slug)
	}

	pages, _ := e.store.ListPages("")
	pageMap := make(map[string]*types.WikiPage)
	for _, p := range pages {
		pageMap[p.Slug] = p
	}

	result := &ContextResult{
		Page:      page,
		WordCount: len(strings.Fields(page.Content)),
	}

	// Outbound links.
	for _, link := range page.LinksSlugs {
		if p := pageMap[link]; p != nil {
			result.OutboundLinks = append(result.OutboundLinks, ContextLink{
				Slug: p.Slug, Title: p.Title, Category: p.Category,
			})
		}
	}

	// Inbound links.
	for _, p := range pages {
		if p.Slug == slug {
			continue
		}
		for _, link := range p.LinksSlugs {
			if link == slug {
				result.InboundLinks = append(result.InboundLinks, ContextLink{
					Slug: p.Slug, Title: p.Title, Category: p.Category,
				})
				break
			}
		}
	}

	// Sources.
	for _, srcID := range page.SourceIDs {
		src, err := e.store.GetSource(srcID)
		if err == nil {
			result.Sources = append(result.Sources, ContextSource{
				ID: src.ID, Title: src.Title,
			})
		}
	}

	// Recent log entries mentioning this page.
	allLog, _ := e.store.RecentLog(100)
	for _, entry := range allLog {
		for _, s := range entry.PageSlugs {
			if s == slug {
				result.RecentLog = append(result.RecentLog, entry)
				break
			}
		}
		if len(result.RecentLog) >= 5 {
			break
		}
	}

	// Hub detection: a page is a hub if it has 5+ total connections.
	totalConnections := len(result.InboundLinks) + len(result.OutboundLinks)
	result.IsHub = totalConnections >= 5

	// Confidence based on source backing and freshness.
	result.Confidence = computeConfidence(page)

	return result, nil
}
