package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lucasepe/codename"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cloud "github.com/calyptia/api/types"
)

func newCmdCreateAggregator(config *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "core_instance",
		Short: "Setup a new core instance on either Kubernetes, Amazon EC2 (TODO), or Google Compute Engine (TODO)",
	}
	cmd.AddCommand(newCmdCreateAggregatorOnK8s(config))
	cmd.AddCommand(newCmdCreateAggregatorOnAWS(config))
	cmd.AddCommand(newCmdCreateAggregatorOnGCP(config))
	return cmd
}

func newCmdCreateAggregatorOnK8s(config *config) *cobra.Command {
	var aggregatorName string
	var noHealthCheckPipeline bool
	var environmentID string
	var tags []string

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	cmd := &cobra.Command{
		Use:     "kubernetes",
		Aliases: []string{"kube", "k8s"},
		Short:   "Setup a new core instance on Kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
			kubeClientConfig, err := kubeConfig.ClientConfig()
			if err != nil {
				return err
			}

			clientset, err := kubernetes.NewForConfig(kubeClientConfig)
			if err != nil {
				return err
			}

			ctx := context.Background()

			created, err := config.cloud.CreateAggregator(ctx, cloud.CreateAggregator{
				Name:                   aggregatorName,
				AddHealthCheckPipeline: !noHealthCheckPipeline,
				Version:                "", // TODO
				EnvironmentID:          environmentID,
				Tags:                   tags,
			})
			if err != nil {
				return fmt.Errorf("could not create core instance at calyptia cloud: %w", err)
			}

			if configOverrides.Context.Namespace == "" {
				configOverrides.Context.Namespace = apiv1.NamespaceDefault
			}

			k8sClient := &k8sClient{
				Clientset:    clientset,
				namespace:    configOverrides.Context.Namespace,
				projectID:    config.projectID,
				projectToken: config.projectToken,
				cloudBaseURL: config.baseURL,
			}

			if err := k8sClient.ensureOwnNamespace(ctx); err != nil {
				return fmt.Errorf("could not ensure kubernetes namespace exists: %w", err)
			}

			clusterRole, err := k8sClient.createClusterRole(ctx, created)
			if err != nil {
				return fmt.Errorf("could not create kubernetes cluster role: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "cluster role: %q\n", clusterRole.Name)

			serviceAccount, err := k8sClient.createServiceAccount(ctx, created)
			if err != nil {
				return fmt.Errorf("could not create kubernetes service account: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "service account: %q\n", serviceAccount.Name)

			binding, err := k8sClient.createClusterRoleBinding(ctx, created, clusterRole, serviceAccount)
			if err != nil {
				return fmt.Errorf("could not create kubernetes cluster role binding: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "cluster role binding: %q\n", binding.Name)

			deploy, err := k8sClient.createDeployment(ctx, created, serviceAccount)
			if err != nil {
				return fmt.Errorf("could not create kubernetes deployment: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deployment: %q\n", deploy.Name)

			return nil
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&aggregatorName, "name", "", "Core instance name (autogenerated if empty)")
	fs.BoolVar(&noHealthCheckPipeline, "no-healthcheck-pipeline", false, "Disable health check pipeline creation alongside the core instance")
	fs.StringVar(&environmentID, "environment-id", "", "Calyptia environment ID") // TODO: accept name and/or ID.
	fs.StringSliceVar(&tags, "tags", nil, "Tags to apply to the core instance")
	clientcmd.BindOverrideFlags(configOverrides, fs, clientcmd.RecommendedConfigOverrideFlags("kube-"))

	_ = cmd.RegisterFlagCompletionFunc("environment-id", config.completeEnvironmentIDs)

	return cmd
}

func (config *config) completeEnvironmentIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	aa, err := config.cloud.Environments(config.ctx, config.projectID, cloud.EnvironmentsParams{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	if len(aa.Items) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return environmentsKeys(aa.Items), cobra.ShellCompDirectiveNoFileComp
}

// environmentsKeys returns unique aggregator names first and then IDs.
func environmentsKeys(aa []cloud.Environment) []string {
	var out []string

	for _, a := range aa {
		out = append(out, a.ID)
	}

	return out
}

func newCmdCreateAggregatorOnAWS(config *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "aws",
		Aliases: []string{"ec2", "amazon"},
		Short:   "Setup a new core instance on Amazon EC2 (TODO)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented")
		},
	}
	return cmd
}

func newCmdCreateAggregatorOnGCP(config *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gcp",
		Aliases: []string{"google", "gce"},
		Short:   "Setup a new core instance on Google Compute Engine (TODO)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented")
		},
	}
	return cmd
}

type k8sClient struct {
	*kubernetes.Clientset
	namespace    string
	projectID    string
	projectToken string
	cloudBaseURL string
}

func (client *k8sClient) ensureOwnNamespace(ctx context.Context) error {
	exists, err := client.ownNamespaceExists(ctx)
	if err != nil {
		return fmt.Errorf("exists: %w", err)
	}

	if exists {
		return nil
	}

	_, err = client.createOwnNamespace(ctx)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	return nil
}

func (client *k8sClient) ownNamespaceExists(ctx context.Context) (bool, error) {
	_, err := client.CoreV1().Namespaces().Get(ctx, client.namespace, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (client *k8sClient) createOwnNamespace(ctx context.Context) (*apiv1.Namespace, error) {
	return client.CoreV1().Namespaces().Create(ctx, &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: client.namespace,
		},
	}, metav1.CreateOptions{})
}

func (client *k8sClient) createClusterRole(ctx context.Context, agg cloud.CreatedAggregator) (*rbacv1.ClusterRole, error) {
	return client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: agg.Name + "-cluster-role",
			Labels: map[string]string{
				"calyptia_project_id":    client.projectID,
				"calyptia_aggregator_id": agg.ID,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps"},
				Resources: []string{
					"namespaces",
					"deployments",
					"replicasets",
					"pods",
					"services",
					"configmaps",
					"deployments/scale",
					"secrets",
				},
				Verbs: []string{
					"get",
					"list",
					"create",
					"delete",
					"patch",
					"update",
					"watch",
					"deletecollection",
				},
			},
		},
	}, metav1.CreateOptions{})
}

func (client *k8sClient) createServiceAccount(ctx context.Context, agg cloud.CreatedAggregator) (*apiv1.ServiceAccount, error) {
	return client.CoreV1().ServiceAccounts(client.namespace).Create(ctx, &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: agg.Name + "-service-account",
			Labels: map[string]string{
				"calyptia_project_id":    client.projectID,
				"calyptia_aggregator_id": agg.ID,
			},
		},
	}, metav1.CreateOptions{})
}

func (client *k8sClient) createClusterRoleBinding(
	ctx context.Context,
	agg cloud.CreatedAggregator,
	clusterRole *rbacv1.ClusterRole,
	serviceAccount *apiv1.ServiceAccount,
) (*rbacv1.ClusterRoleBinding, error) {
	return client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: agg.Name + "-cluster-role-binding",
			Labels: map[string]string{
				"calyptia_project_id":    client.projectID,
				"calyptia_aggregator_id": agg.ID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: client.namespace,
				Name:      serviceAccount.Name,
			},
		},
	}, metav1.CreateOptions{})
}

func (client *k8sClient) createDeployment(
	ctx context.Context,
	agg cloud.CreatedAggregator,
	serviceAccount *apiv1.ServiceAccount,
) (*appsv1.Deployment, error) {
	labels := map[string]string{
		"calyptia_project_id":    client.projectID,
		"calyptia_aggregator_id": agg.ID,
	}
	return client.AppsV1().Deployments(client.namespace).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   agg.Name + "-deployment",
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName:           serviceAccount.Name,
					AutomountServiceAccountToken: ptr(true),
					Containers: []apiv1.Container{
						{
							Name:            agg.Name,
							Image:           "ghcr.io/calyptia/core",
							ImagePullPolicy: apiv1.PullAlways,
							Args:            []string{"-debug=true"},
							Env: []apiv1.EnvVar{
								{
									Name:  "AGGREGATOR_NAME",
									Value: agg.Name,
								},
								{
									Name:  "PROJECT_TOKEN",
									Value: client.projectToken,
								},
								{
									Name:  "AGGREGATOR_FLUENTBIT_CLOUD_URL",
									Value: client.cloudBaseURL,
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
}

func (config *config) aggregatorExistsWithName(ctx context.Context, name string) (bool, error) {
	aa, err := config.cloud.Aggregators(ctx, config.projectID, cloud.AggregatorsParams{
		Name: &name,
		Last: ptr(uint64(1)),
	})
	if err != nil {
		return false, err
	}

	return len(aa.Items) != 0, nil
}

func ptr[T any](p T) *T {
	return &p
}

func generateAggregatorName() (string, error) {
	rng, err := codename.DefaultRNG()
	if err != nil {
		return "", err
	}

	return codename.Generate(rng, 4), nil
}
