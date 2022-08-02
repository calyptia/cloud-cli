package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	cloud "github.com/calyptia/api/types"
)

func newCmdDeleteAgent(config *config) *cobra.Command {
	var confirmed bool
	var environment string

	cmd := &cobra.Command{
		Use:               "agent AGENT",
		Short:             "Delete a single agent by ID or name",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: config.completeAgents,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentKey := args[0]
			var environmentID string
			if environment != "" {
				var err error
				environmentID, err = config.loadEnvironmentID(environment)
				if err != nil {
					return err
				}
			}

			if !confirmed {
				fmt.Printf("Are you sure you want to delete %q? (y/N) ", agentKey)
				var answer string
				_, err := fmt.Scanln(&answer)
				if err != nil && err.Error() == "unexpected newline" {
					err = nil
				}

				if err != nil {
					return fmt.Errorf("could not to read answer: %v", err)
				}

				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return nil
				}
			}

			agentID, err := config.loadAgentID(agentKey, environmentID)
			if err != nil {
				return err
			}

			err = config.cloud.DeleteAgent(config.ctx, agentID)
			if err != nil {
				return fmt.Errorf("could not delete agent: %w", err)
			}

			return nil
		},
	}

	fs := cmd.Flags()
	fs.BoolVarP(&confirmed, "yes", "y", false, "Confirm deletion")
	fs.StringVar(&environment, "environment", "", "Calyptia environment name")
	_ = cmd.RegisterFlagCompletionFunc("environment", config.completeEnvironments)

	return cmd
}

func newCmdDeleteAgents(config *config) *cobra.Command {
	var inactive bool
	var confirmed bool

	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Delete many agents from a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			aa, err := config.cloud.Agents(config.ctx, config.projectID, cloud.AgentsParams{})
			if err != nil {
				return fmt.Errorf("could not prefetch agents to delete: %w", err)
			}

			if inactive {
				var onlyInactive []cloud.Agent
				for _, a := range aa.Items {
					inactive := a.LastMetricsAddedAt.IsZero() || a.LastMetricsAddedAt.Before(time.Now().Add(time.Minute*-5))
					if inactive {
						onlyInactive = append(onlyInactive, a)
					}
				}
				aa.Items = onlyInactive
			}

			if len(aa.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No agents to delete")
				return nil
			}

			if !confirmed {
				fmt.Printf("You are about to delete:\n\n%s\n\nAre you sure you want to delete all of them? (yes/N) ", strings.Join(agentsKeys(aa.Items), "\n"))
				var answer string
				_, err := fmt.Scanln(&answer)
				if err != nil && err.Error() == "unexpected newline" {
					err = nil
				}

				if err != nil {
					return fmt.Errorf("could not to read answer: %v", err)
				}

				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "yes" {
					return nil
				}
			}

			g, gctx := errgroup.WithContext(config.ctx)
			for _, a := range aa.Items {
				a := a
				g.Go(func() error {
					err := config.cloud.DeleteAgent(gctx, a.ID)
					if err != nil {
						return fmt.Errorf("could not delete agent %q: %w", a.ID, err)
					}

					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}

			return nil
		},
	}

	fs := cmd.Flags()
	fs.BoolVar(&inactive, "inactive", true, "Delete inactive agents only")
	fs.BoolVarP(&confirmed, "yes", "y", false, "Confirm deletion")

	return cmd
}
