package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/magodo/armid"
)

type Filter struct {
	ResourceId        string
	ResourceGroupName string
	ResourceTypes     []string
	Caller            string

	// eventTimestampStart is 90 days before the eventTimestampEnd
	eventTimestampStart time.Time
	// eventTimestampEnd is the moment this filter is constructed
	eventTimestampEnd time.Time
	// eventChannels is always "Operation"
	eventChannels []string
	// levels is always "Informational"
	levels []string
}

type FilterOption struct {
	// ResourceId can't be resource group id. Use ResourceGroupName instead.
	ResourceId string

	// The name of the resource group
	ResourceGroupName string

	// ResourceTypes is a list of resource types (in the form of "Microsoft.Foo/bars/bazs").
	// E.g. Microsoft.Resources/subscriptions/resourceGroups
	ResourceTypes []string

	// The object id of the caller
	Caller string
}

func NewFilter(opt *FilterOption) *Filter {
	now := time.Now()

	f := Filter{
		eventTimestampStart: now.Add(-time.Hour * 24 * 90),
		eventTimestampEnd:   now,
		eventChannels:       []string{"Operation"},
		levels:              []string{"Informational"},
	}

	if opt != nil {
		f.ResourceId = opt.ResourceId
		f.ResourceGroupName = opt.ResourceGroupName
		f.ResourceTypes = opt.ResourceTypes
		f.Caller = opt.Caller
	}

	return &f
}

func (f Filter) String() string {
	var segs []string

	const layout = "2006-01-02T15:04:05Z"
	segs = append(segs, fmt.Sprintf("eventTimestamp ge '%s'", f.eventTimestampStart.Format(layout)))
	segs = append(segs, fmt.Sprintf("eventTimestamp le '%s'", f.eventTimestampEnd.Format(layout)))

	segs = append(segs, fmt.Sprintf("eventChannels eq '%s'", strings.Join(f.eventChannels, ",")))

	segs = append(segs, fmt.Sprintf("levels eq '%s'", strings.Join(f.levels, ",")))

	if f.ResourceId != "" {
		segs = append(segs, fmt.Sprintf("resourceId eq '%s'", f.ResourceId))
	}

	if f.ResourceGroupName != "" {
		segs = append(segs, fmt.Sprintf("resourceGroupName eq '%s'", f.ResourceGroupName))
	}

	if len(f.ResourceTypes) != 0 {
		segs = append(segs, fmt.Sprintf("resourceTypes eq '%s'", strings.Join(f.ResourceTypes, ",")))
	}

	if f.Caller != "" {
		segs = append(segs, fmt.Sprintf("caller eq '%s'", f.Caller))
	}

	return strings.Join(segs, " and ")
}

// Match is used to match the resource id of the event being returned by the List call.
// This is because some of the events returns resources beyond.
// E.g. Querying by resourceTypes of resource group can return resources within it.
//
//	(It seems the resourceTypes filter applies to the authZ scope of the operation).
func (f Filter) Match(resourceId string) (bool, error) {
	if f.ResourceId == "" && len(f.ResourceTypes) == 0 {
		return true, nil
	}

	if f.ResourceId != "" && strings.EqualFold(f.ResourceId, resourceId) {
		return true, nil
	}

	for _, rt := range f.ResourceTypes {
		id, err := armid.ParseResourceId(resourceId)
		if err != nil {
			return false, fmt.Errorf("parsing resource id %q: %v", resourceId, err)
		}
		if strings.EqualFold(id.TypeString(), rt) {
			return true, nil
		}
	}

	return false, nil
}
