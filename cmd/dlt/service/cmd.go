/*
Copyright (c) 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	msv1 "github.com/openshift-online/ocm-sdk-go/servicemgmt/v1"
	"github.com/openshift/rosa/pkg/interactive/confirm"
	"github.com/openshift/rosa/pkg/logging"
	"github.com/openshift/rosa/pkg/ocm"
	rprtr "github.com/openshift/rosa/pkg/reporter"
)

var args struct {
	// ID of service
	ID string
}

var Cmd = &cobra.Command{
	Use:   "service",
	Short: "Deletes a service",
	Long:  "Deletes a service.",
	Example: `  # Delete a service with ID "aabbcc"
  rosa delete service --id=aabbcc`,
	Run: run,
}

func init() {
	flags := Cmd.Flags()

	flags.StringVar(&args.ID,
		"id",
		"",
		"The ID of the service to be deleted.")
}

func run(cmd *cobra.Command, _ []string) {
	reporter := rprtr.CreateReporterOrExit()
	logger := logging.CreateLoggerOrExit(reporter)

	if !confirm.Confirm("delete service with id '%s'", args.ID) {
		os.Exit(0)
	}

	// Create the client for the OCM API:
	ocmClient, err := ocm.NewClient().
		Logger(logger).
		Build()
	if err != nil {
		reporter.Errorf("Failed to create OCM connection: %v", err)
		os.Exit(1)
	}
	defer func() {
		err = ocmClient.Close()
		if err != nil {
			reporter.Errorf("Failed to close OCM connection: %v", err)
		}
	}()

	// First get the service to report additional resources
	// that must be manually deleted.
	service, err := ocmClient.GetManagedService(ocm.DescribeManagedServiceArgs{ID: args.ID})
	if err != nil {
		reporter.Errorf("Failed to get Managed Service: %s", err)
		os.Exit(1)
	}

	reporter.Debugf("Deleting service with id '%s'", args.ID)
	_, err = ocmClient.DeleteManagedService(args)
	if err != nil {
		reporter.Errorf("%s", err)
		os.Exit(1)
	}
	reporter.Infof("Service '%s' will start uninstalling now", args.ID)

	if service.Cluster().AWS().STS().RoleARN() != "" {
		reporter.Infof(
			"Your service %q will be deleted by the following objects may remain",
			args.ID,
		)
		if len(service.Cluster().AWS().STS().OperatorIAMRoles()) > 0 {
			str := "Operator IAM Roles:"
			for _, operatorIAMRole := range service.Cluster().AWS().STS().OperatorIAMRoles() {
				str = fmt.Sprintf("%s"+
					" - %s\n", str,
					operatorIAMRole.RoleARN())
			}
			reporter.Infof("%s", str)
		}
		reporter.Infof("OIDC Provider : %s\n", service.Cluster().AWS().STS().OIDCEndpointURL())
		reporter.Infof("Once the service is uninstalled use the following commands to remove the " +
			"above aws resources.\n")
		commands := buildCommands(service.Cluster())
		fmt.Print(commands, "\n")
	}
}

func buildCommands(cluster *msv1.Cluster) string {
	commands := []string{}
	deleteOperatorRole := fmt.Sprintf("\trosa delete operator-roles -c %s", cluster.Id())
	deleteOIDCProvider := fmt.Sprintf("\trosa delete oidc-provider -c %s", cluster.Id())
	commands = append(commands, deleteOperatorRole, deleteOIDCProvider)
	return strings.Join(commands, "\n")
}
