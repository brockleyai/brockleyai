package graph

import (
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// TopologicalSort returns nodes in execution order (excluding back-edges).
// Returns an error if the graph has an unguarded cycle.
func TopologicalSort(g *model.Graph) ([]string, error) {
	// Build adjacency list (forward edges only)
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, node := range g.Nodes {
		inDegree[node.ID] = 0
	}

	for _, edge := range g.Edges {
		if edge.BackEdge {
			continue
		}
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		inDegree[edge.TargetNodeID]++
	}

	// Kahn's algorithm
	var queue []string
	for _, node := range g.Nodes {
		if inDegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, neighbor := range adj[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("graph has a cycle (processed %d of %d nodes)", len(order), len(g.Nodes))
	}

	return order, nil
}

// ParallelGroups returns groups of nodes that can execute in parallel.
// Each group contains nodes whose dependencies are all in previous groups.
func ParallelGroups(g *model.Graph) ([][]string, error) {
	// Build adjacency and reverse adjacency (forward edges only)
	deps := make(map[string]map[string]bool) // node → set of nodes it depends on
	for _, node := range g.Nodes {
		deps[node.ID] = make(map[string]bool)
	}
	for _, edge := range g.Edges {
		if edge.BackEdge {
			continue
		}
		deps[edge.TargetNodeID][edge.SourceNodeID] = true
	}

	completed := make(map[string]bool)
	remaining := make(map[string]bool)
	for _, node := range g.Nodes {
		remaining[node.ID] = true
	}

	var groups [][]string
	for len(remaining) > 0 {
		// Find nodes whose all dependencies are completed
		var ready []string
		for nodeID := range remaining {
			allDepsCompleted := true
			for dep := range deps[nodeID] {
				if !completed[dep] {
					allDepsCompleted = false
					break
				}
			}
			if allDepsCompleted {
				ready = append(ready, nodeID)
			}
		}

		if len(ready) == 0 {
			return nil, fmt.Errorf("cycle detected: %d nodes remaining but none are ready", len(remaining))
		}

		groups = append(groups, ready)
		for _, nodeID := range ready {
			completed[nodeID] = true
			delete(remaining, nodeID)
		}
	}

	return groups, nil
}
