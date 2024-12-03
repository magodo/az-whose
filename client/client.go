package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/google/uuid"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

type Client struct {
	actvt *armmonitor.ActivityLogsClient

	msgraph          *msgraphsdk.GraphServiceClient
	msgraphCache     map[string]string
	msgraphCacheLock sync.RWMutex
}

const ActivityLogsAPIVersion = "2017-03-01-preview"

func New(subscriptionId string, credential azcore.TokenCredential, option *arm.ClientOptions) (*Client, error) {
	if option == nil {
		option = &policy.ClientOptions{
			ClientOptions: azcorePolicy.ClientOptions{
				APIVersion: ActivityLogsAPIVersion,
			},
		}
	} else {
		if option.ClientOptions.APIVersion != ActivityLogsAPIVersion {
			return nil, fmt.Errorf("API version must be set to %q", ActivityLogsAPIVersion)
		}
	}

	actvtClient, err := armmonitor.NewActivityLogsClient(subscriptionId, credential, option)
	if err != nil {
		return nil, err
	}

	msgraphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(credential, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		return nil, err
	}

	return &Client{
		actvt:        actvtClient,
		msgraph:      msgraphClient,
		msgraphCache: map[string]string{},
	}, nil
}

func (c *Client) GetAppNameById(ctx context.Context, id string) (string, error) {
	c.msgraphCacheLock.RLock()
	if v, ok := c.msgraphCache[id]; ok {
		c.msgraphCacheLock.RUnlock()
		return v, nil
	}
	c.msgraphCacheLock.RUnlock()

	c.msgraphCacheLock.Lock()
	defer c.msgraphCacheLock.Unlock()
	result, err := c.msgraph.DirectoryObjects().ByDirectoryObjectId(id).Get(ctx, nil)
	if err != nil {
		return "", err
	}
	var name string
	sp, ok := result.(*models.ServicePrincipal)
	if !ok {
		return "", fmt.Errorf("AAD object %q is not a service principal", id)
	}
	pname := sp.GetDisplayName()
	if pname == nil {
		return "", fmt.Errorf("display name of %q is nil", id)
	}
	name = *pname
	c.msgraphCache[id] = name
	return name, nil
}

// List lists the activity events, with resource id further filtered by the filter option,
// and any application based caller will be enriched by its display name.
func (c *Client) List(ctx context.Context, filter *Filter) (Events, error) {
	var events Events
	if filter == nil {
		filter = NewFilter(nil)
	}
	pager := c.actvt.NewListPager(filter.String(), nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %v", err)
		}
		for _, v := range page.Value {
			if v != nil && v.ResourceID != nil && v.Caller != nil {
				ok, err := filter.Match(*v.ResourceID)
				if err != nil {
					return nil, err
				}

				if uuid.Validate(*v.Caller) == nil {
					name, err := c.GetAppNameById(ctx, *v.Caller)
					if err != nil {
						return nil, err
					}
					caller := fmt.Sprintf("%s (%s)", name, *v.Caller)
					v.Caller = &caller
				}

				if ok {
					events = append(events, *v)
				}
			}
		}
	}

	return events, nil
}
