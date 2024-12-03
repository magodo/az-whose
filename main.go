package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/magodo/armid"
	"github.com/magodo/az-whose/client"
	"github.com/urfave/cli/v2"
)

var (
	flagSubscriptionId    string
	flagResourceId        string
	flagResourceGroupName string
	flagResourceTypes     cli.StringSlice
	flagWriteWeight       int
	flagActionWeight      int
	flagDeleteWeight      int
)

func main() {
	app := &cli.App{
		Name:  "whose",
		Usage: "Find out the owner of the resources by inspecting the activity logs",
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
			&cli.IntFlag{
				Name:        "write-weight",
				Usage:       `The score weight of one "write" operation on the resource`,
				Value:       10,
				Destination: &flagWriteWeight,
			},
			&cli.IntFlag{
				Name:        "action-weight",
				Usage:       `The score weight of one "action" operation on the resource`,
				Value:       1,
				Destination: &flagActionWeight,
			},
			&cli.IntFlag{
				Name:        "delete-weight",
				Usage:       `The score weight of one "delete" operation on the resource`,
				Value:       10,
				Destination: &flagDeleteWeight,
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

			groups := events.Group()

			results := map[string]Result{}
			for id, grp := range groups {
				results[id] = Evaluate(grp, EvaluateOption{
					WriteWeight:  flagWriteWeight,
					ActionWeight: flagActionWeight,
					DeleteWeight: flagDeleteWeight,
				})
			}

			b, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("failed to marshal the final result: %v", err)
			}

			fmt.Println(string(b))

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
