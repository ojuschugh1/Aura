package wiki

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// FilterQuery represents a structured query over page metadata.
// Syntax: "field=value", "field!=value", "field>N", "field<N", "field contains value"
// Fields: category, slug, title, tags, source_count, link_count, created, updated
type FilterQuery struct {
	Field    string
	Operator string // "=", "!=", ">", "<", ">=", "<=", "contains"
	Value    string
}

// ParseFilter parses a filter string like "category=entity" or "tags contains api".
func ParseFilter(s string) (*FilterQuery, error) {
	s = strings.TrimSpace(s)

	// Try multi-word operators first.
	if idx := strings.Index(s, " contains "); idx > 0 {
		return &FilterQuery{
			Field:    strings.TrimSpace(s[:idx]),
			Operator: "contains",
			Value:    strings.TrimSpace(s[idx+10:]),
		}, nil
	}

	// Try comparison operators (>=, <=, !=, >, <, =).
	for _, op := range []string{">=", "<=", "!=", ">", "<", "="} {
		if idx := strings.Index(s, op); idx > 0 {
			return &FilterQuery{
				Field:    strings.TrimSpace(s[:idx]),
				Operator: op,
				Value:    strings.TrimSpace(s[idx+len(op):]),
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid filter %q: use field=value, field>N, or field contains value", s)
}

// ParseFilters parses multiple filter expressions joined by " AND ".
func ParseFilters(s string) ([]*FilterQuery, error) {
	parts := strings.Split(s, " AND ")
	var filters []*FilterQuery
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := ParseFilter(part)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

// FilterPages applies filters to a list of pages and returns matching ones.
func FilterPages(pages []*types.WikiPage, filters []*FilterQuery) ([]*types.WikiPage, error) {
	var result []*types.WikiPage
	for _, p := range pages {
		match := true
		for _, f := range filters {
			if !matchesFilter(p, f) {
				match = false
				break
			}
		}
		if match {
			result = append(result, p)
		}
	}
	return result, nil
}

func matchesFilter(p *types.WikiPage, f *FilterQuery) bool {
	switch strings.ToLower(f.Field) {
	case "category":
		return compareString(p.Category, f.Operator, f.Value)
	case "slug":
		return compareString(p.Slug, f.Operator, f.Value)
	case "title":
		return compareString(strings.ToLower(p.Title), f.Operator, strings.ToLower(f.Value))
	case "tags":
		return matchTags(p.Tags, f.Operator, f.Value)
	case "source_count":
		return compareInt(len(p.SourceIDs), f.Operator, f.Value)
	case "link_count":
		return compareInt(len(p.LinksSlugs), f.Operator, f.Value)
	case "created":
		return compareDate(p.CreatedAt, f.Operator, f.Value)
	case "updated":
		return compareDate(p.UpdatedAt, f.Operator, f.Value)
	default:
		return false
	}
}

func compareString(actual, op, expected string) bool {
	switch op {
	case "=":
		return strings.EqualFold(actual, expected)
	case "!=":
		return !strings.EqualFold(actual, expected)
	case "contains":
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
	default:
		return false
	}
}

func matchTags(tags []string, op, value string) bool {
	value = strings.ToLower(value)
	switch op {
	case "=", "contains":
		for _, t := range tags {
			if strings.EqualFold(t, value) {
				return true
			}
		}
		return false
	case "!=":
		for _, t := range tags {
			if strings.EqualFold(t, value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func compareInt(actual int, op, valueStr string) bool {
	expected, err := strconv.Atoi(valueStr)
	if err != nil {
		return false
	}
	switch op {
	case "=":
		return actual == expected
	case "!=":
		return actual != expected
	case ">":
		return actual > expected
	case "<":
		return actual < expected
	case ">=":
		return actual >= expected
	case "<=":
		return actual <= expected
	default:
		return false
	}
}

func compareDate(actual time.Time, op, valueStr string) bool {
	expected, err := time.Parse("2006-01-02", valueStr)
	if err != nil {
		return false
	}
	switch op {
	case "=":
		return actual.Format("2006-01-02") == expected.Format("2006-01-02")
	case "!=":
		return actual.Format("2006-01-02") != expected.Format("2006-01-02")
	case ">":
		return actual.After(expected)
	case "<":
		return actual.Before(expected)
	case ">=":
		return !actual.Before(expected)
	case "<=":
		return !actual.After(expected)
	default:
		return false
	}
}
