package e2e

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/strings/slices"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/rosa/tests/ci/labels"
	"github.com/openshift/rosa/tests/utils/common"
	"github.com/openshift/rosa/tests/utils/common/constants"
	"github.com/openshift/rosa/tests/utils/config"
	"github.com/openshift/rosa/tests/utils/exec/rosacli"
	. "github.com/openshift/rosa/tests/utils/log"
)

var _ = Describe("Edit nodepool",
	labels.Day2,
	labels.FeatureNodePool,
	labels.NonClassicCluster,
	func() {
		defer GinkgoRecover()

		var (
			clusterID                 string
			rosaClient                *rosacli.Client
			clusterService            rosacli.ClusterService
			machinePoolService        rosacli.MachinePoolService
			machinePoolUpgradeService rosacli.MachinePoolUpgradeService
			versionService            rosacli.VersionService
		)

		const (
			defaultNodePoolReplicas = "2"
		)

		BeforeEach(func() {
			var err error

			By("Get the cluster")
			clusterID = config.GetClusterID()
			Expect(clusterID).ToNot(Equal(""), "ClusterID is required. Please export CLUSTER_ID")

			By("Init the client")
			rosaClient = rosacli.NewClient()
			clusterService = rosaClient.Cluster
			machinePoolService = rosaClient.MachinePool
			machinePoolUpgradeService = rosaClient.MachinePoolUpgrade
			versionService = rosaClient.Version

			By("Check hosted cluster")
			hosted, err := clusterService.IsHostedCPCluster(clusterID)
			Expect(err).ToNot(HaveOccurred())
			if !hosted {
				Skip("Node pools are only supported on Hosted clusters")
			}
		})

		AfterEach(func() {
			By("Clean remaining resources")
			err := rosaClient.CleanResources(clusterID)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create/edit/list/delete nodepool - [id:56782]",
			labels.Critical,
			func() {
				nodePoolName := common.GenerateRandomName("np-56782", 2)
				labels := "label1=value1,label2=value2"
				taints := "t1=v1:NoSchedule,l2=:NoSchedule"
				instanceType := "m5.2xlarge"

				By("Retrieve cluster initial information")
				cluster, err := clusterService.DescribeClusterAndReflect(clusterID)
				Expect(err).ToNot(HaveOccurred())
				cpVersion := cluster.OpenshiftVersion

				By("Create new nodepool")
				output, err := machinePoolService.CreateMachinePool(clusterID, nodePoolName,
					"--replicas", "0",
					"--instance-type", instanceType,
					"--labels", labels,
					"--taints", taints)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Machine pool '%s' created successfully on hosted cluster '%s'", nodePoolName, clusterID))

				By("Check created nodepool")
				npList, err := machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np := npList.Nodepool(nodePoolName)
				Expect(np).ToNot(BeNil())
				Expect(np.AutoScaling).To(Equal("No"))
				Expect(np.Replicas).To(Equal("0/0"))
				Expect(np.InstanceType).To(Equal(instanceType))
				Expect(np.AvalaiblityZones).ToNot(BeNil())
				Expect(np.Subnet).ToNot(BeNil())
				Expect(np.Version).To(Equal(cpVersion))
				Expect(np.AutoRepair).To(Equal("Yes"))
				Expect(len(common.ParseLabels(np.Labels))).To(Equal(len(common.ParseLabels(labels))))
				Expect(common.ParseLabels(np.Labels)).To(ContainElements(common.ParseLabels(labels)))
				Expect(len(common.ParseTaints(np.Taints))).To(Equal(len(common.ParseTaints(taints))))
				Expect(common.ParseTaints(np.Taints)).To(ContainElements(common.ParseTaints(taints)))

				By("Edit nodepool")
				newLabels := "l3=v3"
				newTaints := "t3=value3:NoExecute"
				replicasNb := "3"
				output, err = machinePoolService.EditMachinePool(clusterID, nodePoolName,
					"--replicas", replicasNb,
					"--labels", newLabels,
					"--taints", newTaints)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Updated machine pool '%s' on hosted cluster '%s'", nodePoolName, clusterID))

				By("Check edited nodepool")
				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np = npList.Nodepool(nodePoolName)
				Expect(np).ToNot(BeNil())
				Expect(np.Replicas).To(Equal(fmt.Sprintf("0/%s", replicasNb)))
				Expect(len(common.ParseLabels(np.Labels))).To(Equal(len(common.ParseLabels(newLabels))))
				Expect(common.ParseLabels(np.Labels)).To(BeEquivalentTo(common.ParseLabels(newLabels)))
				Expect(len(common.ParseTaints(np.Taints))).To(Equal(len(common.ParseTaints(newTaints))))
				Expect(common.ParseTaints(np.Taints)).To(BeEquivalentTo(common.ParseTaints(newTaints)))

				By("Check describe nodepool")
				npDesc, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())

				Expect(npDesc).ToNot(BeNil())
				Expect(npDesc.AutoScaling).To(Equal("No"))
				Expect(npDesc.DesiredReplicas).To(Equal(replicasNb))
				Expect(npDesc.CurrentReplicas).To(Equal("0"))
				Expect(npDesc.InstanceType).To(Equal(instanceType))
				Expect(npDesc.AvalaiblityZones).ToNot(BeNil())
				Expect(npDesc.Subnet).ToNot(BeNil())
				Expect(npDesc.Version).To(Equal(cpVersion))
				Expect(npDesc.AutoRepair).To(Equal("Yes"))
				Expect(len(common.ParseLabels(npDesc.Labels))).To(Equal(len(common.ParseLabels(newLabels))))
				Expect(common.ParseLabels(npDesc.Labels)).To(BeEquivalentTo(common.ParseLabels(newLabels)))
				Expect(len(common.ParseTaints(npDesc.Taints))).To(Equal(len(common.ParseTaints(newTaints))))
				Expect(common.ParseTaints(npDesc.Taints)).To(BeEquivalentTo(common.ParseTaints(newTaints)))

				By("Wait for nodepool replicas available")
				err = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 20*time.Minute, false, func(context.Context) (bool, error) {
					npDesc, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
					if err != nil {
						return false, err
					}
					return npDesc.CurrentReplicas == replicasNb, nil
				})
				common.AssertWaitPollNoErr(err, "Replicas are not ready after 600")

				By("Delete nodepool")
				output, err = machinePoolService.DeleteMachinePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Successfully deleted machine pool '%s' from hosted cluster '%s'", nodePoolName, clusterID))

				By("Nodepool does not appear anymore")
				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				Expect(npList.Nodepool(nodePoolName)).To(BeNil())

				if len(npList.NodePools) == 1 {
					By("Try to delete remaining nodepool")
					output, err = machinePoolService.DeleteMachinePool(clusterID, npList.NodePools[0].ID)
					Expect(err).To(HaveOccurred())
					Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Failed to delete machine pool '%s' on hosted cluster '%s': The last node pool can not be deleted from a cluster.", npList.NodePools[0].ID, clusterID))
				}
			})

		It("can create nodepool with defined subnets - [id:60202]",
			labels.Critical,
			func() {
				var subnets []string
				nodePoolName := common.GenerateRandomName("np-60202", 2)
				replicasNumber := 3
				maxReplicasNumber := 6

				By("Retrieve cluster nodes information")
				CD, err := clusterService.DescribeClusterAndReflect(clusterID)
				Expect(err).ToNot(HaveOccurred())
				initialNodesNumber, err := rosacli.RetrieveDesiredComputeNodes(CD)
				Expect(err).ToNot(HaveOccurred())

				By("List nodepools")
				npList, err := machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				for _, np := range npList.NodePools {
					Expect(np.ID).ToNot(BeNil())
					if strings.HasPrefix(np.ID, constants.DefaultHostedWorkerPool) {
						Expect(np.AutoScaling).ToNot(BeNil())
						Expect(np.Subnet).ToNot(BeNil())
						Expect(np.AutoRepair).ToNot(BeNil())
					}

					if !slices.Contains(subnets, np.Subnet) {
						subnets = append(subnets, np.Subnet)
					}
				}

				By("Create new nodepool with defined subnet")
				output, err := machinePoolService.CreateMachinePool(clusterID, nodePoolName,
					"--replicas", strconv.Itoa(replicasNumber),
					"--subnet", subnets[0])
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Machine pool '%s' created successfully on hosted cluster '%s'", nodePoolName, clusterID))

				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np := npList.Nodepool(nodePoolName)
				Expect(np).ToNot(BeNil())
				Expect(np.AutoScaling).To(Equal("No"))
				Expect(np.Replicas).To(Equal("0/3"))
				Expect(np.AvalaiblityZones).ToNot(BeNil())
				Expect(np.Subnet).To(Equal(subnets[0]))
				Expect(np.AutoRepair).To(Equal("Yes"))

				By("Check cluster nodes information with new replicas")
				CD, err = clusterService.DescribeClusterAndReflect(clusterID)
				Expect(err).ToNot(HaveOccurred())
				newNodesNumber, err := rosacli.RetrieveDesiredComputeNodes(CD)
				Expect(err).ToNot(HaveOccurred())
				Expect(newNodesNumber).To(Equal(initialNodesNumber + replicasNumber))

				By("Add autoscaling to nodepool")
				output, err = machinePoolService.EditMachinePool(clusterID, nodePoolName,
					"--enable-autoscaling",
					"--min-replicas", strconv.Itoa(replicasNumber),
					"--max-replicas", strconv.Itoa(maxReplicasNumber),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Updated machine pool '%s' on hosted cluster '%s'", nodePoolName, clusterID))
				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np = npList.Nodepool(nodePoolName)
				Expect(np).ToNot(BeNil())
				Expect(np.AutoScaling).To(Equal("Yes"))

				// Change autorepair
				output, err = machinePoolService.EditMachinePool(clusterID, nodePoolName,
					"--autorepair=false",

					// Temporary fix until https://issues.redhat.com/browse/OCM-5186 is corrected
					"--enable-autoscaling",
					"--min-replicas", strconv.Itoa(replicasNumber),
					"--max-replicas", strconv.Itoa(maxReplicasNumber),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Updated machine pool '%s' on hosted cluster '%s'", nodePoolName, clusterID))
				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np = npList.Nodepool(nodePoolName)
				Expect(np).ToNot(BeNil())
				Expect(np.AutoRepair).To(Equal("No"))

				By("Delete nodepool")
				output, err = machinePoolService.DeleteMachinePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Successfully deleted machine pool '%s' from hosted cluster '%s'", nodePoolName, clusterID))

				By("Check cluster nodes information after deletion")
				CD, err = clusterService.DescribeClusterAndReflect(clusterID)
				Expect(err).ToNot(HaveOccurred())
				newNodesNumber, err = rosacli.RetrieveDesiredComputeNodes(CD)
				Expect(err).ToNot(HaveOccurred())
				Expect(newNodesNumber).To(Equal(initialNodesNumber))

				By("Create new nodepool with replicas 0")
				replicas0NPName := common.GenerateRandomName(nodePoolName, 2)
				_, err = machinePoolService.CreateMachinePool(clusterID, replicas0NPName,
					"--replicas", strconv.Itoa(0),
					"--subnet", subnets[0])
				Expect(err).ToNot(HaveOccurred())
				npList, err = machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				np = npList.Nodepool(replicas0NPName)
				Expect(np).ToNot(BeNil())
				Expect(np.Replicas).To(Equal("0/0"))

				By("Create new nodepool with min replicas 0")
				minReplicas0NPName := common.GenerateRandomName(nodePoolName, 2)
				_, err = machinePoolService.CreateMachinePool(clusterID, minReplicas0NPName,
					"--enable-autoscaling",
					"--min-replicas", strconv.Itoa(0),
					"--max-replicas", strconv.Itoa(3),
					"--subnet", subnets[0],
				)
				Expect(err).To(HaveOccurred())
			})

		It("can create nodepool with tuning config - [id:63178]",
			labels.Critical,
			func() {
				tuningConfigService := rosaClient.TuningConfig
				nodePoolName := common.GenerateRandomName("np-63178", 2)
				tuningConfig1Name := common.GenerateRandomName("tuned01", 2)
				tuningConfig2Name := common.GenerateRandomName("tuned02", 2)
				tuningConfig3Name := common.GenerateRandomName("tuned03", 2)
				allTuningConfigNames := []string{tuningConfig1Name, tuningConfig2Name, tuningConfig3Name}

				tuningConfigPayload := `
		{
			"profile": [
			  {
				"data": "[main]\nsummary=Custom OpenShift profile\ninclude=openshift-node\n\n[sysctl]\nvm.dirty_ratio=\"25\"\n",
				"name": "%s-profile"
			  }
			],
			"recommend": [
			  {
				"priority": 10,
				"profile": "%s-profile"
			  }
			]
		 }
		`

				By("Prepare tuning configs")
				_, err := tuningConfigService.CreateTuningConfig(clusterID, tuningConfig1Name, fmt.Sprintf(tuningConfigPayload, tuningConfig1Name, tuningConfig1Name))
				Expect(err).ToNot(HaveOccurred())
				_, err = tuningConfigService.CreateTuningConfig(clusterID, tuningConfig2Name, fmt.Sprintf(tuningConfigPayload, tuningConfig2Name, tuningConfig2Name))
				Expect(err).ToNot(HaveOccurred())
				_, err = tuningConfigService.CreateTuningConfig(clusterID, tuningConfig3Name, fmt.Sprintf(tuningConfigPayload, tuningConfig3Name, tuningConfig3Name))
				Expect(err).ToNot(HaveOccurred())

				By("Create nodepool with tuning configs")
				_, err = machinePoolService.CreateMachinePool(clusterID, nodePoolName,
					"--replicas", "3",
					"--tuning-configs", strings.Join(allTuningConfigNames, ","),
				)
				Expect(err).ToNot(HaveOccurred())

				By("Describe nodepool")
				np, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(common.ParseTuningConfigs(np.TuningConfigs))).To(Equal(3))
				Expect(common.ParseTuningConfigs(np.TuningConfigs)).To(ContainElements(allTuningConfigNames))

				By("Update nodepool with only one tuning config")
				_, err = machinePoolService.EditMachinePool(clusterID, nodePoolName,
					"--tuning-configs", tuningConfig1Name,
				)
				Expect(err).ToNot(HaveOccurred())
				np, err = machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(common.ParseTuningConfigs(np.TuningConfigs))).To(Equal(1))
				Expect(common.ParseTuningConfigs(np.TuningConfigs)).To(ContainElements([]string{tuningConfig1Name}))

				By("Update nodepool with no tuning config")
				_, err = machinePoolService.EditMachinePool(clusterID, nodePoolName,
					"--tuning-configs", "",
				)
				Expect(err).ToNot(HaveOccurred())
				np, err = machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(common.ParseTuningConfigs(np.TuningConfigs))).To(Equal(0))
			})

		It("does support 'version' parameter on nodepool - [id:61138]",
			labels.High,
			labels.MigrationToVerify,
			labels.Exclude,
			func() {
				nodePoolName := common.GenerateRandomName("np-61138", 2)

				By("Get previous version")
				clusterVersionInfo, err := clusterService.GetClusterVersion(clusterID)
				Expect(err).ToNot(HaveOccurred())
				clusterVersion := clusterVersionInfo.RawID
				clusterChannelGroup := clusterVersionInfo.ChannelGroup
				versionList, err := versionService.ListAndReflectVersions(clusterChannelGroup, true)
				Expect(err).ToNot(HaveOccurred())

				previousVersionsList, err := versionList.FilterVersionsLowerThan(clusterVersion)
				Expect(err).ToNot(HaveOccurred())
				if previousVersionsList.Len() <= 1 {
					Skip("Skipping as no previous version is available for testing")
				}
				previousVersionsList.Sort(true)
				previousVersion := previousVersionsList.OpenShiftVersions[0].Version

				By("Check create nodepool version help parameter")
				help, err := machinePoolService.RetrieveHelpForCreate()
				Expect(err).ToNot(HaveOccurred())
				Expect(help.String()).To(ContainSubstring("--version"))

				By("Check version is displayed in list")
				nps, err := machinePoolService.ListAndReflectNodePools(clusterID)
				Expect(err).ToNot(HaveOccurred())
				for _, np := range nps.NodePools {
					Expect(np.Version).To(Not(BeEmpty()))
				}

				By("Create NP with previous version")
				_, err = machinePoolService.CreateMachinePool(clusterID, nodePoolName,
					"--replicas", defaultNodePoolReplicas,
					"--version", previousVersion,
				)
				Expect(err).ToNot(HaveOccurred())

				By("Check NodePool was correctly created")
				np, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
				Expect(err).ToNot(HaveOccurred())
				Expect(np.Version).To(Equal(previousVersion))

				By("Wait for NodePool replicas to be available")
				err = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 20*time.Minute, false, func(context.Context) (bool, error) {
					npDesc, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
					if err != nil {
						return false, err
					}
					return npDesc.CurrentReplicas == defaultNodePoolReplicas, nil
				})
				common.AssertWaitPollNoErr(err, "Replicas are not ready after 600")

				nodePoolVersion, err := versionList.FindNearestBackwardMinorVersion(clusterVersion, 1, true)
				Expect(err).ToNot(HaveOccurred())
				if nodePoolVersion != nil {
					By("Create NodePool with version minor - 1")
					nodePoolName = common.GenerateRandomName("np-61138-m1", 2)
					_, err = machinePoolService.CreateMachinePool(clusterID,
						nodePoolName,
						"--replicas", defaultNodePoolReplicas,
						"--version", nodePoolVersion.Version,
					)
					Expect(err).ToNot(HaveOccurred())
					np, err = machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(np.Version).To(Equal(nodePoolVersion.Version))
				}

				nodePoolVersion, err = versionList.FindNearestBackwardMinorVersion(clusterVersion, 2, true)
				Expect(err).ToNot(HaveOccurred())
				if nodePoolVersion != nil {
					By("Create NodePool with version minor - 2")
					nodePoolName = common.GenerateRandomName("np-61138-m1", 2)
					_, err = machinePoolService.CreateMachinePool(clusterID,
						nodePoolName,
						"--replicas", defaultNodePoolReplicas,
						"--version", nodePoolVersion.Version,
					)
					Expect(err).ToNot(HaveOccurred())
					np, err = machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(np.Version).To(Equal(nodePoolVersion.Version))
				}

				nodePoolVersion, err = versionList.FindNearestBackwardMinorVersion(clusterVersion, 3, true)
				Expect(err).ToNot(HaveOccurred())
				if nodePoolVersion != nil {
					By("Create NodePool with version minor - 3 should fail")
					_, err = machinePoolService.CreateMachinePool(clusterID,
						common.GenerateRandomName("np-61138-m3", 2),
						"--replicas", defaultNodePoolReplicas,
						"--version", nodePoolVersion.Version,
					)
					Expect(err).To(HaveOccurred())
				}
			})

		It("can validate the version parameter on nodepool creation/editing - [id:61139]",
			labels.Medium,
			func() {
				testVersionFailFunc := func(flags ...string) {
					Logger.Infof("Creating nodepool with flags %v", flags)
					output, err := machinePoolService.CreateMachinePool(clusterID, common.GenerateRandomName("np-61139", 2), flags...)
					Expect(err).To(HaveOccurred())
					textData := rosaClient.Parser.TextData.Input(output).Parse().Tip()
					Expect(textData).Should(ContainSubstring(`ERR: Expected a valid OpenShift version: A valid version number must be specified`))
					textData = rosaClient.Parser.TextData.Input(output).Parse().Output()
					Expect(textData).Should(ContainSubstring(`Valid versions:`))
				}

				By("Get cluster version")
				clusterVersionInfo, err := clusterService.GetClusterVersion(clusterID)
				Expect(err).ToNot(HaveOccurred())
				clusterVersion := clusterVersionInfo.RawID
				clusterChannelGroup := clusterVersionInfo.ChannelGroup
				clusterSemVer, err := semver.NewVersion(clusterVersion)
				Expect(err).ToNot(HaveOccurred())
				clusterVersionList, err := versionService.ListAndReflectVersions(clusterChannelGroup, true)
				Expect(err).ToNot(HaveOccurred())

				By("Create a nodepool with version greater than cluster's version should fail")
				testVersion := fmt.Sprintf("%d.%d.%d", clusterSemVer.Major()+100, clusterSemVer.Minor()+100, clusterSemVer.Patch()+100)
				testVersionFailFunc("--replicas",
					defaultNodePoolReplicas,
					"--version",
					testVersion)

				if clusterChannelGroup != rosacli.VersionChannelGroupNightly {
					versionList, err := versionService.ListAndReflectVersions(rosacli.VersionChannelGroupNightly, true)
					Expect(err).ToNot(HaveOccurred())
					lowerVersionsList, err := versionList.FilterVersionsLowerThan(clusterVersion)
					Expect(err).ToNot(HaveOccurred())
					if lowerVersionsList.Len() > 0 {
						By("Create a nodepool with version from incompatible channel group should fail")
						lowerVersionsList.Sort(true)
						testVersion := lowerVersionsList.OpenShiftVersions[0].Version
						testVersionFailFunc("--replicas",
							defaultNodePoolReplicas,
							"--version",
							testVersion)
					}
				}

				By("Create a nodepool with major different from cluster's version should fail")
				testVersion = fmt.Sprintf("%d.%d.%d", clusterSemVer.Major()-1, clusterSemVer.Minor(), clusterSemVer.Patch())
				testVersionFailFunc("--replicas",
					defaultNodePoolReplicas,
					"--version",
					testVersion)

				foundVersion, err := clusterVersionList.FindNearestBackwardMinorVersion(clusterVersion, 3, false)
				Expect(err).ToNot(HaveOccurred())
				if foundVersion != nil {
					By("Create a nodepool with minor lower than cluster's 'minor - 3' should fail")
					testVersion = foundVersion.Version
					testVersionFailFunc("--replicas",
						defaultNodePoolReplicas,
						"--version",
						testVersion)
				}

				By("Create a nodepool with non existing version should fail")
				testVersion = "24512.5632.85"
				testVersionFailFunc("--replicas",
					defaultNodePoolReplicas,
					"--version",
					testVersion)

				lowerVersionsList, err := clusterVersionList.FilterVersionsLowerThan(clusterVersion)
				Expect(err).ToNot(HaveOccurred())
				if lowerVersionsList.Len() > 0 {
					By("Edit nodepool version should fail")
					nodePoolName := common.GenerateRandomName("np-61139", 2)
					lowerVersionsList.Sort(true)
					testVersion := lowerVersionsList.OpenShiftVersions[0].Version
					_, err := machinePoolService.CreateMachinePool(clusterID, nodePoolName,
						"--replicas",
						defaultNodePoolReplicas,
						"--version",
						testVersion)
					Expect(err).ToNot(HaveOccurred())

					output, err := machinePoolService.EditMachinePool(clusterID, nodePoolName, "--version", clusterVersion)
					Expect(err).To(HaveOccurred())
					textData := rosaClient.Parser.TextData.Input(output).Parse().Tip()
					Expect(textData).Should(ContainSubstring(`ERR: Editing versions is not supported, for upgrades please use 'rosa upgrade machinepool'`))
				}
			})

		It("can list/describe/delete nodepool upgrade policies - [id:67414]",
			labels.Critical,
			func() {
				currentDateTimeUTC := time.Now().UTC()

				By("Check help(s) for node pool upgrade")
				_, err := machinePoolUpgradeService.RetrieveHelpForCreate()
				Expect(err).ToNot(HaveOccurred())
				help, err := machinePoolUpgradeService.RetrieveHelpForDescribe()
				Expect(err).ToNot(HaveOccurred())
				Expect(help.String()).To(ContainSubstring("--machinepool"))
				help, err = machinePoolUpgradeService.RetrieveHelpForList()
				Expect(err).ToNot(HaveOccurred())
				Expect(help.String()).To(ContainSubstring("--machinepool"))
				help, err = machinePoolUpgradeService.RetrieveHelpForDelete()
				Expect(err).ToNot(HaveOccurred())
				Expect(help.String()).To(ContainSubstring("--machinepool"))

				By("Get previous version")
				clusterVersionInfo, err := clusterService.GetClusterVersion(clusterID)
				Expect(err).ToNot(HaveOccurred())
				clusterVersion := clusterVersionInfo.RawID
				clusterChannelGroup := clusterVersionInfo.ChannelGroup
				versionList, err := versionService.ListAndReflectVersions(clusterChannelGroup, true)
				Expect(err).ToNot(HaveOccurred())
				previousVersionsList, err := versionList.FilterVersionsLowerThan(clusterVersion)
				Expect(err).ToNot(HaveOccurred())
				if previousVersionsList.Len() <= 1 {
					Skip("Skipping as no previous version is available for testing")
				}
				previousVersionsList.Sort(true)
				previousVersion := previousVersionsList.OpenShiftVersions[0].Version
				Logger.Infof("Using previous version %s", previousVersion)

				By("Prepare a node pool with previous version with manual upgrade")
				nodePoolManualName := common.GenerateRandomName("np-67414", 2)
				output, err := machinePoolService.CreateMachinePool(clusterID, nodePoolManualName,
					"--replicas", "2",
					"--version", previousVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Machine pool '%s' created successfully on hosted cluster '%s'", nodePoolManualName, clusterID))
				output, err = machinePoolUpgradeService.CreateManualUpgrade(clusterID, nodePoolManualName, "", "", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Upgrade successfully scheduled for the machine pool '%s' on cluster '%s'", nodePoolManualName, clusterID))

				By("Prepare a node pool with previous version with automatic upgrade")
				nodePoolAutoName := common.GenerateRandomName("np-67414", 2)
				output, err = machinePoolService.CreateMachinePool(clusterID, nodePoolAutoName,
					"--replicas", "2",
					"--version", previousVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Machine pool '%s' created successfully on hosted cluster '%s'", nodePoolAutoName, clusterID))
				output, err = machinePoolUpgradeService.CreateAutomaticUpgrade(clusterID, nodePoolAutoName, "2 5 * * *")
				Expect(err).ToNot(HaveOccurred())
				Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Upgrade successfully scheduled for the machine pool '%s' on cluster '%s'", nodePoolAutoName, clusterID))

				analyzeUpgrade := func(nodePoolName string, scheduleType string) {
					By(fmt.Sprintf("Describe node pool in json format (%s upgrade)", scheduleType))
					rosaClient.Runner.JsonFormat()
					jsonOutput, err := machinePoolService.DescribeMachinePool(clusterID, nodePoolName)
					Expect(err).To(BeNil())
					rosaClient.Runner.UnsetFormat()
					jsonData := rosaClient.Parser.JsonData.Input(jsonOutput).Parse()
					var npAvailableUpgrades []string
					for _, value := range jsonData.DigObject("version", "available_upgrades").([]interface{}) {
						npAvailableUpgrades = append(npAvailableUpgrades, fmt.Sprint(value))
					}

					By(fmt.Sprintf("Describe node pool upgrade (%s upgrade)", scheduleType))
					npuDesc, err := machinePoolUpgradeService.DescribeAndReflectUpgrade(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(npuDesc.ScheduleType).To(Equal(scheduleType))
					Expect(npuDesc.NextRun).ToNot(BeNil())
					nextRunDT, err := time.Parse("2006-01-02 15:04 UTC", npuDesc.NextRun)
					Expect(err).ToNot(HaveOccurred())
					Expect(nextRunDT.After(currentDateTimeUTC)).To(BeTrue())
					Expect(npuDesc.UpgradeState).To(BeElementOf("pending", "scheduled"))
					Expect(npuDesc.Version).To(Equal(clusterVersion))

					nextRun := npuDesc.NextRun

					By(fmt.Sprintf("Describe node pool should contain upgrade (%s upgrade)", scheduleType))
					npDesc, err := machinePoolService.DescribeAndReflectNodePool(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(npDesc.ScheduledUpgrade).To(ContainSubstring(clusterVersion))
					Expect(npDesc.ScheduledUpgrade).To(ContainSubstring(nextRun))
					Expect(npDesc.ScheduledUpgrade).To(Or(ContainSubstring("pending"), ContainSubstring("scheduled")))

					By(fmt.Sprintf("List the upgrade policies (%s upgrade)", scheduleType))
					npuList, err := machinePoolUpgradeService.ListAndReflectUpgrades(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(npuList.MachinePoolUpgrades)).To(Equal(len(npAvailableUpgrades)))
					var upgradeMPU rosacli.MachinePoolUpgrade
					for _, mpu := range npuList.MachinePoolUpgrades {
						Expect(mpu.Version).To(BeElementOf(npAvailableUpgrades))
						if mpu.Version == clusterVersion {
							upgradeMPU = mpu
						}
					}
					Expect(upgradeMPU.Notes).To(Or(ContainSubstring("pending"), ContainSubstring("scheduled")))
					Expect(upgradeMPU.Notes).To(ContainSubstring(nextRun))

					By(fmt.Sprintf("Delete the upgrade policy (%s upgrade)", scheduleType))
					output, err = machinePoolUpgradeService.DeleteUpgrade(clusterID, nodePoolName)
					Expect(err).ToNot(HaveOccurred())
					Expect(rosaClient.Parser.TextData.Input(output).Parse().Tip()).Should(ContainSubstring("Successfully canceled scheduled upgrade for machine pool '%s' for cluster '%s'", nodePoolName, clusterID))
				}

				analyzeUpgrade(nodePoolManualName, "manual")
				analyzeUpgrade(nodePoolAutoName, "automatic")
			})
	})
