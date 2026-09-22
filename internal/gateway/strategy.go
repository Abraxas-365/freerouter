package gateway

import (
	"math/rand"
	"sort"
)

// RoutingStrategy determines the order in which routes are tried.
type RoutingStrategy string

const (
	StrategyCheapest      RoutingStrategy = "cheapest"
	StrategyLowestLatency RoutingStrategy = "lowest-latency"
	StrategyRoundRobin    RoutingStrategy = "round-robin"
)

// ValidRoutingStrategies lists all accepted strategy values.
var ValidRoutingStrategies = []RoutingStrategy{StrategyCheapest, StrategyLowestLatency, StrategyRoundRobin}

// IsValid reports whether the strategy is a known value.
func (s RoutingStrategy) IsValid() bool {
	for _, v := range ValidRoutingStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// ApplyStrategy orders routes according to the given strategy.
func ApplyStrategy(routes []*RouteResult, strategy RoutingStrategy, ht *KeyHealthTracker) {
	if len(routes) <= 1 {
		return
	}
	switch strategy {
	case StrategyLowestLatency:
		sortByLatency(routes, ht)
	case StrategyRoundRobin:
		shuffleRoutes(routes)
	default:
		sortByCost(routes)
	}
}

func sortByCost(routes []*RouteResult) {
	sort.SliceStable(routes, func(i, j int) bool {
		pi, pj := routes[i].OutputPrice, routes[j].OutputPrice
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		return *pi < *pj
	})
}

func sortByLatency(routes []*RouteResult, ht *KeyHealthTracker) {
	sort.SliceStable(routes, func(i, j int) bool {
		li, lj := ht.AverageLatency(routes[i].KeyID), ht.AverageLatency(routes[j].KeyID)
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
}

func shuffleRoutes(routes []*RouteResult) {
	rand.Shuffle(len(routes), func(i, j int) { routes[i], routes[j] = routes[j], routes[i] })
}
