package client

import (
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
)

type CallerType string

const (
	CallerTypeUser CallerType = "user"
	CallerTypeApp  CallerType = "app"
)

type EventCaller interface {
	// CallerIdentifier returns name for the user type; object id for the app type.
	CallerIdentifier() string
}

type EventCallerUser struct {
	Type CallerType `json:"type"`
	Name string     `json:"name"`
}

func (c *EventCallerUser) CallerIdentifier() string {
	return c.Name
}

type EventCallerApp struct {
	Type     CallerType `json:"type"`
	ObjectId string     `json:"object_id"`
	Name     string     `json:"name"`
	Owners   []string   `json:"owners"`
}

func (c *EventCallerApp) CallerIdentifier() string {
	return c.ObjectId
}

type EventData struct {
	armmonitor.EventData
	caller EventCaller
}

func (d EventData) GetCaller() EventCaller {
	return d.caller
}

type EventGroups map[string]EventGroup

type EventGroup struct {
	// The Azure resource id
	Id string
	// Events that are sorted by `.eventTimestamp`
	Events []EventData
}

type Events []EventData

// Group groups events by resource id with events sorted by timestamp.
func (events Events) Group() EventGroups {
	out := EventGroups{}

	for _, ev := range events {
		// Only keep the events whose `.status.Value` equals to "Succeeded"
		if ev.Status == nil {
			continue
		}
		if ev.Status.Value == nil {
			continue
		}
		if !strings.EqualFold(*ev.Status.Value, "Succeeded") {
			continue
		}

		// Normalize the resource id to eliminate the casing issue (e.g. resourceGroups vs resourcegroups)
		if ev.ResourceID == nil {
			continue
		}
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
