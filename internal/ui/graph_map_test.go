// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tunesmith/dagim/internal/graph"
)

func TestGraphMapRanksReadyNodesAtZeroAndEdgesForward(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"first-ready", "second-ready", "middle", "side", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("first-ready", "middle"))
	must(t, g.AddEdge("second-ready", "side"))
	must(t, g.AddEdge("middle", "finish"))
	must(t, g.AddEdge("first-ready", "finish"))

	p := newGraphMapProjection(g, false)
	ranks := graphMapRanks(p)
	if ranks["first-ready"] != 0 || ranks["second-ready"] != 0 {
		t.Fatalf("ready ranks = %d, %d", ranks["first-ready"], ranks["second-ready"])
	}
	if ranks["middle"] != 1 || ranks["side"] != 1 || ranks["finish"] != 2 {
		t.Fatalf("ranks = %#v", ranks)
	}
	for _, edge := range p.edges {
		if ranks[edge.parent] >= ranks[edge.child] {
			t.Fatalf("edge %s -> %s does not move right: %#v", edge.parent, edge.child, ranks)
		}
	}
}

func TestGraphMapCompletedHistoryFallsLeftOfReadyFrontier(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "ready", "future"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "ready"))
	must(t, g.AddEdge("ready", "future"))
	must(t, g.MarkComplete("start"))

	remaining := graphMapRanks(newGraphMapProjection(g, false))
	if !reflect.DeepEqual(remaining, map[graph.NodeID]int{"ready": 0, "future": 1}) {
		t.Fatalf("remaining ranks = %#v", remaining)
	}
	all := graphMapRanks(newGraphMapProjection(g, true))
	if all["start"] != -1 || all["ready"] != 0 || all["future"] != 1 {
		t.Fatalf("all ranks = %#v", all)
	}
}

func TestGraphMapFullyCompleteGraphUsesRootToLeafRanks(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "middle", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "middle"))
	must(t, g.AddEdge("middle", "finish"))
	must(t, g.MarkComplete("start"))
	must(t, g.MarkComplete("middle"))
	must(t, g.MarkComplete("finish"))

	ranks := graphMapRanks(newGraphMapProjection(g, true))
	if ranks["start"] != 0 || ranks["middle"] != 1 || ranks["finish"] != 2 {
		t.Fatalf("ranks = %#v", ranks)
	}
}

func TestGraphMapLongEdgeGetsRoutingVertices(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "middle", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "middle"))
	must(t, g.AddEdge("middle", "finish"))
	must(t, g.AddEdge("start", "finish"))

	layout := layoutGraphMap(newGraphMapProjectionWithTransitive(g, false, true))
	dummies := 0
	for _, vertex := range layout.vertices {
		if !vertex.real {
			dummies++
			if vertex.rank != 1 {
				t.Fatalf("dummy rank = %d", vertex.rank)
			}
		}
	}
	if dummies != 1 {
		t.Fatalf("dummies = %d", dummies)
	}
	if len(layout.segments) != 4 {
		t.Fatalf("segments = %d", len(layout.segments))
	}
}

func TestGraphMapHidesTransitiveEdgesByDefault(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "middle", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "middle"))
	must(t, g.AddEdge("middle", "finish"))
	must(t, g.AddEdge("start", "finish"))

	reduced := newGraphMapProjection(g, false)
	if len(reduced.edges) != 2 || reduced.hiddenTransitive != 1 {
		t.Fatalf("reduced edges, hidden = %d, %d", len(reduced.edges), reduced.hiddenTransitive)
	}
	if containsID(reduced.byID["start"].children, "finish") {
		t.Fatalf("transitive child remains in reduced projection: %#v", reduced.byID["start"].children)
	}
	full := newGraphMapProjectionWithTransitive(g, false, true)
	if len(full.edges) != 3 || full.hiddenTransitive != 0 || !full.showingTransitive {
		t.Fatalf("full edges, hidden, showing = %d, %d, %v", len(full.edges), full.hiddenTransitive, full.showingTransitive)
	}
}

func TestGraphMapHighlightsSelectedIncidentEdges(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"source", "selected", "child"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("source", "selected"))
	must(t, g.AddEdge("selected", "child"))
	layout := layoutGraphMap(newGraphMapProjection(g, false))
	canvas := layout.canvas("selected")
	for _, segment := range layout.segments {
		edgeX := segment.from.x + graphMapNodeWidth
		edgeY := segment.from.y
		if role := canvas.cells[edgeY][edgeX].role; role != graphMapHighlightedEdgeRole {
			t.Fatalf("edge %s -> %s role = %v", segment.route, segment.target, role)
		}
		if got := canvas.cells[segment.to.y][segment.to.x-1].r; got != '▶' {
			t.Fatalf("edge %s -> %s arrow = %q", segment.route, segment.target, got)
		}
	}
}

func TestGraphMapHighlightsLongInboundEdgeThroughRoutingVertex(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"source", "other-source", "middle", "target"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("source", "target"))
	must(t, g.AddEdge("other-source", "middle"))
	must(t, g.AddEdge("middle", "target"))
	layout := layoutGraphMap(newGraphMapProjection(g, false))
	canvas := layout.canvas("target")

	var routed *graphMapVertex
	for _, vertex := range layout.vertices {
		if !vertex.real && vertex.route == "source" && vertex.target == "target" {
			routed = vertex
			break
		}
	}
	if routed == nil {
		t.Fatal("missing routing vertex for long inbound edge")
	}
	x := routed.x + graphMapNodeWidth/2
	if role := canvas.cells[routed.y][x].role; role != graphMapHighlightedEdgeRole {
		t.Fatalf("long inbound track role = %v", role)
	}
}

func TestGraphMapInspectorWrapsFullSelectedText(t *testing.T) {
	text := "A deliberately long node description that wraps across several lines without losing its ending"
	lines := graphMapInspectorLines(text, 34, 12)
	if len(lines) < 4 {
		t.Fatalf("inspector lines = %#v", lines)
	}
	var content []string
	for _, line := range lines[1 : len(lines)-1] {
		runes := []rune(line)
		content = append(content, strings.TrimSpace(string(runes[2:len(runes)-2])))
	}
	if got, want := strings.Join(content, " "), text; got != want {
		t.Fatalf("inspector text = %q, want %q", got, want)
	}
	for _, line := range lines {
		if got := len([]rune(line)); got != 34 {
			t.Fatalf("line width = %d, want 34: %q", got, line)
		}
	}
}

func TestGraphMapVerticalNavigationKeepsTallGraphSelectionVisibleBelowInspector(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"first", "second", "third", "fourth", "fifth"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	m := newTestModel(t, g)
	m.width = 48
	m.height = 14
	m = m.openGraphMap(modeReady)
	for range 4 {
		m = m.moveGraphMapVertical(1)
	}
	if m.mapSelected != "fifth" {
		t.Fatalf("selected = %q", m.mapSelected)
	}
	if m.mapOffsetY == 0 {
		t.Fatal("expected vertical camera to follow selection")
	}
	layout := layoutGraphMap(m.graphMapProjection())
	placed := layout.nodes[m.mapSelected]
	_, viewportHeight := m.graphMapViewportSize()
	if placed.y < m.mapOffsetY || placed.y+graphMapNodeHeight > m.mapOffsetY+viewportHeight {
		t.Fatalf("selected card y=%d..%d outside viewport y=%d..%d", placed.y, placed.y+graphMapNodeHeight, m.mapOffsetY, m.mapOffsetY+viewportHeight)
	}
	assertViewFits(t, m.View(), m.width, m.height)
}

func TestGraphMapInspectorPreservesCardHeightInConstrainedTerminal(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("selected", strings.Repeat("long description ", 20)))
	m := newTestModel(t, g)
	m.mapSelected = "selected"
	m.width = 32
	m.height = 10

	_, viewportHeight, inspector := m.graphMapFrame()
	if viewportHeight < graphMapNodeHeight {
		t.Fatalf("viewport height = %d, want at least %d", viewportHeight, graphMapNodeHeight)
	}
	if len(inspector) != m.height-2-graphMapNodeHeight {
		t.Fatalf("inspector height = %d", len(inspector))
	}
	if !strings.Contains(strings.Join(inspector, "\n"), "...") {
		t.Fatalf("constrained inspector does not disclose truncation: %#v", inspector)
	}
}

func TestGraphMapHorizontalNavigationKeepsEdgeContextOnBothSides(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"first", "second", "middle", "fourth", "fifth"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("first", "second"))
	must(t, g.AddEdge("second", "middle"))
	must(t, g.AddEdge("middle", "fourth"))
	must(t, g.AddEdge("fourth", "fifth"))
	m := newTestModel(t, g)
	m.width = 42
	m.height = 14
	m.mapSelected = "second"
	m = m.ensureGraphMapVisible()

	m = m.moveGraphMapHorizontal(true)
	if m.mapSelected != "middle" {
		t.Fatalf("selected after right = %q", m.mapSelected)
	}
	assertGraphMapHorizontalContext(t, m)

	m.mapSelected = "fourth"
	m.mapOffsetX = 1 << 20
	m = m.ensureGraphMapVisible()
	m = m.moveGraphMapHorizontal(false)
	if m.mapSelected != "middle" {
		t.Fatalf("selected after left = %q", m.mapSelected)
	}
	assertGraphMapHorizontalContext(t, m)
}

func assertGraphMapHorizontalContext(t *testing.T, m Model) {
	t.Helper()
	layout := layoutGraphMap(m.graphMapProjection())
	placed := layout.nodes[m.mapSelected]
	width, _ := m.graphMapViewportSize()
	leftContext := placed.x - m.mapOffsetX
	rightContext := m.mapOffsetX + width - (placed.x + graphMapNodeWidth)
	if leftContext < graphMapHorizontalContext || rightContext < graphMapHorizontalContext {
		t.Fatalf("horizontal context left=%d right=%d, want at least %d", leftContext, rightContext, graphMapHorizontalContext)
	}
}

func TestGraphMapHorizontalNavigationPrefersNearestRank(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "near", "far"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "near"))
	must(t, g.AddEdge("near", "far"))
	must(t, g.AddEdge("start", "far"))
	m := newTestModel(t, g)
	m.mapTransitive = true
	m.mapSelected = "start"

	m = m.moveGraphMapHorizontal(true)
	if m.mapSelected != "near" {
		t.Fatalf("selected after right = %q", m.mapSelected)
	}
	m.mapSelected = "far"
	m = m.moveGraphMapHorizontal(false)
	if m.mapSelected != "near" {
		t.Fatalf("selected after left = %q", m.mapSelected)
	}
}

func TestGraphMapHorizontalHistoryRetracesExactEdges(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "other-start", "middle", "join", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "join"))
	must(t, g.AddEdge("other-start", "middle"))
	must(t, g.AddEdge("middle", "join"))
	must(t, g.AddEdge("join", "finish"))
	m := newTestModel(t, g)
	m.mapSelected = "start"

	m = m.moveGraphMapHorizontal(true)
	if m.mapSelected != "join" {
		t.Fatalf("selected after first right = %q", m.mapSelected)
	}
	m = m.moveGraphMapHorizontal(true)
	if m.mapSelected != "finish" {
		t.Fatalf("selected after second right = %q", m.mapSelected)
	}
	m = m.moveGraphMapHorizontal(false)
	if m.mapSelected != "join" {
		t.Fatalf("selected after first reversal = %q", m.mapSelected)
	}
	m = m.moveGraphMapHorizontal(false)
	if m.mapSelected != "start" {
		t.Fatalf("selected after second reversal = %q", m.mapSelected)
	}
	if len(m.mapHistory) != 0 {
		t.Fatalf("history after retracing = %#v", m.mapHistory)
	}
}

func TestGraphMapVerticalMovementClearsHorizontalHistory(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "other-start", "middle", "join"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "join"))
	must(t, g.AddEdge("other-start", "middle"))
	must(t, g.AddEdge("middle", "join"))
	m := newTestModel(t, g)
	m.mapSelected = "start"
	m = m.moveGraphMapHorizontal(true)
	m = m.moveGraphMapVertical(1)
	if len(m.mapHistory) != 0 {
		t.Fatalf("history after vertical movement = %#v", m.mapHistory)
	}
	m = m.moveGraphMapHorizontal(false)
	if m.mapSelected != "middle" {
		t.Fatalf("fallback parent after cleared history = %q", m.mapSelected)
	}
}

func TestGraphMapTransitiveToggleChangesProjectionAndDisclosure(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"start", "middle", "finish"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("start", "middle"))
	must(t, g.AddEdge("middle", "finish"))
	must(t, g.AddEdge("start", "finish"))
	m := newTestModel(t, g)
	m.width = 100
	m.height = 14

	next, _ := m.Update(runeKey('g'))
	mapped := next.(Model)
	if !strings.Contains(mapped.View(), "1 transitive edge hidden") {
		t.Fatalf("missing hidden-edge disclosure:\n%s", mapped.View())
	}
	if len(mapped.graphMapProjection().edges) != 2 {
		t.Fatalf("default edge count = %d", len(mapped.graphMapProjection().edges))
	}

	next, _ = mapped.Update(runeKey('t'))
	mapped = next.(Model)
	if !mapped.mapTransitive || len(mapped.graphMapProjection().edges) != 3 {
		t.Fatalf("toggle, edge count = %v, %d", mapped.mapTransitive, len(mapped.graphMapProjection().edges))
	}
	if !strings.Contains(mapped.View(), "transitive edges shown") {
		t.Fatalf("missing shown-edge disclosure:\n%s", mapped.View())
	}
}

func TestGraphMapEdgeRunesReflectConnectedDirections(t *testing.T) {
	tests := []struct {
		directions graphMapEdgeDirections
		want       rune
	}{
		{graphMapEast | graphMapWest, '─'},
		{graphMapNorth | graphMapSouth, '│'},
		{graphMapEast | graphMapSouth, '┌'},
		{graphMapWest | graphMapSouth, '┐'},
		{graphMapEast | graphMapNorth, '└'},
		{graphMapWest | graphMapNorth, '┘'},
		{graphMapNorth | graphMapEast | graphMapSouth, '├'},
		{graphMapNorth | graphMapSouth | graphMapWest, '┤'},
		{graphMapEast | graphMapSouth | graphMapWest, '┬'},
		{graphMapNorth | graphMapEast | graphMapWest, '┴'},
		{graphMapNorth | graphMapEast | graphMapSouth | graphMapWest, '┼'},
	}
	for _, test := range tests {
		if got := graphMapEdgeRune(test.directions); got != test.want {
			t.Errorf("directions %04b: got %q, want %q", test.directions, got, test.want)
		}
	}
}

func TestGraphMapBranchUsesCornersAndTJunctions(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"source", "first", "second"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("source", "first"))
	must(t, g.AddEdge("source", "second"))
	layout := layoutGraphMap(newGraphMapProjection(g, false))
	canvas := layout.canvas("source")

	var lane int
	for _, segment := range layout.segments {
		if segment.route == "source" {
			lane = segment.from.x + graphMapNodeWidth + segment.lane
			break
		}
	}
	if got := canvas.cells[2][lane].r; got != '┬' {
		t.Fatalf("source branch glyph = %q, want ┬", got)
	}
	if role := canvas.cells[2][lane].role; role != graphMapHighlightedEdgeRole {
		t.Fatalf("selected-only branch role = %v", role)
	}
	if got := canvas.cells[8][lane].r; got != '└' {
		t.Fatalf("lower branch glyph = %q, want └", got)
	}
}

func TestGraphMapMixedJunctionRendersNeutral(t *testing.T) {
	canvas := newGraphMapCanvas(3, 3)
	canvas.drawHorizontal(0, 2, 1, graphMapHighlightedEdgeRole)
	if role := canvas.cells[1][1].role; role != graphMapHighlightedEdgeRole {
		t.Fatalf("selected segment role = %v", role)
	}
	canvas.drawVertical(1, 0, 2, graphMapEdgeRole)
	if got := canvas.cells[1][1].r; got != '┼' {
		t.Fatalf("mixed junction glyph = %q, want ┼", got)
	}
	if role := canvas.cells[1][1].role; role != graphMapEdgeRole {
		t.Fatalf("mixed junction role = %v", role)
	}
}

func TestGraphMapHighlightResumesAfterMixedJunction(t *testing.T) {
	canvas := newGraphMapCanvas(5, 3)
	canvas.drawHorizontal(0, 4, 1, graphMapHighlightedEdgeRole)
	canvas.drawHorizontal(2, 4, 1, graphMapEdgeRole)
	canvas.drawVertical(2, 0, 2, graphMapEdgeRole)

	if role := canvas.cells[1][2].role; role != graphMapEdgeRole {
		t.Fatalf("mixed junction role = %v", role)
	}
	for x := 3; x <= 4; x++ {
		if role := canvas.cells[1][x].role; role != graphMapHighlightedEdgeRole {
			t.Fatalf("shared continuation role at x=%d = %v", x, role)
		}
	}
	canvas.setEdgeSymbol(4, 1, graphMapEdgeArrow, graphMapEdgeRole)
	canvas.setEdgeSymbol(4, 1, graphMapEdgeArrow, graphMapHighlightedEdgeRole)
	if role := canvas.cells[1][4].role; role != graphMapHighlightedEdgeRole {
		t.Fatalf("shared destination arrow role = %v", role)
	}
}

func TestGraphMapRoutingLanesShareSourcesAndSeparateUnrelatedEdges(t *testing.T) {
	g := graph.New()
	for _, id := range []graph.NodeID{"first-source", "second-source", "first-child", "second-child", "other-child"} {
		must(t, g.AddNodeWithID(id, string(id)))
	}
	must(t, g.AddEdge("first-source", "first-child"))
	must(t, g.AddEdge("first-source", "second-child"))
	must(t, g.AddEdge("second-source", "other-child"))
	layout := layoutGraphMap(newGraphMapProjection(g, false))

	lanes := make(map[graph.NodeID]map[int]bool)
	for _, segment := range layout.segments {
		if lanes[segment.route] == nil {
			lanes[segment.route] = make(map[int]bool)
		}
		lanes[segment.route][segment.lane] = true
	}
	if len(lanes["first-source"]) != 1 {
		t.Fatalf("first source lanes = %#v", lanes["first-source"])
	}
	if len(lanes["second-source"]) != 1 {
		t.Fatalf("second source lanes = %#v", lanes["second-source"])
	}
	var firstLane, secondLane int
	for lane := range lanes["first-source"] {
		firstLane = lane
	}
	for lane := range lanes["second-source"] {
		secondLane = lane
	}
	if firstLane == secondLane {
		t.Fatalf("unrelated sources share lane %d", firstLane)
	}
}

func TestGraphMapOpensFromReadySelectionAndFocusesNode(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("first", "First ready"))
	must(t, g.AddNodeWithID("second", "Second ready"))
	must(t, g.AddNodeWithID("child", "Child of second"))
	must(t, g.AddEdge("second", "child"))
	m := newTestModel(t, g)
	m.readyCursor = 1
	m.width = 52
	m.height = 14

	next, _ := m.Update(runeKey('g'))
	mapped := next.(Model)
	if mapped.mode != modeGraphMap || mapped.mapReturn != modeReady {
		t.Fatalf("mode, return = %v, %v", mapped.mode, mapped.mapReturn)
	}
	if mapped.mapSelected != "second" {
		t.Fatalf("selected = %q", mapped.mapSelected)
	}
	assertViewFits(t, mapped.View(), mapped.width, mapped.height)
	if view := mapped.View(); !strings.Contains(view, "Graph map") || !strings.Contains(view, "Second ready") {
		t.Fatalf("unexpected graph map:\n%s", view)
	}

	next, _ = mapped.Update(runeKey('l'))
	mapped = next.(Model)
	if mapped.mapSelected != "child" {
		t.Fatalf("selected after l = %q", mapped.mapSelected)
	}
	if mapped.mapOffsetX == 0 {
		t.Fatal("expected viewport to follow the child horizontally")
	}

	next, _ = mapped.Update(tea.KeyMsg{Type: tea.KeyEnter})
	focused := next.(Model)
	if focused.mode != modeNode || focused.current != "child" {
		t.Fatalf("mode, current = %v, %q", focused.mode, focused.current)
	}
}

func TestGraphMapEscReturnsToLaunchingView(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("root", "Root"))
	m := newTestModel(t, g)

	next, _ := m.Update(runeKey('g'))
	mapped := next.(Model)
	next, _ = mapped.Update(tea.KeyMsg{Type: tea.KeyEsc})
	returned := next.(Model)
	if returned.mode != modeReady {
		t.Fatalf("mode = %v", returned.mode)
	}
}

func TestGraphMapCompletedToggleRevealsHistoryAndKeepsSelection(t *testing.T) {
	g := graph.New()
	must(t, g.AddNodeWithID("done", "Done"))
	must(t, g.AddNodeWithID("ready", "Ready"))
	must(t, g.AddEdge("done", "ready"))
	must(t, g.MarkComplete("done"))
	m := newTestModel(t, g)
	m.width = 80
	m.height = 16

	next, _ := m.Update(runeKey('g'))
	mapped := next.(Model)
	if mapped.mapSelected != "ready" {
		t.Fatalf("selected = %q", mapped.mapSelected)
	}
	if strings.Contains(mapped.View(), "Done") {
		t.Fatalf("completed node visible before toggle:\n%s", mapped.View())
	}

	next, _ = mapped.Update(runeKey('v'))
	mapped = next.(Model)
	if !mapped.showCompleted || mapped.mapSelected != "ready" {
		t.Fatalf("showCompleted, selected = %v, %q", mapped.showCompleted, mapped.mapSelected)
	}
	if !strings.Contains(mapped.View(), "Done") {
		t.Fatalf("completed history not visible:\n%s", mapped.View())
	}
}
