package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	cloud "github.com/calyptia/api/types"
)

func newCmdGetAggregators(config *config) *cobra.Command {
	var last uint64
	var format string
	var showIDs bool
	cmd := &cobra.Command{
		Use:     "instances",
		Aliases: []string{"aggregators"},
		Short:   "Display latest core instances from a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			aa, err := config.cloud.Aggregators(config.ctx, config.projectID, cloud.AggregatorsParams{
				Last: &last,
			})
			if err != nil {
				return fmt.Errorf("could not fetch your core instances: %w", err)
			}

			switch format {
			case "table":
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 1, ' ', 0)
				if showIDs {
					fmt.Fprint(tw, "ID\t")
				}
				fmt.Fprintln(tw, "NAME\tAGE")
				for _, a := range aa.Items {
					if showIDs {
						fmt.Fprintf(tw, "%s\t", a.ID)
					}
					fmt.Fprintf(tw, "%s\t%s\n", a.Name, fmtAgo(a.CreatedAt))
				}
				tw.Flush()
			case "json":
				err := json.NewEncoder(cmd.OutOrStdout()).Encode(aa.Items)
				if err != nil {
					return fmt.Errorf("could not json encode your core instances: %w", err)
				}
			default:
				return fmt.Errorf("unknown output format %q", format)
			}
			return nil
		},
	}

	fs := cmd.Flags()
	fs.Uint64VarP(&last, "last", "l", 0, "Last `N` core instances. 0 means no limit")
	fs.StringVarP(&format, "output-format", "o", "table", "Output format. Allowed: table, json")
	fs.BoolVar(&showIDs, "show-ids", false, "Include core instance IDs in table output")

	_ = cmd.RegisterFlagCompletionFunc("output-format", config.completeOutputFormat)

	return cmd
}

func (config *config) completeAggregators(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	aa, err := config.cloud.Aggregators(config.ctx, config.projectID, cloud.AggregatorsParams{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	if len(aa.Items) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return aggregatorsKeys(aa.Items), cobra.ShellCompDirectiveNoFileComp
}

// aggregatorsKeys returns unique aggregator names first and then IDs.
func aggregatorsKeys(aa []cloud.Aggregator) []string {
	namesCount := map[string]int{}
	for _, a := range aa {
		if _, ok := namesCount[a.Name]; ok {
			namesCount[a.Name] += 1
			continue
		}

		namesCount[a.Name] = 1
	}

	var out []string

	for _, a := range aa {
		var nameIsUnique bool
		for name, count := range namesCount {
			if a.Name == name && count == 1 {
				nameIsUnique = true
				break
			}
		}
		if nameIsUnique {
			out = append(out, a.Name)
			continue
		}

		out = append(out, a.ID)
	}

	return out
}

func (config *config) loadAggregatorID(aggregatorKey string) (string, error) {
	aa, err := config.cloud.Aggregators(config.ctx, config.projectID, cloud.AggregatorsParams{
		Name: &aggregatorKey,
		Last: ptrUint64(2),
	})
	if err != nil {
		return "", err
	}

	if len(aa.Items) != 1 && !validUUID(aggregatorKey) {
		if len(aa.Items) != 0 {
			return "", fmt.Errorf("ambiguous core instance name %q, use ID instead", aggregatorKey)
		}

		return "", fmt.Errorf("could not find core instance %q", aggregatorKey)
	}

	if len(aa.Items) == 1 {
		return aa.Items[0].ID, nil
	}

	return aggregatorKey, nil

}
