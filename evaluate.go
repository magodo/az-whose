package main

import (
	"sort"
	"strings"

	"github.com/magodo/az-whose/client"
)

type Aggregation struct {
	Id      string
	Writes  CallCounts
	Actions CallCounts
	Deletes CallCounts
}

type CallCount struct {
	Caller string
	Count  int
}

type CallCounts []CallCount

func (c CallCounts) Len() int {
	return len(c)
}

func (c CallCounts) Less(i int, j int) bool {
	if c[i].Count == c[j].Count {
		return c[i].Count < c[j].Count
	}
	return c[i].Caller < c[j].Caller
}

func (c CallCounts) Swap(i int, j int) {
	c[i], c[j] = c[j], c[i]
}

func aggregateGroup(grp client.EventGroup) Aggregation {
	agg := Aggregation{
		Id: grp.Id,
	}

	writeCallCounts := map[string]int{}
	actionCallCounts := map[string]int{}
	deleteCallCounts := map[string]int{}

	for i, ev := range grp.Events {
		if ev.Caller == nil {
			continue
		}
		caller := *ev.Caller

		if ev.OperationName == nil || ev.OperationName.Value == nil {
			continue
		}
		switch {
		case strings.HasSuffix(*ev.OperationName.Value, "/action"):
			actionCallCounts[caller] = actionCallCounts[caller] + 1
		case strings.HasSuffix(*ev.OperationName.Value, "/write"):
			writeCallCounts[caller] = writeCallCounts[caller] + 1
		case strings.HasSuffix(*ev.OperationName.Value, "/delete"):
			// In case we get a delete operation, if this is the last operation, we record it as the operations.
			// Otherwise, it indicates the resource is being recreated, in this case we'll reset the counters.
			if i == len(grp.Events)-1 {
				deleteCallCounts[caller] = deleteCallCounts[caller] + 1
			} else {
				writeCallCounts = map[string]int{}
				actionCallCounts = map[string]int{}
				deleteCallCounts = map[string]int{}
			}
		default:
			continue
		}
	}

	for caller, cnt := range writeCallCounts {
		agg.Writes = append(agg.Writes, CallCount{Caller: caller, Count: cnt})
	}
	sort.Sort(agg.Writes)

	for caller, cnt := range actionCallCounts {
		agg.Actions = append(agg.Actions, CallCount{Caller: caller, Count: cnt})
	}
	sort.Sort(agg.Actions)

	for caller, cnt := range deleteCallCounts {
		agg.Deletes = append(agg.Deletes, CallCount{Caller: caller, Count: cnt})
	}
	sort.Sort(agg.Deletes)

	return agg
}

type Result struct {
	Id    string      `json:"id"`
	Stats []CallStats `json:"stats"`
}

type CallStats struct {
	Caller  string          `json:"caller"`
	Score   int             `json:"score"`
	Total   int             `json:"total"`
	Details CallStatsDetail `json:"details"`
}

type CallStatsDetail struct {
	Write  int `json:"write"`
	Action int `json:"action"`
	Delete int `json:"delete"`
}

type EvaluateOption struct {
	WriteWeight  int
	ActionWeight int
	DeleteWeight int
}

func Evaluate(grp client.EventGroup, opt EvaluateOption) Result {
	agg := aggregateGroup(grp)

	var writeCnt int
	for _, cc := range agg.Writes {
		writeCnt += cc.Count
	}

	var actionCnt int
	for _, cc := range agg.Actions {
		actionCnt += cc.Count
	}

	var deleteCnt int
	for _, cc := range agg.Deletes {
		deleteCnt += cc.Count
	}

	total := opt.WriteWeight*writeCnt + opt.ActionWeight*actionCnt + opt.DeleteWeight*deleteCnt

	callStatMap := map[string]*CallStats{}

	for _, cc := range agg.Writes {
		stat, ok := callStatMap[cc.Caller]
		if !ok {
			stat = &CallStats{
				Caller: cc.Caller,
				Total:  total,
			}
			callStatMap[cc.Caller] = stat
		}
		stat.Score += cc.Count * opt.WriteWeight
		stat.Details.Write = cc.Count
	}

	for _, cc := range agg.Actions {
		stat, ok := callStatMap[cc.Caller]
		if !ok {
			stat = &CallStats{
				Caller: cc.Caller,
				Total:  total,
			}
			callStatMap[cc.Caller] = stat
		}
		stat.Score += cc.Count * opt.ActionWeight
		stat.Details.Action = cc.Count
	}

	for _, cc := range agg.Deletes {
		stat, ok := callStatMap[cc.Caller]
		if !ok {
			stat = &CallStats{
				Caller: cc.Caller,
				Total:  total,
			}
			callStatMap[cc.Caller] = stat
		}
		stat.Score += cc.Count * opt.DeleteWeight
		stat.Details.Delete = cc.Count
	}

	var calls []CallStats
	for _, stat := range callStatMap {
		calls = append(calls, *stat)
	}

	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Score != calls[j].Score {
			return calls[i].Score > calls[j].Score
		}
		if calls[i].Details.Write != calls[j].Details.Write {
			return calls[i].Details.Write > calls[j].Details.Write
		}
		if calls[i].Details.Action != calls[j].Details.Action {
			return calls[i].Details.Action > calls[j].Details.Action
		}
		if calls[i].Details.Delete != calls[j].Details.Delete {
			return calls[i].Details.Delete > calls[j].Details.Delete
		}
		return calls[i].Caller > calls[j].Caller
	})

	return Result{
		Id:    grp.Id,
		Stats: calls,
	}
}
