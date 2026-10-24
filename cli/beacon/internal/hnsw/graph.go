package hnsw

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sync"
	"time"
)

// This file is vendored from github.com/coder/hnsw v0.6.1 (MIT; see LICENSE in
// this directory) with two changes.
//
// Correctness: the layer search's termination condition used the closest
// element of the result set instead of the farthest, so it stopped after the
// entry node's immediate neighbours and never explored the graph. The
// replacement below is the canonical HNSW layer search.
//
// Performance (Phase 2): the original stored each node's neighbours in a map and
// rebuilt+sorted the key slice on every frontier expansion. Profiling showed
// that sort and the maps.Keys allocation dominated construction (about half of
// all bytes allocated and a third of CPU). The graph now keeps adjacency in a
// slice with the edge distance cached alongside each neighbour, iterates in
// insertion order (deterministic and allocation-free), and reuses a per-search
// scratch buffer for the visited set and the two candidate heaps. The algorithm,
// its parameters and its pruning rule are unchanged.

type Vector = []float32

// Node is a node in the graph.
type Node[K cmp.Ordered] struct {
	Key   K
	Value Vector
}

func MakeNode[K cmp.Ordered](key K, vec Vector) Node[K] {
	return Node[K]{Key: key, Value: vec}
}

// layerNode is a node in a layer of the graph.
type layerNode[K cmp.Ordered] struct {
	Node[K]

	// neighbors holds the node's neighbours in insertion order. A slice, not a
	// map, so iteration is deterministic and allocation-free. M bounds the
	// length, so the linear scans below stay cheap.
	neighbors []*layerNode[K]
	// dists[i] is the distance between this node and neighbors[i], computed once
	// when the edge was added. Cosine and Euclidean distances are symmetric, so
	// the cached value is the same in both directions; caching removes the
	// repeated distance recomputation that dominated addNeighbor.
	dists []float32
}

// indexOfNeighbor returns the position of target, or -1.
func (n *layerNode[K]) indexOfNeighbor(target *layerNode[K]) int {
	for i, neighbor := range n.neighbors {
		if neighbor == target {
			return i
		}
	}
	return -1
}

// removeAt removes the neighbour at position i, swapping the last element into
// its place so the removal is O(1) and the ordering stays deterministic.
func (n *layerNode[K]) removeAt(i int) {
	last := len(n.neighbors) - 1
	n.neighbors[i] = n.neighbors[last]
	n.dists[i] = n.dists[last]
	n.neighbors[last] = nil
	n.dists[last] = 0
	n.neighbors = n.neighbors[:last]
	n.dists = n.dists[:last]
}

// removeNeighbor drops target from the neighbour set if present.
func (n *layerNode[K]) removeNeighbor(target *layerNode[K]) {
	if i := n.indexOfNeighbor(target); i >= 0 {
		n.removeAt(i)
	}
}

// addNeighbor adds a neighbour to the node, replacing the neighbour with the
// worst distance if the neighbour set is full.
func (n *layerNode[K]) addNeighbor(newNode *layerNode[K], m int, dist DistanceFunc) {
	if n.indexOfNeighbor(newNode) >= 0 {
		return
	}
	if n.neighbors == nil {
		// Preallocate to the neighbour bound so the slice never grows past it.
		// Growth-by-append reallocated repeatedly and the resulting churn was
		// the largest remaining source of allocation and of runtime.madvise.
		n.neighbors = make([]*layerNode[K], 0, m)
		n.dists = make([]float32, 0, m)
	}
	n.neighbors = append(n.neighbors, newNode)
	n.dists = append(n.dists, dist(n.Value, newNode.Value))
	if len(n.neighbors) <= m {
		return
	}

	// Find the neighbour with the worst (largest) distance. The distance is
	// cached, so this is a comparison scan, not a recomputation.
	worst := 0
	for i := 1; i < len(n.neighbors); i++ {
		if n.dists[i] > n.dists[worst] {
			worst = i
		}
	}

	worstNode := n.neighbors[worst]
	n.removeAt(worst)
	// Delete the backlink from the worst neighbour.
	worstNode.removeNeighbor(n)
	worstNode.replenish(m, dist)
}

type searchCandidate[K cmp.Ordered] struct {
	node *layerNode[K]
	dist float32
}

func (s searchCandidate[K]) Less(o searchCandidate[K]) bool {
	return s.dist < o.dist
}

// searchScratch holds the reusable buffers of one layer search. A scratch is
// never shared between concurrent searches: the graph hands Add a persistent
// one (Add is serial) and Search a pooled one.
type searchScratch[K cmp.Ordered] struct {
	epoch   int32
	visited map[*layerNode[K]]int32
	cand    distHeap[K]
	res     distHeap[K]
}

func newSearchScratch[K cmp.Ordered]() *searchScratch[K] {
	return &searchScratch[K]{
		visited: make(map[*layerNode[K]]int32, 256),
		cand:    distHeap[K]{items: make([]searchCandidate[K], 0, 256), max: false},
		res:     distHeap[K]{items: make([]searchCandidate[K], 0, 256), max: true},
	}
}

// search returns the layer nodes closest to the target, ordered closest first.
// The returned slice is owned by the scratch and is valid only until the next
// search on the same scratch.
func (n *layerNode[K]) search(
	// k is the number of candidates in the result set.
	k int,
	efSearch int,
	target Vector,
	distance DistanceFunc,
	s *searchScratch[K],
) []searchCandidate[K] {
	// Canonical HNSW layer search: a closest-first candidate frontier and a
	// farthest-first result set of the k best found so far. Exploration stops
	// when the closest remaining candidate is farther than the worst kept
	// result. The vendored version instead compared against the *closest* result
	// and stopped immediately, which is the bug this replaces.
	s.epoch++
	epoch := s.epoch
	visited := s.visited
	visited[n] = epoch

	candidates := s.cand
	candidates.items = candidates.items[:0]
	results := s.res
	results.items = results.items[:0]

	entry := searchCandidate[K]{node: n, dist: distance(n.Value, target)}
	candidates.push(entry)
	results.push(entry)

	for candidates.len() > 0 {
		current := candidates.pop()
		if results.len() >= k && current.dist > results.items[0].dist {
			break
		}
		for _, neighbor := range current.node.neighbors {
			if visited[neighbor] == epoch {
				continue
			}
			visited[neighbor] = epoch
			d := distance(neighbor.Value, target)
			if results.len() < k || d < results.items[0].dist {
				candidates.push(searchCandidate[K]{node: neighbor, dist: d})
				results.push(searchCandidate[K]{node: neighbor, dist: d})
				if results.len() > k {
					results.pop() // max-heap: drops the farthest
				}
				if candidates.len() > efSearch {
					// Drop the farthest frontier candidate; the frontier is a
					// min-heap, so find it by scanning (efSearch is small).
					worst := 0
					for i := 1; i < candidates.len(); i++ {
						if candidates.items[i].dist > candidates.items[worst].dist {
							worst = i
						}
					}
					candidates.remove(worst)
				}
			}
		}
	}

	s.cand = candidates
	s.res = results
	out := results.items
	slices.SortFunc(out, func(a, b searchCandidate[K]) int {
		switch {
		case a.dist < b.dist:
			return -1
		case a.dist > b.dist:
			return 1
		default:
			return 0
		}
	})
	return out
}

// distHeap is a typed binary heap of searchCandidate ordered by distance. When
// max is true it is a max-heap (root is the farthest), otherwise a min-heap (root
// is the closest). The operations are typed rather than routed through
// container/heap so no value is boxed into an interface, which the profiler
// showed was allocating on every push and pop.
type distHeap[K cmp.Ordered] struct {
	items []searchCandidate[K]
	max   bool
}

func (h *distHeap[K]) len() int { return len(h.items) }

func (h *distHeap[K]) less(i, j int) bool {
	if h.max {
		return h.items[i].dist > h.items[j].dist
	}
	return h.items[i].dist < h.items[j].dist
}

func (h *distHeap[K]) swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

func (h *distHeap[K]) push(x searchCandidate[K]) {
	h.items = append(h.items, x)
	h.up(len(h.items) - 1)
}

func (h *distHeap[K]) pop() searchCandidate[K] {
	n := len(h.items) - 1
	h.swap(0, n)
	h.down(0, n)
	x := h.items[n]
	h.items = h.items[:n]
	return x
}

func (h *distHeap[K]) remove(i int) {
	n := len(h.items) - 1
	if i != n {
		h.swap(i, n)
		if !h.down(i, n) {
			h.up(i)
		}
	}
	h.items = h.items[:n]
}

func (h *distHeap[K]) up(j int) {
	for {
		i := (j - 1) / 2
		if i == j || !h.less(j, i) {
			break
		}
		h.swap(i, j)
		j = i
	}
}

func (h *distHeap[K]) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && h.less(j2, j1) {
			j = j2
		}
		if !h.less(j, i) {
			break
		}
		h.swap(i, j)
		i = j
	}
	return i > i0
}

func (n *layerNode[K]) replenish(m int, dist DistanceFunc) {
	if len(n.neighbors) >= m {
		return
	}

	// Restore connectivity by adding new neighbors. This is a naive
	// implementation that could be improved by using a priority queue to find
	// the best candidates.
	for _, neighbor := range n.neighbors {
		for _, candidate := range neighbor.neighbors {
			if candidate == n {
				continue
			}
			if n.indexOfNeighbor(candidate) >= 0 {
				// do not add duplicates
				continue
			}
			n.addNeighbor(candidate, m, dist)
			if len(n.neighbors) >= m {
				return
			}
		}
	}
}

// isolate remove the node from the graph by removing all connections
// to neighbors.
func (n *layerNode[K]) isolate(m int, dist DistanceFunc) {
	neighbors := append([]*layerNode[K](nil), n.neighbors...)
	for _, neighbor := range neighbors {
		neighbor.removeNeighbor(n)
		neighbor.replenish(m, dist)
	}
}

type layer[K cmp.Ordered] struct {
	// nodes is a map of nodes IDs to nodes.
	// All nodes in a higher layer are also in the lower layers, an essential
	// property of the graph.
	//
	// nodes is exported for interop with encoding/gob.
	nodes map[K]*layerNode[K]
}

// entry returns the entry node of the layer.
// It doesn't matter which node is returned, even that the
// entry node is consistent, so we just return the first node
// in the map to avoid tracking extra state.
func (l *layer[K]) entry() *layerNode[K] {
	if l == nil {
		return nil
	}
	for _, node := range l.nodes {
		return node
	}
	return nil
}

func (l *layer[K]) size() int {
	if l == nil {
		return 0
	}
	return len(l.nodes)
}

// Graph is a Hierarchical Navigable Small World graph.
// All public parameters must be set before adding nodes to the graph.
// K is cmp.Ordered instead of of comparable so that they can be sorted.
type Graph[K cmp.Ordered] struct {
	// Distance is the distance function used to compare embeddings.
	Distance DistanceFunc

	// Rng is used for level generation. It may be set to a deterministic value
	// for reproducibility. Note that deterministic number generation can lead to
	// degenerate graphs when exposed to adversarial inputs.
	Rng *rand.Rand

	// M is the maximum number of neighbors to keep for each node.
	// A good default for OpenAI embeddings is 16.
	M int

	// Ml is the level generation factor.
	// E.g., for Ml = 0.25, each layer is 1/4 the size of the previous layer.
	Ml float64

	// EfSearch is the number of nodes to consider in the search phase.
	// 20 is a reasonable default. Higher values improve search accuracy at the
	// expense of memory.
	EfSearch int

	// EfConstruction is the number of nodes to consider while inserting a node.
	// A wider construction beam explores more of the graph when choosing a new
	// node's neighbours, which recovers recall at smaller M. Zero means use
	// EfSearch, which is the vendored behaviour.
	EfConstruction int

	// layers is a slice of layers in the graph.
	layers []*layer[K]

	// addScratch is the reusable search buffer for Add, which is serial. Search
	// is concurrent, so it draws from searchPool instead.
	addScratch *searchScratch[K]
	searchPool sync.Pool
}

func defaultRand() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

// NewGraph returns a new graph with default parameters, roughly designed for
// storing OpenAI embeddings.
func NewGraph[K cmp.Ordered]() *Graph[K] {
	return &Graph[K]{
		M:        16,
		Ml:       0.25,
		Distance: CosineDistance,
		EfSearch: 20,
		Rng:      defaultRand(),
	}
}

// maxLevel returns an upper-bound on the number of levels in the graph
// based on the size of the base layer.
func maxLevel(ml float64, numNodes int) int {
	if ml == 0 {
		panic("ml must be greater than 0")
	}

	if numNodes == 0 {
		return 1
	}

	l := math.Log(float64(numNodes))
	l /= math.Log(1 / ml)

	m := int(math.Round(l)) + 1

	return m
}

// efConstruction returns the construction beam width.
func (g *Graph[K]) efConstruction() int {
	if g.EfConstruction > 0 {
		return g.EfConstruction
	}
	return g.EfSearch
}

// randomLevel generates a random level for a new node.
func (h *Graph[K]) randomLevel() int {
	// max avoids having to accept an additional parameter for the maximum level
	// by calculating a probably good one from the size of the base layer.
	max := 1
	if len(h.layers) > 0 {
		if h.Ml == 0 {
			panic("(*Graph).Ml must be greater than 0")
		}
		max = maxLevel(h.Ml, h.layers[0].size())
	}

	for level := 0; level < max; level++ {
		if h.Rng == nil {
			h.Rng = defaultRand()
		}
		r := h.Rng.Float64()
		if r > h.Ml {
			return level
		}
	}

	return max
}

func (g *Graph[K]) assertDims(n Vector) {
	if len(g.layers) == 0 {
		return
	}
	hasDims := g.Dims()
	if hasDims != len(n) {
		panic(fmt.Sprint("embedding dimension mismatch: ", hasDims, " != ", len(n)))
	}
}

// Dims returns the number of dimensions in the graph, or
// 0 if the graph is empty.
func (g *Graph[K]) Dims() int {
	if len(g.layers) == 0 {
		return 0
	}
	return len(g.layers[0].entry().Value)
}

func ptr[T any](v T) *T {
	return &v
}

// searchScratchFor returns the serial Add scratch, allocating it once.
func (g *Graph[K]) searchScratchFor() *searchScratch[K] {
	if g.addScratch == nil {
		g.addScratch = newSearchScratch[K]()
	}
	return g.addScratch
}

// acquireScratch returns a search buffer safe for concurrent use.
func (g *Graph[K]) acquireScratch() *searchScratch[K] {
	if v := g.searchPool.Get(); v != nil {
		return v.(*searchScratch[K])
	}
	return newSearchScratch[K]()
}

func (g *Graph[K]) releaseScratch(s *searchScratch[K]) {
	g.searchPool.Put(s)
}

// Add inserts nodes into the graph.
// If another node with the same ID exists, it is replaced.
func (g *Graph[K]) Add(nodes ...Node[K]) {
	scratch := g.searchScratchFor()
	for _, node := range nodes {
		key := node.Key
		vec := node.Value

		g.assertDims(vec)
		insertLevel := g.randomLevel()
		// Create layers that don't exist yet.
		for insertLevel >= len(g.layers) {
			g.layers = append(g.layers, &layer[K]{})
		}

		if insertLevel < 0 {
			panic("invalid level")
		}

		var elevator *K

		preLen := g.Len()

		// Insert node at each layer, beginning with the highest.
		for i := len(g.layers) - 1; i >= 0; i-- {
			layer := g.layers[i]
			newNode := &layerNode[K]{
				Node: Node[K]{
					Key:   key,
					Value: vec,
				},
			}

			// Insert the new node into the layer.
			if layer.entry() == nil {
				layer.nodes = map[K]*layerNode[K]{key: newNode}
				continue
			}

			// Now at the highest layer with more than one node, so we can begin
			// searching for the best way to enter the graph.
			searchPoint := layer.entry()

			// On subsequent layers, we use the elevator node to enter the graph
			// at the best point.
			if elevator != nil {
				searchPoint = layer.nodes[*elevator]
			}

			if g.Distance == nil {
				panic("(*Graph).Distance must be set")
			}

			neighborhood := searchPoint.search(g.M, g.efConstruction(), vec, g.Distance, scratch)
			if len(neighborhood) == 0 {
				// This should never happen because the searchPoint itself
				// should be in the result set.
				panic("no nodes found")
			}

			// Re-set the elevator node for the next layer.
			elevator = ptr(neighborhood[0].node.Key)

			if insertLevel >= i {
				if _, ok := layer.nodes[key]; ok {
					g.Delete(key)
				}
				// Insert the new node into the layer.
				layer.nodes[key] = newNode
				for _, node := range neighborhood {
					// Create a bi-directional edge between the new node and the best node.
					node.node.addNeighbor(newNode, g.M, g.Distance)
					newNode.addNeighbor(node.node, g.M, g.Distance)
				}
			}
		}

		// Invariant check: the node should have been added to the graph.
		if g.Len() != preLen+1 {
			panic("node not added")
		}
	}
}

// Search finds the k nearest neighbors from the target node.
func (h *Graph[K]) Search(near Vector, k int) []Node[K] {
	h.assertDims(near)
	if len(h.layers) == 0 {
		return nil
	}

	scratch := h.acquireScratch()
	defer h.releaseScratch(scratch)

	var (
		efSearch = h.EfSearch

		elevator *K
	)

	for layer := len(h.layers) - 1; layer >= 0; layer-- {
		searchPoint := h.layers[layer].entry()
		if elevator != nil {
			searchPoint = h.layers[layer].nodes[*elevator]
		}

		// Descending hierarchies
		if layer > 0 {
			nodes := searchPoint.search(1, efSearch, near, h.Distance, scratch)
			elevator = ptr(nodes[0].node.Key)
			continue
		}

		nodes := searchPoint.search(k, efSearch, near, h.Distance, scratch)
		out := make([]Node[K], 0, len(nodes))

		for _, node := range nodes {
			out = append(out, node.node.Node)
		}

		return out
	}

	panic("unreachable")
}

// Len returns the number of nodes in the graph.
func (h *Graph[K]) Len() int {
	if len(h.layers) == 0 {
		return 0
	}
	return h.layers[0].size()
}

// Delete removes a node from the graph by key.
// It tries to preserve the clustering properties of the graph by
// replenishing connectivity in the affected neighborhoods.
func (h *Graph[K]) Delete(key K) bool {
	if len(h.layers) == 0 {
		return false
	}

	var deleted bool
	for _, layer := range h.layers {
		node, ok := layer.nodes[key]
		if !ok {
			continue
		}
		delete(layer.nodes, key)
		node.isolate(h.M, h.Distance)
		deleted = true
	}

	return deleted
}

// Lookup returns the vector with the given key.
func (h *Graph[K]) Lookup(key K) (Vector, bool) {
	if len(h.layers) == 0 {
		return nil, false
	}

	node, ok := h.layers[0].nodes[key]
	if !ok {
		return nil, false
	}
	return node.Value, ok
}
