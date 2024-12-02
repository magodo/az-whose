package main

import (
	"sort"
	"strings"

	"github.com/magodo/azfind/client"
)

type Aggregation struct {
	Id      string
	Writes  CallCounts
	Actions CallCounts
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

func AggregateGroup(grp client.EventGroup) Aggregation {
	agg := Aggregation{
		Id: grp.Id,
	}

	writeCallCounts := map[string]int{}
	actionCallCounts := map[string]int{}

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
			// In case we get a delete operation, if this is the last operation, we regard
			// it as a "write"; Otherwise, it indicates the resource is being recreated,
			// in this case we'll reset the counters.
			if i == len(grp.Events)-1 {
				writeCallCounts[caller] = writeCallCounts[caller] + 1
			} else {
				writeCallCounts = map[string]int{}
				actionCallCounts = map[string]int{}
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

	return agg
}

type WhoseResult struct {
	Id    string               `json:"id"`
	Calls []CallCountWithTotal `json:"calls"`
}

type CallCountWithTotal struct {
	Caller string `json:"caller"`
	Count  int    `json:"count"`
	Total  int    `json:"total"`
}

func WhoseEvaluate(grp client.EventGroup) WhoseResult {
	agg := AggregateGroup(grp)

	// The strategy used here regard the "write" as dominent factors, and "action" as the minor factors.
	// The sum of "actions" makes up the unit of each "write".
	var writeCnt int
	for _, cc := range agg.Writes {
		writeCnt += cc.Count
	}

	var actionCnt int
	for _, cc := range agg.Actions {
		actionCnt += cc.Count
	}

	factor := 1
	if actionCnt != 0 {
		factor = actionCnt
	}
	total := factor*writeCnt + actionCnt

	normCallerCntMap := map[string]int{}
	for _, cc := range agg.Writes {
		normCallerCntMap[cc.Caller] = normCallerCntMap[cc.Caller] + cc.Count*factor
	}
	for _, cc := range agg.Actions {
		normCallerCntMap[cc.Caller] = normCallerCntMap[cc.Caller] + cc.Count
	}

	var normCallerCnts CallCounts
	for caller, cnt := range normCallerCntMap {
		normCallerCnts = append(normCallerCnts, CallCount{Caller: caller, Count: cnt})
	}
	sort.Sort(sort.Reverse(normCallerCnts))

	var calls []CallCountWithTotal
	for _, cc := range normCallerCnts {
		calls = append(calls, CallCountWithTotal{
			Caller: cc.Caller,
			Count:  cc.Count,
			Total:  total,
		})
	}

	return WhoseResult{
		Id:    grp.Id,
		Calls: calls,
	}
}
