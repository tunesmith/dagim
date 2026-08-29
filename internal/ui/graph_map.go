// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tunesmith/dagim/internal/graph"
)

const (
	graphMapNodeWidth         = 24
	graphMapNodeHeight        = 4
	graphMapColumnGap         = 8
	graphMapHorizontalContext = graphMapColumnGap / 2
	graphMapRowStep           = 6
	graphMapEdgeArrow         = '▶'
)

type graphMapNode struct {
	node      graph.Node
	ready     bool
	fileIndex int
	parents   []graph.NodeID
	children  []graph.NodeID
}

type graphMapEdge struct {
	parent graph.NodeID
	child  graph.NodeID
}

// graphMapProjection contains graph meaning but no terminal geometry, keeping
// visibility, frontier, and stable-order decisions independent of rendering.
type graphMapProjection struct {
	nodes             []graphMapNode
	byID              map[graph.NodeID]*graphMapNode
	edges             []graphMapEdge
	ready             []graph.NodeID
	hiddenTransitive  int
	showingTransitive bool
}

func newGraphMapProjection(g *graph.Graph, showCompleted bool) graphMapProjection {
	return newGraphMapProjectionWithTransitive(g, showCompleted, false)
}

func newGraphMapProjectionWithTransitive(g *graph.Graph, showCompleted, showTransitive bool) graphMapProjection {
	p := graphMapProjection{
		byID:              make(map[graph.NodeID]*graphMapNode),
		showingTransitive: showTransitive,
	}
	ready := make(map[graph.NodeID]bool)
	for _, id := range g.Ready() {
		ready[id] = true
	}
	for i, node := range g.Nodes() {
		if node.Complete && !showCompleted {
			continue
		}
		p.nodes = append(p.nodes, graphMapNode{
			node:      node,
			ready:     ready[node.ID],
			fileIndex: i,
		})
	}
	for i := range p.nodes {
		p.byID[p.nodes[i].node.ID] = &p.nodes[i]
		if p.nodes[i].ready {
			p.ready = append(p.ready, p.nodes[i].node.ID)
		}
	}
	transitive := make(map[graphMapEdge]bool)
	for _, edge := range g.TransitiveEdges() {
		visiblePath := true
		for _, id := range edge.Path {
			if p.byID[id] == nil {
				visiblePath = false
				break
			}
		}
		if visiblePath {
			transitive[graphMapEdge{parent: edge.Parent, child: edge.Child}] = true
		}
	}
	for _, edge := range g.Edges() {
		parent := p.byID[edge.Parent]
		child := p.byID[edge.Child]
		if parent == nil || child == nil {
			continue
		}
		projected := graphMapEdge{parent: edge.Parent, child: edge.Child}
		if transitive[projected] && !showTransitive {
			p.hiddenTransitive++
			continue
		}
		parent.children = append(parent.children, edge.Child)
		child.parents = append(child.parents, edge.Parent)
		p.edges = append(p.edges, projected)
	}
	return p
}

type graphMapVertex struct {
	key      string
	id       graph.NodeID
	route    graph.NodeID
	target   graph.NodeID
	real     bool
	rank     int
	seed     float64
	x        int
	y        int
	incoming []*graphMapVertex
	outgoing []*graphMapVertex
}

type graphMapSegment struct {
	from   *graphMapVertex
	to     *graphMapVertex
	route  graph.NodeID
	target graph.NodeID
	lane   int
}

type graphMapPlacedNode struct {
	id   graph.NodeID
	rank int
	x    int
	y    int
}

type graphMapTraversal struct {
	from    graph.NodeID
	to      graph.NodeID
	forward bool
}

type graphMapLayout struct {
	projection graphMapProjection
	vertices   []*graphMapVertex
	segments   []graphMapSegment
	nodes      map[graph.NodeID]graphMapPlacedNode
	layers     map[int][]*graphMapVertex
	ranks      []int
	minRank    int
	maxRank    int
	width      int
	height     int
}

func layoutGraphMap(p graphMapProjection) graphMapLayout {
	l := graphMapLayout{
		projection: p,
		nodes:      make(map[graph.NodeID]graphMapPlacedNode),
		layers:     make(map[int][]*graphMapVertex),
	}
	if len(p.nodes) == 0 {
		return l
	}

	ranks := graphMapRanks(p)
	l.minRank, l.maxRank = ranks[p.nodes[0].node.ID], ranks[p.nodes[0].node.ID]
	real := make(map[graph.NodeID]*graphMapVertex, len(p.nodes))
	for _, node := range p.nodes {
		rank := ranks[node.node.ID]
		if rank < l.minRank {
			l.minRank = rank
		}
		if rank > l.maxRank {
			l.maxRank = rank
		}
		v := &graphMapVertex{
			key:  "node:" + string(node.node.ID),
			id:   node.node.ID,
			real: true,
			rank: rank,
			seed: float64(node.fileIndex),
		}
		real[node.node.ID] = v
		l.vertices = append(l.vertices, v)
		l.layers[rank] = append(l.layers[rank], v)
	}

	for edgeIndex, edge := range p.edges {
		from, to := real[edge.parent], real[edge.child]
		if from == nil || to == nil || from.rank >= to.rank {
			continue
		}
		previous := from
		span := to.rank - from.rank
		fromIndex := p.byID[edge.parent].fileIndex
		toIndex := p.byID[edge.child].fileIndex
		for rank := from.rank + 1; rank < to.rank; rank++ {
			fraction := float64(rank-from.rank) / float64(span)
			dummy := &graphMapVertex{
				key:    fmt.Sprintf("edge:%d:%d", edgeIndex, rank),
				route:  edge.parent,
				target: edge.child,
				rank:   rank,
				seed:   float64(fromIndex) + fraction*float64(toIndex-fromIndex),
			}
			l.vertices = append(l.vertices, dummy)
			l.layers[rank] = append(l.layers[rank], dummy)
			linkGraphMapVertices(&l, previous, dummy, edge.parent, edge.child)
			previous = dummy
		}
		linkGraphMapVertices(&l, previous, to, edge.parent, edge.child)
	}

	for rank := l.minRank; rank <= l.maxRank; rank++ {
		l.ranks = append(l.ranks, rank)
		layer := l.layers[rank]
		sort.SliceStable(layer, func(i, j int) bool {
			if layer[i].seed == layer[j].seed {
				return layer[i].key < layer[j].key
			}
			return layer[i].seed < layer[j].seed
		})
	}
	l.reduceCrossings()

	maxRows := 0
	for _, rank := range l.ranks {
		layer := l.layers[rank]
		if len(layer) > maxRows {
			maxRows = len(layer)
		}
		x := (rank - l.minRank) * (graphMapNodeWidth + graphMapColumnGap)
		for row, vertex := range layer {
			vertex.x = x
			vertex.y = row*graphMapRowStep + 2
			if vertex.real {
				placed := graphMapPlacedNode{id: vertex.id, rank: rank, x: x, y: row * graphMapRowStep}
				l.nodes[vertex.id] = placed
			}
		}
	}
	l.assignRoutingLanes()
	l.width = (l.maxRank-l.minRank)*(graphMapNodeWidth+graphMapColumnGap) + graphMapNodeWidth
	l.height = maxRows * graphMapRowStep
	if l.height > 0 {
		l.height -= graphMapRowStep - graphMapNodeHeight
	}
	return l
}

func linkGraphMapVertices(l *graphMapLayout, from, to *graphMapVertex, route, target graph.NodeID) {
	from.outgoing = append(from.outgoing, to)
	to.incoming = append(to.incoming, from)
	l.segments = append(l.segments, graphMapSegment{from: from, to: to, route: route, target: target})
}

func (l *graphMapLayout) assignRoutingLanes() {
	byGap := make(map[int][]*graphMapSegment)
	for i := range l.segments {
		segment := &l.segments[i]
		byGap[segment.from.rank] = append(byGap[segment.from.rank], segment)
	}
	for _, segments := range byGap {
		routeSet := make(map[graph.NodeID]bool)
		var routes []graph.NodeID
		for _, segment := range segments {
			if routeSet[segment.route] {
				continue
			}
			routeSet[segment.route] = true
			routes = append(routes, segment.route)
		}
		sort.SliceStable(routes, func(i, j int) bool {
			return l.projection.byID[routes[i]].fileIndex < l.projection.byID[routes[j]].fileIndex
		})
		lanes := make(map[graph.NodeID]int, len(routes))
		for i, route := range routes {
			lanes[route] = graphMapLane(i, len(routes))
		}
		for _, segment := range segments {
			segment.lane = lanes[segment.route]
		}
	}
}

func graphMapLane(index, total int) int {
	const first = 1
	last := graphMapColumnGap - 2
	if total <= 1 {
		return graphMapColumnGap / 2
	}
	if index >= total {
		index = total - 1
	}
	if total > last-first+1 {
		return first + index%(last-first+1)
	}
	return first + index*(last-first)/(total-1)
}

func graphMapRanks(p graphMapProjection) map[graph.NodeID]int {
	ranks := make(map[graph.NodeID]int, len(p.nodes))
	visiting := make(map[graph.NodeID]bool)
	incomplete := 0
	for _, node := range p.nodes {
		if !node.node.Complete {
			incomplete++
		}
	}

	var forward func(graph.NodeID) int
	forward = func(id graph.NodeID) int {
		if rank, ok := ranks[id]; ok {
			return rank
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		rank := 0
		for _, parentID := range p.byID[id].parents {
			parent := p.byID[parentID]
			if parent == nil || (incomplete > 0 && parent.node.Complete) {
				continue
			}
			if candidate := forward(parentID) + 1; candidate > rank {
				rank = candidate
			}
		}
		delete(visiting, id)
		ranks[id] = rank
		return rank
	}

	if incomplete == 0 {
		for _, node := range p.nodes {
			forward(node.node.ID)
		}
		return ranks
	}
	for _, node := range p.nodes {
		if !node.node.Complete {
			forward(node.node.ID)
		}
	}

	var backward func(graph.NodeID) int
	backward = func(id graph.NodeID) int {
		if rank, ok := ranks[id]; ok {
			return rank
		}
		if visiting[id] {
			return -1
		}
		visiting[id] = true
		rank := -1
		for _, childID := range p.byID[id].children {
			child := p.byID[childID]
			if child == nil {
				continue
			}
			candidate := 0
			if child.node.Complete {
				candidate = backward(childID) - 1
			} else {
				candidate = ranks[childID] - 1
			}
			if candidate < rank {
				rank = candidate
			}
		}
		delete(visiting, id)
		ranks[id] = rank
		return rank
	}
	for _, node := range p.nodes {
		if node.node.Complete {
			backward(node.node.ID)
		}
	}
	return ranks
}

func (l *graphMapLayout) reduceCrossings() {
	for pass := 0; pass < 4; pass++ {
		positions := l.vertexPositions()
		for _, rank := range l.ranks {
			if rank == 0 {
				continue // keep the ready/frontier order identical to file order
			}
			stableBarycenterSort(l.layers[rank], positions, true)
			positions = l.vertexPositions()
		}
		for i := len(l.ranks) - 1; i >= 0; i-- {
			rank := l.ranks[i]
			if rank == 0 {
				continue
			}
			stableBarycenterSort(l.layers[rank], positions, false)
			positions = l.vertexPositions()
		}
	}
}

func (l graphMapLayout) vertexPositions() map[*graphMapVertex]float64 {
	positions := make(map[*graphMapVertex]float64, len(l.vertices))
	for _, layer := range l.layers {
		for i, vertex := range layer {
			positions[vertex] = float64(i)
		}
	}
	return positions
}

func stableBarycenterSort(layer []*graphMapVertex, positions map[*graphMapVertex]float64, useIncoming bool) {
	type weighted struct {
		vertex *graphMapVertex
		value  float64
		has    bool
	}
	items := make([]weighted, len(layer))
	for i, vertex := range layer {
		neighbors := vertex.outgoing
		if useIncoming {
			neighbors = vertex.incoming
		}
		items[i].vertex = vertex
		for _, neighbor := range neighbors {
			items[i].value += positions[neighbor]
		}
		if len(neighbors) > 0 {
			items[i].value /= float64(len(neighbors))
			items[i].has = true
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].has != items[j].has {
			return items[i].has
		}
		if !items[i].has || items[i].value == items[j].value {
			return false
		}
		return items[i].value < items[j].value
	})
	for i := range items {
		layer[i] = items[i].vertex
	}
}

type graphMapCellRole uint8

const (
	graphMapPlain graphMapCellRole = iota
	graphMapEdgeRole
	graphMapHighlightedEdgeRole
	graphMapBlockedRole
	graphMapReadyRole
	graphMapCompleteRole
	graphMapSelectedRole
)

type graphMapCell struct {
	r                 rune
	role              graphMapCellRole
	edge              graphMapEdgeDirections
	highlightedEdge   graphMapEdgeDirections
	highlightedSymbol bool
	ordinarySymbol    bool
}

type graphMapEdgeDirections uint8

const (
	graphMapNorth graphMapEdgeDirections = 1 << iota
	graphMapEast
	graphMapSouth
	graphMapWest
)

type graphMapCanvas struct {
	width  int
	height int
	cells  [][]graphMapCell
}

func newGraphMapCanvas(width, height int) graphMapCanvas {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	c := graphMapCanvas{width: width, height: height, cells: make([][]graphMapCell, height)}
	for y := range c.cells {
		c.cells[y] = make([]graphMapCell, width)
		for x := range c.cells[y] {
			c.cells[y][x].r = ' '
		}
	}
	return c
}

func (l graphMapLayout) canvas(selected graph.NodeID) graphMapCanvas {
	c := newGraphMapCanvas(l.width, l.height)
	for _, segment := range l.segments {
		c.drawSegment(segment, segment.route == selected || segment.target == selected)
	}
	for _, vertex := range l.vertices {
		if vertex.real {
			continue
		}
		role := graphMapEdgeRole
		if vertex.route == selected || vertex.target == selected {
			role = graphMapHighlightedEdgeRole
		}
		c.drawHorizontal(vertex.x, vertex.x+graphMapNodeWidth-1, vertex.y, role)
	}
	for _, projected := range l.projection.nodes {
		placed, ok := l.nodes[projected.node.ID]
		if !ok {
			continue
		}
		role := graphMapBlockedRole
		switch {
		case projected.node.ID == selected:
			role = graphMapSelectedRole
		case projected.node.Complete:
			role = graphMapCompleteRole
		case projected.ready:
			role = graphMapReadyRole
		}
		for row, line := range graphMapCardLines(projected, projected.node.ID == selected) {
			c.drawText(placed.x, placed.y+row, line, role)
		}
	}
	return c
}

func (c graphMapCanvas) drawSegment(segment graphMapSegment, highlighted bool) {
	role := graphMapEdgeRole
	if highlighted {
		role = graphMapHighlightedEdgeRole
	}
	fromX := segment.from.x + graphMapNodeWidth
	toX := segment.to.x
	if segment.to.real {
		toX--
	}
	midX := fromX + segment.lane
	c.drawHorizontal(fromX, midX, segment.from.y, role)
	c.drawVertical(midX, segment.from.y, segment.to.y, role)
	c.drawHorizontal(midX, toX, segment.to.y, role)
	if segment.to.real {
		c.setEdgeSymbol(toX, segment.to.y, graphMapEdgeArrow, role)
	}
}

func (c graphMapCanvas) drawHorizontal(x1, x2, y int, role graphMapCellRole) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		var directions graphMapEdgeDirections
		if x > x1 {
			directions |= graphMapWest
		}
		if x < x2 {
			directions |= graphMapEast
		}
		c.connectEdge(x, y, directions, role)
	}
}

func (c graphMapCanvas) drawVertical(x, y1, y2 int, role graphMapCellRole) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if y1 == y2 {
		return
	}
	for y := y1; y <= y2; y++ {
		var directions graphMapEdgeDirections
		if y > y1 {
			directions |= graphMapNorth
		}
		if y < y2 {
			directions |= graphMapSouth
		}
		c.connectEdge(x, y, directions, role)
	}
}

func (c graphMapCanvas) connectEdge(x, y int, directions graphMapEdgeDirections, role graphMapCellRole) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	if directions == 0 {
		return
	}
	cell := &c.cells[y][x]
	cell.edge |= directions
	cell.r = graphMapEdgeRune(cell.edge)
	if role == graphMapHighlightedEdgeRole {
		cell.highlightedEdge |= directions
	}
	cell.updateEdgeRole()
}

func (c graphMapCanvas) setEdgeSymbol(x, y int, r rune, role graphMapCellRole) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	cell := &c.cells[y][x]
	cell.r = r
	if role == graphMapHighlightedEdgeRole {
		cell.highlightedSymbol = true
	} else {
		cell.ordinarySymbol = true
	}
	cell.updateEdgeRole()
}

func (c *graphMapCell) updateEdgeRole() {
	if c.highlightedSymbol || c.ordinarySymbol {
		if c.highlightedSymbol {
			c.role = graphMapHighlightedEdgeRole
		} else {
			c.role = graphMapEdgeRole
		}
		return
	}
	if c.highlightedEdge != 0 && c.edge & ^c.highlightedEdge == 0 {
		c.role = graphMapHighlightedEdgeRole
	} else {
		c.role = graphMapEdgeRole
	}
}

func graphMapEdgeRune(directions graphMapEdgeDirections) rune {
	switch directions {
	case graphMapNorth, graphMapSouth, graphMapNorth | graphMapSouth:
		return '│'
	case graphMapEast, graphMapWest, graphMapEast | graphMapWest:
		return '─'
	case graphMapEast | graphMapSouth:
		return '┌'
	case graphMapWest | graphMapSouth:
		return '┐'
	case graphMapEast | graphMapNorth:
		return '└'
	case graphMapWest | graphMapNorth:
		return '┘'
	case graphMapNorth | graphMapEast | graphMapSouth:
		return '├'
	case graphMapNorth | graphMapSouth | graphMapWest:
		return '┤'
	case graphMapEast | graphMapSouth | graphMapWest:
		return '┬'
	case graphMapNorth | graphMapEast | graphMapWest:
		return '┴'
	case graphMapNorth | graphMapEast | graphMapSouth | graphMapWest:
		return '┼'
	default:
		return '·'
	}
}

func (c graphMapCanvas) set(x, y int, r rune, role graphMapCellRole) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	c.cells[y][x] = graphMapCell{r: r, role: role}
}

func (c graphMapCanvas) drawText(x, y int, text string, role graphMapCellRole) {
	for _, r := range []rune(text) {
		c.set(x, y, r, role)
		x++
	}
}

func graphMapCardLines(node graphMapNode, selected bool) []string {
	const innerWidth = graphMapNodeWidth - 2
	const textWidth = innerWidth - 2
	marker := "  "
	if node.node.Complete {
		marker = "✓ "
	} else if node.ready {
		marker = "▶ "
	}
	wrapped := wrapWords(node.node.Text, textWidth)
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}
	if len(wrapped) > 2 {
		wrapped[1] = truncateText(strings.Join(wrapped[1:], " "), textWidth)
		wrapped = wrapped[:2]
	}
	for len(wrapped) < 2 {
		wrapped = append(wrapped, "")
	}
	topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical := "┌", "┐", "└", "┘", "─", "│"
	if selected {
		topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical = "╔", "╗", "╚", "╝", "═", "║"
	}
	lines := []string{
		topLeft + strings.Repeat(horizontal, innerWidth) + topRight,
		vertical + marker + padGraphMapText(wrapped[0], textWidth) + vertical,
		vertical + "  " + padGraphMapText(wrapped[1], textWidth) + vertical,
		bottomLeft + strings.Repeat(horizontal, innerWidth) + bottomRight,
	}
	return lines
}

func padGraphMapText(text string, width int) string {
	return padDisplay(truncateText(text, width), width)
}

func (c graphMapCanvas) render(offsetX, offsetY, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	var b strings.Builder
	for viewY := 0; viewY < height; viewY++ {
		if viewY > 0 {
			b.WriteByte('\n')
		}
		y := offsetY + viewY
		row := make([]graphMapCell, width)
		last := -1
		for viewX := 0; viewX < width; viewX++ {
			x := offsetX + viewX
			cell := graphMapCell{r: ' '}
			if y >= 0 && y < c.height && x >= 0 && x < c.width {
				cell = c.cells[y][x]
			}
			row[viewX] = cell
			if cell.r != ' ' {
				last = viewX
			}
		}
		if last < 0 {
			continue
		}
		for start := 0; start <= last; {
			role := row[start].role
			end := start + 1
			for end <= last && row[end].role == role {
				end++
			}
			var segment strings.Builder
			for i := start; i < end; i++ {
				segment.WriteRune(row[i].r)
			}
			b.WriteString(renderGraphMapRole(role, segment.String()))
			start = end
		}
	}
	return b.String()
}

func renderGraphMapRole(role graphMapCellRole, text string) string {
	switch role {
	case graphMapEdgeRole:
		return mutedStyle.Render(text)
	case graphMapHighlightedEdgeRole:
		return selectStyle.Render(text)
	case graphMapReadyRole:
		return readyStyle.Render(text)
	case graphMapCompleteRole:
		return completeStyle.Render(text)
	case graphMapSelectedRole:
		return selectStyle.Render(text)
	default:
		return lipgloss.NewStyle().Render(text)
	}
}

func (m Model) openGraphMap(returnMode mode) Model {
	m.mapReturn = returnMode
	m.mode = modeGraphMap
	candidate := m.current
	switch returnMode {
	case modeReady:
		items := m.readyItems()
		if len(items) > 0 {
			candidate = items[clampedCursor(m.readyCursor, len(items))].id
		}
	case modeLeaves:
		leaves := m.visibleLeaves()
		if len(leaves) > 0 {
			candidate = leaves[clampedCursor(m.leavesCursor, len(leaves))]
		}
	}
	m.mapSelected = candidate
	m.mapHistory = nil
	m = m.ensureGraphMapSelection()
	m.mapOffsetX, m.mapOffsetY = 0, 0
	return m.ensureGraphMapVisible()
}

func (m Model) graphMapProjection() graphMapProjection {
	return newGraphMapProjectionWithTransitive(m.g, m.showCompleted, m.mapTransitive)
}

func (m Model) ensureGraphMapSelection() Model {
	p := m.graphMapProjection()
	if p.byID[m.mapSelected] != nil {
		return m
	}
	m.mapHistory = nil
	if len(p.ready) > 0 {
		m.mapSelected = p.ready[0]
	} else if len(p.nodes) > 0 {
		m.mapSelected = p.nodes[0].node.ID
	} else {
		m.mapSelected = ""
	}
	return m
}

func (m Model) ensureGraphMapVisible() Model {
	m = m.ensureGraphMapSelection()
	layout := layoutGraphMap(m.graphMapProjection())
	placed, ok := layout.nodes[m.mapSelected]
	if !ok {
		m.mapOffsetX, m.mapOffsetY = 0, 0
		return m
	}
	width, height := m.graphMapViewportSize()
	if left := placed.x - graphMapHorizontalContext; left < m.mapOffsetX {
		m.mapOffsetX = left
	}
	if right := placed.x + graphMapNodeWidth + graphMapHorizontalContext; right > m.mapOffsetX+width {
		m.mapOffsetX = right - width
	}
	if placed.y < m.mapOffsetY {
		m.mapOffsetY = placed.y
	}
	if bottom := placed.y + graphMapNodeHeight; bottom > m.mapOffsetY+height {
		m.mapOffsetY = bottom - height
	}
	m.mapOffsetX = clampInt(m.mapOffsetX, 0, maxInt(0, layout.width-width))
	m.mapOffsetY = clampInt(m.mapOffsetY, 0, maxInt(0, layout.height-height))
	return m
}

func (m Model) graphMapViewportSize() (int, int) {
	width, height, _ := m.graphMapFrame()
	return width, height
}

func (m Model) graphMapFrame() (int, int, []string) {
	width := 80
	if m.width > 1 {
		width = m.width - 1
	} else if m.width == 1 {
		width = 1
	}
	totalHeight := m.height
	if totalHeight <= 0 {
		totalHeight = 24
	}
	maxInspectorHeight := totalHeight - 2 - graphMapNodeHeight
	inspector := m.graphMapInspector(width, maxInspectorHeight)
	height := totalHeight - 2 - len(inspector)
	if height < 1 {
		height = 1
	}
	return width, height, inspector
}

func (m Model) graphMapInspector(width, maxHeight int) []string {
	if m.mapSelected == "" {
		return nil
	}
	node, ok := m.g.Node(m.mapSelected)
	if !ok {
		return nil
	}
	return graphMapInspectorLines(node.Text, width, maxHeight)
}

func graphMapInspectorLines(text string, width, maxHeight int) []string {
	if width < 1 || maxHeight < 1 {
		return nil
	}
	if width < 4 || maxHeight < 3 {
		return []string{truncateText(text, width)}
	}
	contentWidth := width - 4
	wrapped := wrapWords(text, contentWidth)
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}
	contentLimit := maxHeight - 2
	if len(wrapped) > contentLimit {
		wrapped[contentLimit-1] = truncateText(strings.Join(wrapped[contentLimit-1:], " "), contentWidth)
		wrapped = wrapped[:contentLimit]
	}
	lines := make([]string, 0, len(wrapped)+2)
	lines = append(lines, "╔"+strings.Repeat("═", width-2)+"╗")
	for _, line := range wrapped {
		lines = append(lines, "║ "+padGraphMapText(line, contentWidth)+" ║")
	}
	lines = append(lines, "╚"+strings.Repeat("═", width-2)+"╝")
	return lines
}

func (m Model) moveGraphMapHorizontal(forward bool) Model {
	layout := layoutGraphMap(m.graphMapProjection())
	current, ok := layout.nodes[m.mapSelected]
	if !ok {
		return m.ensureGraphMapSelection()
	}
	projected := layout.projection.byID[m.mapSelected]
	if projected == nil {
		return m
	}
	if total := len(m.mapHistory); total > 0 {
		last := m.mapHistory[total-1]
		if last.to == m.mapSelected && last.forward != forward {
			m.mapSelected = last.from
			m.mapHistory = m.mapHistory[:total-1]
			return m.ensureGraphMapVisible()
		}
	}
	candidates := projected.parents
	if forward {
		candidates = projected.children
	}
	best := graph.NodeID("")
	bestRankDistance := math.MaxInt
	bestVerticalDistance := math.MaxInt
	bestFileIndex := math.MaxInt
	for _, id := range candidates {
		placed, visible := layout.nodes[id]
		if !visible {
			continue
		}
		rankDistance := placed.rank - current.rank
		if !forward {
			rankDistance = current.rank - placed.rank
		}
		if rankDistance <= 0 {
			continue
		}
		verticalDistance := absInt(placed.y - current.y)
		fileIndex := layout.projection.byID[id].fileIndex
		if rankDistance < bestRankDistance ||
			(rankDistance == bestRankDistance && verticalDistance < bestVerticalDistance) ||
			(rankDistance == bestRankDistance && verticalDistance == bestVerticalDistance && fileIndex < bestFileIndex) {
			best = id
			bestRankDistance = rankDistance
			bestVerticalDistance = verticalDistance
			bestFileIndex = fileIndex
		}
	}
	if best != "" {
		m.mapHistory = append(m.mapHistory, graphMapTraversal{
			from:    m.mapSelected,
			to:      best,
			forward: forward,
		})
		m.mapSelected = best
	}
	return m.ensureGraphMapVisible()
}

func (m Model) moveGraphMapVertical(delta int) Model {
	m.mapHistory = nil
	layout := layoutGraphMap(m.graphMapProjection())
	current, ok := layout.nodes[m.mapSelected]
	if !ok {
		return m.ensureGraphMapSelection()
	}
	var column []graphMapPlacedNode
	for _, node := range layout.nodes {
		if node.rank == current.rank {
			column = append(column, node)
		}
	}
	sort.Slice(column, func(i, j int) bool { return column[i].y < column[j].y })
	for i, node := range column {
		if node.id != m.mapSelected {
			continue
		}
		next := clampInt(i+delta, 0, len(column)-1)
		m.mapSelected = column[next].id
		break
	}
	return m.ensureGraphMapVisible()
}

func (m Model) viewGraphMap() string {
	m = m.ensureGraphMapSelection()
	layout := layoutGraphMap(m.graphMapProjection())
	width, height, inspector := m.graphMapFrame()
	title := "Graph map"
	if m.showCompleted {
		title += " · all nodes"
	} else {
		title += " · remaining"
	}
	title += fmt.Sprintf(" · %d nodes", len(layout.projection.nodes))
	if layout.projection.hiddenTransitive > 0 {
		label := "transitive edge"
		if layout.projection.hiddenTransitive != 1 {
			label += "s"
		}
		title += fmt.Sprintf(" · %d %s hidden", layout.projection.hiddenTransitive, label)
	} else if layout.projection.showingTransitive {
		title += " · transitive edges shown"
	}
	if m.message != "" {
		title += " · " + m.message
	}
	title = truncateText(title, width)
	footer := truncateText("h/l parent/child  j/k column  Enter focus  v completed  t edges  g/Esc", width)
	canvas := layout.canvas(m.mapSelected).render(m.mapOffsetX, m.mapOffsetY, width, height)
	var sections []string
	sections = append(sections, titleStyle.Render(title))
	for _, line := range inspector {
		sections = append(sections, selectStyle.Render(line))
	}
	sections = append(sections, canvas, commandStyle.Render(footer))
	return strings.Join(sections, "\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
