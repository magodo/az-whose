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
	msgraphCache     map[string]AppInfo
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
		msgraphCache: map[string]AppInfo{},
	}, nil
}

type AppInfo struct {
	Name   string
	Owners []string
}

// GetAppInfo gets the AAD application's information by its obejct id.
func (c *Client) GetAppInfo(ctx context.Context, id string) (*AppInfo, error) {
	c.msgraphCacheLock.RLock()
	if v, ok := c.msgraphCache[id]; ok {
		c.msgraphCacheLock.RUnlock()
		return &v, nil
	}
	c.msgraphCacheLock.RUnlock()

	c.msgraphCacheLock.Lock()
	defer c.msgraphCacheLock.Unlock()
	object, err := c.msgraph.DirectoryObjects().ByDirectoryObjectId(id).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	var name string
	sp, ok := object.(*models.ServicePrincipal)
	if !ok {
		return nil, fmt.Errorf("AAD object %q is not a service principal", id)
	}
	pname := sp.GetDisplayName()
	if pname == nil {
		return nil, fmt.Errorf("display name of %q is nil", id)
	}
	name = *pname

	var owners []string
	for _, owner := range sp.GetOwners() {
		puid := owner.GetId()
		if puid == nil {
			continue
		}
		user, err := c.msgraph.Users().ByUserId(*puid).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
		pmail := user.GetMail()
		if pmail == nil {
			continue
		}
		owners = append(owners, *pmail)
	}

	info := AppInfo{Name: name, Owners: owners}
	c.msgraphCache[id] = info
	return &info, nil
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

				var caller EventCaller
				if uuid.Validate(*v.Caller) == nil {
					// In case the caller is a UUID, it represents it is a service principal.
					// We'll try to find the application by this UUID (AAD object ID).
					appCaller := EventCallerApp{
						Type:     CallerTypeApp,
						ObjectId: *v.Caller,
					}
					appInfo, err := c.GetAppInfo(ctx, *v.Caller)
					if err == nil {
						appCaller.Name = appInfo.Name
						appCaller.Owners = appInfo.Owners
					}
					caller = &appCaller
				} else {
					// Otherwise, it represents a user.
					caller = &EventCallerUser{
						Type: CallerTypeUser,
						Name: *v.Caller,
					}
				}

				event := EventData{
					EventData: *v,
					caller:    caller,
				}

				if ok {
					events = append(events, event)
				}
			}
		}
	}

	return events, nil
}
