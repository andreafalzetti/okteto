// Copyright 2022 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package destroy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	ownerLabel           = "owner"
	nameLabel            = "name"
	helmOwner            = "helm"
	helmUninstallCommand = "helm uninstall %s"
)

type destroyer interface {
	DestroyWithLabel(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
	DestroySFSVolumes(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
}

type secretHandler interface {
	List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error)
}

// Options destroy commands options
type Options struct {
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the patah to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath        string
	Name                string
	Variables           []string
	Namespace           string
	DestroyVolumes      bool
	DestroyDependencies bool
	ForceDestroy        bool
	K8sContext          string
	RunWithoutBash      bool
}

type destroyCommand struct {
	getManifest func(path string) (*model.Manifest, error)

	executor          executor.ManifestExecutor
	nsDestroyer       destroyer
	secrets           secretHandler
	k8sClientProvider okteto.K8sClientProvider
	configMapHandler  configMapHandler
}

// Destroy destroys the dev application defined by the manifest
func Destroy(ctx context.Context) *cobra.Command {
	options := &Options{
		Variables: []string{},
	}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: `Destroy everything created by the 'okteto deploy' command`,
		Long:  `Destroy everything created by the 'okteto deploy' command. You can also include a 'destroy' section in your okteto manifest with a list of custom commands to be executed on destroy`,
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#destroy"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.ManifestPath != "" {
				// if path is absolute, its transformed to rel from root
				initialCWD, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get the current working directory: %w", err)
				}
				manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, options.ManifestPath)
				if err != nil {
					return err
				}
				// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
				options.ManifestPathFlag = manifestPathFlag

				// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
				uptManifestPath, err := model.UpdateCWDtoManifestPath(options.ManifestPath)
				if err != nil {
					return err
				}
				options.ManifestPath = uptManifestPath
			}
			if err := contextCMD.LoadContextFromPath(ctx, options.Namespace, options.K8sContext, options.ManifestPath); err != nil {
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.CloudURL).Error() {
					return err
				}
				if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{Namespace: options.Namespace}); err != nil {
					return err
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}
			name := options.Name
			if options.Name == "" {
				name = utils.InferName(cwd)
				if err != nil {
					return fmt.Errorf("could not infer environment name")
				}
			}

			dynClient, _, err := okteto.GetDynamicClient()
			if err != nil {
				return err
			}
			discClient, _, err := okteto.GetDiscoveryClient()
			if err != nil {
				return err
			}
			k8sClient, cfg, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}

			if options.Namespace == "" {
				options.Namespace = okteto.Context().Namespace
			}

			c := &destroyCommand{
				getManifest: model.GetManifestV2,

				executor:          executor.NewExecutor(oktetoLog.GetOutputFormat(), options.RunWithoutBash),
				configMapHandler:  newConfigmapHandler(k8sClient),
				nsDestroyer:       namespaces.NewNamespace(dynClient, discClient, cfg, k8sClient),
				secrets:           secrets.NewSecrets(k8sClient),
				k8sClientProvider: okteto.NewK8sClientProvider(),
			}

			kubeconfigPath := getTempKubeConfigFile(name)
			if err := kubeconfig.Write(okteto.Context().Cfg, kubeconfigPath); err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", kubeconfigPath)
			defer os.Remove(kubeconfigPath)
			err = c.runDestroy(ctx, options)
			analytics.TrackDestroy(err == nil)
			if err == nil {
				oktetoLog.Success("Development environment '%s' successfully destroyed", options.Name)
			}

			return err
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().BoolVarP(&options.DestroyVolumes, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().BoolVarP(&options.DestroyDependencies, "dependencies", "d", false, "destroy dependencies")
	cmd.Flags().BoolVar(&options.ForceDestroy, "force-destroy", false, "forces the development environment to be destroyed even if there is an error executing the custom destroy commands defined in the manifest")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment was deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment was deployed")
	cmd.Flags().BoolVarP(&options.RunWithoutBash, "no-bash", "", false, "execute commands without bash")

	return cmd
}

func (dc *destroyCommand) runDestroy(ctx context.Context, opts *Options) error {
	// Read manifest file with the commands to be executed
	manifest, err := dc.getManifest(opts.ManifestPath)
	if err != nil {
		// Log error message but application can still be deleted
		oktetoLog.Infof("could not find manifest file to be executed: %s", err)
		manifest = &model.Manifest{
			Destroy: []model.DeployCommand{},
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}
	if opts.Name == "" {
		if manifest.Name != "" {
			opts.Name = manifest.Name
		} else {
			opts.Name = utils.InferName(cwd)
		}

	}
	err = manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	for _, variable := range opts.Variables {
		value := strings.SplitN(variable, "=", 2)[1]
		if strings.TrimSpace(value) != "" {
			oktetoLog.AddMaskedWord(value)
		}
	}
	oktetoLog.EnableMasking()

	namespace := opts.Namespace
	if namespace == "" {
		namespace = okteto.Context().Namespace
	}

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destroying...")

	data := &pipeline.CfgData{
		Name:      opts.Name,
		Namespace: namespace,
		Status:    pipeline.DestroyingStatus,
		Filename:  opts.ManifestPathFlag,
	}

	cfg, err := dc.configMapHandler.translateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	if manifest.Context == "" {
		manifest.Context = okteto.Context().Name
	}
	if manifest.Namespace == okteto.Context().Namespace {
		manifest.Namespace = okteto.Context().Namespace
	}
	os.Setenv(constants.OktetoNameEnvVar, opts.Name)

	if opts.DestroyDependencies {
		for depName, depInfo := range manifest.Dependencies {
			namespace := okteto.Context().Namespace
			if depInfo.Namespace != "" {
				namespace = depInfo.Namespace
			}
			destOpts := &pipelineCMD.DestroyOptions{
				Name:           depName,
				DestroyVolumes: opts.DestroyVolumes,
				Namespace:      namespace,
			}
			pipelineCmd, err := pipelineCMD.NewCommand()
			if err != nil {
				if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
					return err
				}
				return err
			}
			if err := pipelineCmd.ExecuteDestroyPipeline(ctx, destOpts); err != nil {
				if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
					return err
				}
				return err
			}
		}
	}

	var commandErr error
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		for _, command := range manifest.Destroy {
			oktetoLog.Information("Running '%s'", command.Name)
			oktetoLog.SetStage(command.Name)
			if err := dc.executor.Execute(command, opts.Variables); err != nil {
				err = fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
				if !opts.ForceDestroy {
					if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
						exit <- err
						return
					}
					exit <- err
					return
				}

				// Store the error to return if the force destroy option is set
				commandErr = err
			}
		}
		exit <- nil
	}()
	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		errStop := "interrupt signal received"
		dc.executor.CleanUp(errors.New(errStop))
		if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, fmt.Errorf(errStop)); err != nil {
			return err
		}
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	oktetoLog.SetStage("")
	oktetoLog.DisableMasking()

	oktetoLog.Spinner(fmt.Sprintf("Destroying development environment '%s'...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	deployedByLs, err := labels.NewRequirement(
		model.DeployedByLabel,
		selection.Equals,
		[]string{opts.Name},
	)
	if err != nil {
		if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}
	deployedBySelector := labels.NewSelector().Add(*deployedByLs).String()
	deleteOpts := namespaces.DeleteAllOptions{
		LabelSelector:  deployedBySelector,
		IncludeVolumes: opts.DestroyVolumes,
	}

	oktetoLog.SetStage("Destroying volumes")
	if err := dc.nsDestroyer.DestroySFSVolumes(ctx, opts.Namespace, deleteOpts); err != nil {
		if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}

	oktetoLog.SetStage("Destroying Helm release")
	if err := dc.destroyHelmReleasesIfPresent(ctx, opts, deployedBySelector); err != nil {
		if !opts.ForceDestroy {
			if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
				return err
			}
			return err
		}
	}

	oktetoLog.Debugf("destroying resources with deployed-by label '%s'", deployedBySelector)
	oktetoLog.SetStage(fmt.Sprintf("Destroying by label '%s'", deployedBySelector))
	if err := dc.nsDestroyer.DestroyWithLabel(ctx, opts.Namespace, deleteOpts); err != nil {
		oktetoLog.Infof("could not delete all the resources: %s", err)
		if err := dc.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}

	oktetoLog.SetStage("Destroying configmap")

	if err := dc.configMapHandler.destroyConfigMap(ctx, cfg, namespace); err != nil {
		return err
	}

	return commandErr
}

func (dc *destroyCommand) destroyHelmReleasesIfPresent(ctx context.Context, opts *Options, labelSelector string) error {
	sList, err := dc.secrets.List(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return err
	}

	oktetoLog.Debugf("checking if application installed something with helm")
	helmReleases := map[string]bool{}
	for _, s := range sList {
		if s.Type == model.HelmSecretType && s.Labels[ownerLabel] == helmOwner {
			helmReleaseName, ok := s.Labels[nameLabel]
			if !ok {
				continue
			}

			helmReleases[helmReleaseName] = true
		}
	}

	// If the application to be destroyed was deployed with helm, we try to uninstall it to avoid to leave orphan release resources
	for releaseName := range helmReleases {
		oktetoLog.Debugf("uninstalling helm release '%s'", releaseName)
		cmd := fmt.Sprintf(helmUninstallCommand, releaseName)
		cmdInfo := model.DeployCommand{Command: cmd, Name: cmd}
		oktetoLog.Information("Running '%s'", cmdInfo.Name)
		if err := dc.executor.Execute(cmdInfo, opts.Variables); err != nil {
			oktetoLog.Infof("could not uninstall helm release '%s': %s", releaseName, err)
			if !opts.ForceDestroy {
				return err
			}
		}
	}

	return nil
}

func getTempKubeConfigFile(name string) string {
	tempKubeconfigFileName := fmt.Sprintf("kubeconfig-destroy-%s-%d", name, time.Now().UnixMilli())
	return filepath.Join(config.GetOktetoHome(), tempKubeconfigFileName)
}
