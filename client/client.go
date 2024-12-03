package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	azcorePolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
)

type Client struct {
	client *armmonitor.ActivityLogsClient
}

const APIVersion = "2017-03-01-preview"

func New(subscriptionId string, credential azcore.TokenCredential, option *arm.ClientOptions) (*Client, error) {
	if option == nil {
		option = &policy.ClientOptions{
			ClientOptions: azcorePolicy.ClientOptions{
				APIVersion: APIVersion,
			},
		}
	} else {
		if option.ClientOptions.APIVersion != APIVersion {
			return nil, fmt.Errorf("API version must be set to %q", APIVersion)
		}
	}

	clientFactory, err := armmonitor.NewClientFactory(subscriptionId, credential, option)
	if err != nil {
		return nil, err
	}

	return &Client{client: clientFactory.NewActivityLogsClient()}, nil
}

func (c Client) List(ctx context.Context, filter *Filter) (Events, error) {
	var events Events
	if filter == nil {
		filter = NewFilter(nil)
	}
	pager := c.client.NewListPager(filter.String(), nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %v", err)
		}
		for _, v := range page.Value {
			if v != nil && v.ResourceID != nil {
				ok, err := filter.Match(*v.ResourceID)
				if err != nil {
					return nil, err
				}
				if ok {
					events = append(events, *v)
				}
			}
		}
	}

	return events, nil
}
