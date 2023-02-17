package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	cloud "github.com/calyptia/api/types"
	"github.com/calyptia/cli/pkg/completer"
	cfg "github.com/calyptia/cli/pkg/config"
	"github.com/calyptia/cli/pkg/formatters"
)

func newCmdGetEndpoints(config *cfg.Config) *cobra.Command {
	var pipelineKey string
	var last uint
	var outputFormat, goTemplate string
	var showIDs bool
	completer := completer.Completer{Config: config}

	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "Display latest endpoints from a pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			pipelineID, err := config.LoadPipelineID(pipelineKey)
			if err != nil {
				return err
			}

			pp, err := config.Cloud.PipelinePorts(config.Ctx, pipelineID, cloud.PipelinePortsParams{
				Last: &last,
			})
			if err != nil {
				return fmt.Errorf("could not fetch your pipeline endpoints: %w", err)
			}

			if strings.HasPrefix(outputFormat, "go-template") {
				return applyGoTemplate(cmd.OutOrStdout(), outputFormat, goTemplate, pp.Items)
			}

			switch outputFormat {
			case "table":
				renderEndpointsTable(cmd.OutOrStdout(), pp.Items, showIDs)
			case "json":
				return json.NewEncoder(cmd.OutOrStdout()).Encode(pp.Items)
			case "yml", "yaml":
				return yaml.NewEncoder(cmd.OutOrStdout()).Encode(pp.Items)
			default:
				return fmt.Errorf("unknown output format %q", outputFormat)
			}
			return nil
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&pipelineKey, "pipeline", "", "Parent pipeline ID or name")
	fs.UintVarP(&last, "last", "l", 0, "Last `N` pipeline endpoints. 0 means no limit")
	fs.BoolVar(&showIDs, "show-ids", false, "Include endpoint IDs in table output")
	fs.StringVarP(&outputFormat, "output-format", "o", "table", "Output format. Allowed: table, json, yaml, go-template, go-template-file")
	fs.StringVar(&goTemplate, "template", "", "Template string or path to use when -o=go-template, -o=go-template-file. The template format is golang templates\n[http://golang.org/pkg/text/template/#pkg-overview]")

	_ = cmd.RegisterFlagCompletionFunc("output-format", formatters.CompleteOutputFormat)
	_ = cmd.RegisterFlagCompletionFunc("pipeline", completer.CompletePipelines)

	_ = cmd.MarkFlagRequired("pipeline") // TODO: use default pipeline key from config cmd.

	return cmd
}

func renderEndpointsTable(w io.Writer, pp []cloud.PipelinePort, showIDs bool) {
	tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
	if showIDs {
		fmt.Fprint(tw, "ID\t")
	}
	fmt.Fprintln(tw, "PROTOCOL\tFRONTEND-PORT\tBACKEND-PORT\tENDPOINT\tAGE")
	for _, p := range pp {
		endpoint := p.Endpoint
		if endpoint == "" {
			endpoint = "Pending"
		}
		if showIDs {
			fmt.Fprintf(tw, "%s\t", p.ID)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\n", p.Protocol, p.FrontendPort, p.BackendPort, endpoint, fmtTime(p.CreatedAt))
	}
	tw.Flush()
}
