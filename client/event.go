package client

import (
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
)

type EventGroups map[string]EventGroup

type EventGroup struct {
	// The Azure resource id
	Id string
	// Events that are sorted by `.eventTimestamp`
	Events []armmonitor.EventData
}

type Events []armmonitor.EventData

// Succeeded only keep the events whose `.status.Value` equals to "Succeeded"
func (events Events) Succeeded() Events {
	var out Events

	for _, ev := range events {
		if ev.Status == nil {
			continue
		}
		if ev.Status.Value == nil {
			continue
		}
		if !strings.EqualFold(*ev.Status.Value, "Succeeded") {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// Group groups events by resource id (upper cased to avoid casing problems).
func (events Events) Group() EventGroups {
	out := EventGroups{}

	for _, ev := range events {
		if ev.ResourceID == nil {
			continue
		}
		// The ToUpper here is intentional to eliminate the casing issue (e.g. resourceGroups vs resourcegroups)
		id := strings.ToUpper(*ev.ResourceID)
		grp, ok := out[id]
		if !ok {
			grp = EventGroup{Id: id}
		}
		grp.Events = append(grp.Events, ev)
		out[id] = grp
	}

	for id, grp := range out {
		sort.Slice(grp.Events, func(i, j int) bool {
			if grp.Events[i].EventTimestamp == nil {
				return true
			}
			if grp.Events[j].EventTimestamp == nil {
				return false
			}
			return grp.Events[i].EventTimestamp.Before(*grp.Events[j].EventTimestamp)
		})
		out[id] = grp
	}

	return out
}
