package wiki

import (
	"fmt"
	"sort"

	"github.com/ojuschugh1/aura/pkg/types"
)

// GraphStats provides connectivity analysis of the wiki — hub pages,
// clusters, density, and the overall shape of the knowledge graph.
type GraphStats struct {
	TotalPages  int            `json:"total_pages"`
	TotalEdges  int            `json:"total_edges"`
	Density     float64        `json:"density"`      // edges / (pages * (pages-1))
	AvgLinks    float64        `json:"avg_links"`    // average outbound links per page
	Hubs        []HubPage      `json:"hubs"`         // top pages by total connections
	Orphans     []string       `json:"orphans"`      // pages with zero connections
	Categories  map[string]int `json:"categories"`   // page count per category
	Clusters    []Cluster      `json:"clusters"`     // connected components
}

// HubPage is a page ranked by its total connections (inbound + outbound).
type HubPage struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Inbound    int    `json:"inbound"`
	Outbound   int    `json:"outbound"`
	Total      int    `json:"total"`
}

// Cluster is a connected component in the wiki graph.
type Cluster struct {
	ID    int      `json:"id"`
	Size  int      `json:"size"`
	Pages []string `json:"pages"`
}

// Graph computes connectivity statistics for the wiki.
func (e *Engine) Graph() (*GraphStats, error) {
	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, err
	}

	stats := &GraphStats{
		TotalPages: len(pages),
		Categories: make(map[string]int),
	}

	if len(pages) == 0 {
		return stats, nil
	}

	// Build adjacency data.
	slugToPage := make(map[string]*types.WikiPage)
	outbound := make(map[string]int)
	inbound := make(map[string]int)
	allSlugs := make(map[string]bool)

	for _, p := range pages {
		slugToPage[p.Slug] = p
		allSlugs[p.Slug] = true
		stats.Categories[p.Category]++
		outbound[p.Slug] = len(p.LinksSlugs)
		stats.TotalEdges += len(p.LinksSlugs)

		for _, link := range p.LinksSlugs {
			inbound[link]++
		}
	}

	// Density: edges / (n * (n-1)) for a directed graph.
	n := float64(len(pages))
	if n > 1 {
		stats.Density = float64(stats.TotalEdges) / (n * (n - 1))
	}

	// Average links.
	stats.AvgLinks = float64(stats.TotalEdges) / n

	// Hub ranking: sort by total connections (inbound + outbound).
	type pageRank struct {
		slug     string
		title    string
		in, out  int
	}
	var ranks []pageRank
	for _, p := range pages {
		ranks = append(ranks, pageRank{
			slug:  p.Slug,
			title: p.Title,
			in:    inbound[p.Slug],
			out:   outbound[p.Slug],
		})
	}
	sort.Slice(ranks, func(i, j int) bool {
		totalI := ranks[i].in + ranks[i].out
		totalJ := ranks[j].in + ranks[j].out
		return totalI > totalJ
	})

	// Top 10 hubs.
	limit := 10
	if len(ranks) < limit {
		limit = len(ranks)
	}
	for _, r := range ranks[:limit] {
		stats.Hubs = append(stats.Hubs, HubPage{
			Slug:     r.slug,
			Title:    r.title,
			Inbound:  r.in,
			Outbound: r.out,
			Total:    r.in + r.out,
		})
	}

	// Orphans: pages with zero inbound AND zero outbound.
	for _, p := range pages {
		if inbound[p.Slug] == 0 && outbound[p.Slug] == 0 {
			stats.Orphans = append(stats.Orphans, p.Slug)
		}
	}

	// Connected components via union-find.
	stats.Clusters = findClusters(pages, allSlugs)

	_ = e.store.AppendLog("graph",
		fmt.Sprintf("Graph: %d pages, %d edges, density=%.3f, %d clusters",
			stats.TotalPages, stats.TotalEdges, stats.Density, len(stats.Clusters)),
		nil, nil)

	return stats, nil
}

// findClusters computes connected components using union-find.
func findClusters(pages []*types.WikiPage, allSlugs map[string]bool) []Cluster {
	parent := make(map[string]string)

	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Initialize each page as its own component.
	for slug := range allSlugs {
		parent[slug] = slug
	}

	// Union pages that are linked.
	for _, p := range pages {
		for _, link := range p.LinksSlugs {
			if allSlugs[link] {
				union(p.Slug, link)
			}
		}
	}

	// Group by root.
	components := make(map[string][]string)
	for slug := range allSlugs {
		root := find(slug)
		components[root] = append(components[root], slug)
	}

	// Convert to Cluster slice, sorted by size descending.
	var clusters []Cluster
	id := 1
	for _, members := range components {
		sort.Strings(members)
		clusters = append(clusters, Cluster{
			ID:    id,
			Size:  len(members),
			Pages: members,
		})
		id++
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Size > clusters[j].Size
	})

	// Re-number after sorting.
	for i := range clusters {
		clusters[i].ID = i + 1
	}

	return clusters
}
