package formatters

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	text_template "text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/calyptia/api/types"
	"github.com/calyptia/cli/cmd/utils"
	"github.com/calyptia/cli/helpers"
	"github.com/spf13/cobra"
)

func CompleteOutputFormat(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return []string{"table", "json", "yaml", "go-template"}, cobra.ShellCompDirectiveNoFileComp
}

func ConfigSectionKindName(cs types.ConfigSection) string {
	return fmt.Sprintf("%s:%s", cs.Kind, helpers.PairsName(cs.Properties))
}

func RenderEndpointsTable(w io.Writer, pp []types.PipelinePort, showIDs bool) {
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
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\n", p.Protocol, p.FrontendPort, p.BackendPort, endpoint, utils.FmtTime(p.CreatedAt))
	}
	tw.Flush()
}

func RenderUpdatedTable(w io.Writer, updatedAt time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
	fmt.Fprintln(tw, "UPDATED-AT")
	_, err := fmt.Fprintln(tw, updatedAt.Local().Format(time.RFC822))
	if err != nil {
		return err
	}

	return tw.Flush()
}

func ApplyGoTemplate(w io.Writer, outputFormat, goTemplate string, data any) error {
	if goTemplate == "" {
		parts := strings.SplitN(outputFormat, "=", 2)
		if len(parts) != 2 {
			return nil
		}

		goTemplate = trimQuotes(parts[1])

		if goTemplate == "" {
			return nil
		}
	}

	goTemplate = strings.TrimSpace(goTemplate)

	if strings.HasPrefix(outputFormat, "go-template-file") {
		b, err := os.ReadFile(goTemplate)
		if err != nil {
			return fmt.Errorf("reading go-template-file: %w", err)
		}

		goTemplate = string(bytes.TrimSpace(b))
	}

	tmpl, err := text_template.New("").Funcs(sprig.FuncMap()).Parse(goTemplate + "\n")
	if err != nil {
		return fmt.Errorf("parsing go-template: %w", err)
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		return fmt.Errorf("rendering go-template: %w", err)
	}

	return nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'' || c == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func RenderCreatedTable(w io.Writer, createdID string, createdAt time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
	fmt.Fprintln(tw, "ID\tCREATED-AT")
	_, err := fmt.Fprintf(tw, "%s\t%s\n", createdID, createdAt.Local().Format(time.RFC822))
	if err != nil {
		return err
	}

	return tw.Flush()
}
