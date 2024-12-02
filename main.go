package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/magodo/armid"
	"github.com/magodo/azfind/client"
	"github.com/urfave/cli/v2"
)

var (
	flagSubscriptionId    string
	flagResourceId        string
	flagResourceGroupName string
	flagResourceTypes     cli.StringSlice
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "whose",
				Usage: "Find out the owner of the resources",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "subscription-id",
						Aliases:     []string{"s"},
						Usage:       "The Azure subscription id",
						Required:    true,
						Destination: &flagSubscriptionId,
					},
					&cli.StringFlag{
						Name:        "resource-id",
						Usage:       "The resource id",
						Destination: &flagResourceId,
					},
					&cli.StringFlag{
						Name:        "resource-group-name",
						Usage:       "The resource group name",
						Destination: &flagResourceGroupName,
					},
					&cli.StringSliceFlag{
						Name:        "resource-type",
						Usage:       "The resource type (can be specified multiple times)",
						Destination: &flagResourceTypes,
					},
				},
				Before: func(ctx *cli.Context) error {
					if flagResourceId != "" {
						if flagResourceGroupName != "" {
							return errors.New(`"-resource-id" conflicts with "-resource-group-name"`)
						}
						if len(flagResourceTypes.Value()) != 0 {
							return errors.New(`"-resource-id" conflicts with "-resource-type"`)
						}
					}
					return nil
				},
				Action: func(cCtx *cli.Context) error {
					cred, err := azidentity.NewDefaultAzureCredential(nil)
					if err != nil {
						return fmt.Errorf("failed to obtain a credential: %v", err)
					}

					c, err := client.New(flagSubscriptionId, cred, nil)
					if err != nil {
						return fmt.Errorf("new client: %v", err)
					}

					filterOption := client.FilterOption{
						ResourceGroupName: flagResourceGroupName,
						ResourceTypes:     flagResourceTypes.Value(),
					}
					if flagResourceId != "" {
						rid, err := armid.ParseResourceId(flagResourceId)
						if err != nil {
							return fmt.Errorf("failed to parse resource id: %v", err)
						}
						filterOption = client.FilterOption{
							ResourceId: flagResourceId,
						}
						if rgid, ok := rid.(*armid.ResourceGroup); ok {
							filterOption = client.FilterOption{
								ResourceTypes:     []string{rgid.TypeString()},
								ResourceGroupName: rgid.Name,
							}
						}
					}

					events, err := c.List(cCtx.Context, client.NewFilter(&filterOption))
					if err != nil {
						return fmt.Errorf("failed to list events: %v", err)
					}

					groups := events.Filter().Group()

					if flagResourceId != "" {
						rid, _ := armid.ParseResourceId(flagResourceId)
						// In case the resource-id is used, the groups shall only contain one entry as we filter by resource id.
						// Specifically, the filter for resouce group id is a bit different: instead of using the resourceId, we
						// were using resourceTypes + resourceGroupName, and this can return more events for resources within this
						// resource group (e.g. deleting the resource group with all its resources).
						// In this case, we'll skip the other resources than the resource group.
						if rgid, ok := rid.(*armid.ResourceGroup); ok {
							newGroups := client.EventGroups{}
							for id, grp := range groups {
								if strings.EqualFold(id, rgid.String()) {
									newGroups[id] = grp
									break
								}
							}
							groups = newGroups
							if len(groups) != 1 {
								return fmt.Errorf("expect one resource group entry to be found, got=%d", len(groups))
							}
						}
					}

					results := map[string]WhoseResult{}
					for id, grp := range groups {
						results[id] = WhoseEvaluate(grp)
					}

					b, err := json.Marshal(results)
					if err != nil {
						return fmt.Errorf("failed to marshal the final result: %v", err)
					}

					fmt.Println(string(b))

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
