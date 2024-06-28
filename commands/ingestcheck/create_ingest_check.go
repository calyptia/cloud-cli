package ingestcheck

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/calyptia/api/types"
	cfg "github.com/calyptia/cli/config"
)

func NewCmdCreateIngestCheck(config *cfg.Config) *cobra.Command {
	var (
		retries         uint
		configSectionID string
		status          string
		environment     string
		collectLogs     bool
	)

	cmd := &cobra.Command{
		Use:   "ingest_check CORE_INSTANCE",
		Short: "Create an ingest check",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			coreInstance := args[0]

			params := types.CreateIngestCheck{
				CollectLogs: collectLogs,
			}
			if configSectionID == "" {
				return fmt.Errorf("invalid config section id")
			}

			params.ConfigSectionID = configSectionID
			if retries > 0 {
				params.Retries = retries
			}

			if status != "" && !types.ValidCheckStatus(types.CheckStatus(status)) {
				return fmt.Errorf("invalid check status")
			}

			var environmentID string
			if environment != "" {
				var err error
				environmentID, err = config.Completer.LoadEnvironmentID(ctx, environment)
				if err != nil {
					return err
				}
			}
			coreInstanceID, err := config.Completer.LoadCoreInstanceID(ctx, coreInstance, environmentID)
			if err != nil {
				return err
			}

			check, err := config.Cloud.CreateIngestCheck(ctx, coreInstanceID, params)
			if err != nil {
				return err
			}
			cmd.Println(check.ID)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.UintVar(&retries, "retries", 0, "number of retries")
	flags.StringVar(&configSectionID, "config-section-id", "", "config section ID")
	flags.StringVar(&status, "status", "", "status")
	flags.BoolVar(&collectLogs, "collect-logs", false, "Collect logs from the kubernetes pods once the job is finished")
	flags.StringVar(&environment, "environment", "default", "calyptia environment name")
	return cmd
}
