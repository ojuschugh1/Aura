package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// VizResult holds the output of a visualization generation.
type VizResult struct {
	FilePath   string `json:"file_path"`
	TotalNodes int    `json:"total_nodes"`
	TotalEdges int    `json:"total_edges"`
}

// Visualize generates an interactive HTML file showing the wiki as a
// force-directed network. Nodes are colored by category, sized by
// connection count, and clickable to reveal content. Built with vanilla
// HTML5 Canvas — no external dependencies, no build step, works offline.
func (e *Engine) Visualize(outPath string) (*VizResult, error) {
	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, err
	}

	// Build node and edge data.
	type vizNode struct {
		ID         string  `json:"id"`
		Label      string  `json:"label"`
		Category   string  `json:"category"`
		Size       int     `json:"size"`
		Confidence float64 `json:"confidence"`
		WordCount  int     `json:"word_count"`
		Excerpt    string  `json:"excerpt"`
	}
	type vizEdge struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	slugSet := make(map[string]bool)
	for _, p := range pages {
		slugSet[p.Slug] = true
	}

	// Count inbound links for sizing.
	inboundCount := make(map[string]int)
	for _, p := range pages {
		for _, link := range p.LinksSlugs {
			inboundCount[link]++
		}
	}

	var nodes []vizNode
	var edges []vizEdge
	totalEdges := 0

	for _, p := range pages {
		totalConns := len(p.LinksSlugs) + inboundCount[p.Slug]
		size := 8 + totalConns*2
		if size > 40 {
			size = 40
		}

		excerpt := p.Content
		if len(excerpt) > 150 {
			excerpt = excerpt[:150] + "…"
		}
		excerpt = strings.ReplaceAll(excerpt, "\n", " ")
		excerpt = strings.ReplaceAll(excerpt, "\"", "'")

		nodes = append(nodes, vizNode{
			ID:         p.Slug,
			Label:      p.Title,
			Category:   p.Category,
			Size:       size,
			Confidence: computeConfidence(p),
			WordCount:  len(strings.Fields(p.Content)),
			Excerpt:    excerpt,
		})

		for _, link := range p.LinksSlugs {
			if slugSet[link] {
				edges = append(edges, vizEdge{From: p.Slug, To: link})
				totalEdges++
			}
		}
	}

	nodesJSON, _ := json.Marshal(nodes)
	edgesJSON, _ := json.Marshal(edges)

	html := generateVizHTML(string(nodesJSON), string(edgesJSON), len(nodes), totalEdges)

	if err := os.WriteFile(outPath, []byte(html), 0o644); err != nil {
		return nil, fmt.Errorf("write viz: %w", err)
	}

	_ = e.store.AppendLog("visualize",
		fmt.Sprintf("Generated visualization: %d nodes, %d edges → %s", len(nodes), totalEdges, outPath),
		nil, nil)

	return &VizResult{
		FilePath:   outPath,
		TotalNodes: len(nodes),
		TotalEdges: totalEdges,
	}, nil
}

func generateVizHTML(nodesJSON, edgesJSON string, nodeCount, edgeCount int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Aura Wiki — Knowledge Map</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0d1117; color: #c9d1d9; overflow: hidden; }
#canvas { display: block; cursor: grab; }
#canvas:active { cursor: grabbing; }
#controls { position: fixed; top: 16px; left: 16px; z-index: 10; display: flex; flex-direction: column; gap: 8px; }
#search { padding: 8px 12px; border-radius: 6px; border: 1px solid #30363d; background: #161b22; color: #c9d1d9; font-size: 14px; width: 260px; outline: none; }
#search:focus { border-color: #58a6ff; }
#stats { font-size: 12px; color: #8b949e; padding: 4px 0; }
#legend { display: flex; gap: 12px; flex-wrap: wrap; }
.legend-item { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #8b949e; cursor: pointer; }
.legend-dot { width: 10px; height: 10px; border-radius: 50%%; }
#tooltip { position: fixed; display: none; background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 12px 16px; max-width: 350px; font-size: 13px; line-height: 1.5; z-index: 20; pointer-events: none; box-shadow: 0 4px 12px rgba(0,0,0,0.4); }
#tooltip h3 { color: #f0f6fc; margin-bottom: 4px; font-size: 14px; }
#tooltip .meta { color: #8b949e; font-size: 11px; margin-bottom: 6px; }
#tooltip .excerpt { color: #c9d1d9; }
.filter-active { opacity: 0.4; }
</style>
</head>
<body>
<canvas id="canvas"></canvas>
<div id="controls">
  <input id="search" type="text" placeholder="Search pages…" autocomplete="off">
  <div id="stats">%d pages · %d connections</div>
  <div id="legend"></div>
</div>
<div id="tooltip"></div>
<script>
const NODES = %s;
const EDGES = %s;

const COLORS = {
  entity: '#58a6ff', concept: '#a371f7', source: '#3fb950',
  synthesis: '#d29922', tool: '#f78166', index: '#8b949e', log: '#8b949e'
};

const canvas = document.getElementById('canvas');
const ctx = canvas.getContext('2d');
const tooltip = document.getElementById('tooltip');
const searchInput = document.getElementById('search');
const legend = document.getElementById('legend');

let W, H, dpr;
let offsetX = 0, offsetY = 0, scale = 1;
let dragging = false, dragStartX, dragStartY;
let hoveredNode = null;
let activeFilter = null;

// Layout: force-directed simulation.
const nodes = NODES.map((n, i) => ({
  ...n, x: Math.cos(i * 2.399) * 200 + Math.random() * 50,
  y: Math.sin(i * 2.399) * 200 + Math.random() * 50, vx: 0, vy: 0
}));
const nodeMap = {};
nodes.forEach(n => nodeMap[n.id] = n);
const edges = EDGES.filter(e => nodeMap[e.from] && nodeMap[e.to]);

function resize() {
  dpr = window.devicePixelRatio || 1;
  W = window.innerWidth; H = window.innerHeight;
  canvas.width = W * dpr; canvas.height = H * dpr;
  canvas.style.width = W + 'px'; canvas.style.height = H + 'px';
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  offsetX = W / 2; offsetY = H / 2;
}
resize();
window.addEventListener('resize', resize);

// Build legend.
const cats = [...new Set(nodes.map(n => n.category))].sort();
cats.forEach(cat => {
  const item = document.createElement('div');
  item.className = 'legend-item';
  item.innerHTML = '<div class="legend-dot" style="background:' + (COLORS[cat]||'#8b949e') + '"></div>' + cat;
  item.onclick = () => {
    if (activeFilter === cat) { activeFilter = null; item.classList.remove('filter-active'); }
    else { activeFilter = cat; document.querySelectorAll('.legend-item').forEach(i => i.classList.add('filter-active')); item.classList.remove('filter-active'); }
  };
  legend.appendChild(item);
});

// Physics.
function tick() {
  const repulsion = 800, attraction = 0.005, damping = 0.85, center = 0.01;
  for (let i = 0; i < nodes.length; i++) {
    const a = nodes[i];
    a.vx -= a.x * center; a.vy -= a.y * center;
    for (let j = i + 1; j < nodes.length; j++) {
      const b = nodes[j];
      let dx = a.x - b.x, dy = a.y - b.y;
      let d2 = dx * dx + dy * dy || 1;
      let f = repulsion / d2;
      a.vx += dx * f; a.vy += dy * f;
      b.vx -= dx * f; b.vy -= dy * f;
    }
  }
  for (const e of edges) {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    if (!a || !b) continue;
    let dx = b.x - a.x, dy = b.y - a.y;
    a.vx += dx * attraction; a.vy += dy * attraction;
    b.vx -= dx * attraction; b.vy -= dy * attraction;
  }
  for (const n of nodes) {
    n.vx *= damping; n.vy *= damping;
    n.x += n.vx; n.y += n.vy;
  }
}

function draw() {
  ctx.clearRect(0, 0, W, H);
  ctx.save();
  ctx.translate(offsetX, offsetY);
  ctx.scale(scale, scale);

  const searchTerm = searchInput.value.toLowerCase();

  // Edges.
  ctx.lineWidth = 0.5;
  for (const e of edges) {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    if (!a || !b) continue;
    let alpha = 0.15;
    if (hoveredNode && (e.from === hoveredNode.id || e.to === hoveredNode.id)) alpha = 0.6;
    if (activeFilter && a.category !== activeFilter && b.category !== activeFilter) alpha = 0.03;
    ctx.strokeStyle = 'rgba(139,148,158,' + alpha + ')';
    ctx.beginPath(); ctx.moveTo(a.x, a.y); ctx.lineTo(b.x, b.y); ctx.stroke();
  }

  // Nodes.
  for (const n of nodes) {
    let alpha = 1;
    if (activeFilter && n.category !== activeFilter) alpha = 0.15;
    if (searchTerm && !n.label.toLowerCase().includes(searchTerm) && !n.id.includes(searchTerm)) alpha = 0.1;

    const color = COLORS[n.category] || '#8b949e';
    const r = n.size / 2;

    ctx.globalAlpha = alpha;
    ctx.fillStyle = color;
    ctx.beginPath(); ctx.arc(n.x, n.y, r, 0, Math.PI * 2); ctx.fill();

    if (n === hoveredNode) {
      ctx.strokeStyle = '#f0f6fc'; ctx.lineWidth = 2;
      ctx.beginPath(); ctx.arc(n.x, n.y, r + 2, 0, Math.PI * 2); ctx.stroke();
    }

    // Label for larger nodes.
    if (r >= 6 && alpha > 0.3) {
      ctx.fillStyle = 'rgba(240,246,252,' + alpha + ')';
      ctx.font = '10px -apple-system, sans-serif';
      ctx.textAlign = 'center';
      const label = n.label.length > 20 ? n.label.slice(0, 18) + '…' : n.label;
      ctx.fillText(label, n.x, n.y + r + 12);
    }
    ctx.globalAlpha = 1;
  }

  ctx.restore();
  tick();
  requestAnimationFrame(draw);
}

// Interaction.
canvas.addEventListener('mousedown', e => { dragging = true; dragStartX = e.clientX - offsetX; dragStartY = e.clientY - offsetY; });
canvas.addEventListener('mousemove', e => {
  if (dragging) { offsetX = e.clientX - dragStartX; offsetY = e.clientY - dragStartY; return; }
  const mx = (e.clientX - offsetX) / scale, my = (e.clientY - offsetY) / scale;
  hoveredNode = null;
  for (const n of nodes) {
    const dx = n.x - mx, dy = n.y - my, r = n.size / 2 + 4;
    if (dx * dx + dy * dy < r * r) { hoveredNode = n; break; }
  }
  if (hoveredNode) {
    const n = hoveredNode;
    const conf = (n.confidence * 100).toFixed(0);
    tooltip.innerHTML = '<h3>' + n.label + '</h3><div class="meta">' + n.category + ' · ' + n.word_count + ' words · ' + conf + '%%</div><div class="excerpt">' + n.excerpt + '</div>';
    tooltip.style.display = 'block';
    tooltip.style.left = (e.clientX + 16) + 'px';
    tooltip.style.top = (e.clientY + 16) + 'px';
    canvas.style.cursor = 'pointer';
  } else {
    tooltip.style.display = 'none';
    canvas.style.cursor = 'grab';
  }
});
canvas.addEventListener('mouseup', () => dragging = false);
canvas.addEventListener('wheel', e => {
  e.preventDefault();
  const factor = e.deltaY > 0 ? 0.9 : 1.1;
  scale *= factor;
  scale = Math.max(0.1, Math.min(5, scale));
}, { passive: false });

searchInput.addEventListener('input', () => { /* redraw handles it */ });

draw();
</script>
</body>
</html>`, nodeCount, edgeCount, nodesJSON, edgesJSON)
}
