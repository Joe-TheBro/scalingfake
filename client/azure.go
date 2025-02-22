package main

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/security"
	"github.com/charmbracelet/log"
)

// https://learn.microsoft.com/en-us/entra/identity-platform/howto-create-service-principal-portal
// Subscription ID AZURE_SUBSCRIPTION_ID
// Client ID AZURE_CLIENT_ID
// Client Secret AZURE_CLIENT_SECRET
// Tenant ID AZURE_TENANT_ID

var subscriptionId string

const (
	resourceGroupName = "deepfake-resource-group"
	vmName            = "deepfake-vm"
	vnetName          = "deepfake-vnet"
	subnetName        = "deepfake-subnet"
	nsgName           = "deepfake-nsg"
	nicName           = "deepfake-nic"
	diskName          = "deepfake-disk"
	publicIPName      = "deepfake-public-ip"
)

var (
	location = "eastus3"

	resourcesClientFactory *armresources.ClientFactory
	computeClientFactory   *armcompute.ClientFactory
	networkClientFactory   *armnetwork.ClientFactory

	resourceGroupClient *armresources.ResourceGroupsClient

	virtualNetworksClient   *armnetwork.VirtualNetworksClient
	subnetsClient           *armnetwork.SubnetsClient
	securityGroupsClient    *armnetwork.SecurityGroupsClient
	publicIPAddressesClient *armnetwork.PublicIPAddressesClient
	interfacesClient        *armnetwork.InterfacesClient

	virtualMachinesClient *armcompute.VirtualMachinesClient
	disksClient           *armcompute.DisksClient
)

func createVM() *armnetwork.PublicIPAddress {
	conn, err := connectionAzure()
	if err != nil {
		log.Fatal("cannot connect to Azure:%+v", err)
	}
	ctx := context.Background()

	resourcesClientFactory, err = armresources.NewClientFactory(subscriptionId, conn, nil)
	if err != nil {
		log.Fatal(err)
	}
	resourceGroupClient = resourcesClientFactory.NewResourceGroupsClient()

	networkClientFactory, err = armnetwork.NewClientFactory(subscriptionId, conn, nil)
	if err != nil {
		log.Fatal(err)
	}
	virtualNetworksClient = networkClientFactory.NewVirtualNetworksClient()
	subnetsClient = networkClientFactory.NewSubnetsClient()
	securityGroupsClient = networkClientFactory.NewSecurityGroupsClient()
	publicIPAddressesClient = networkClientFactory.NewPublicIPAddressesClient()
	interfacesClient = networkClientFactory.NewInterfacesClient()

	computeClientFactory, err = armcompute.NewClientFactory(subscriptionId, conn, nil)
	if err != nil {
		log.Fatal(err)
	}
	virtualMachinesClient = computeClientFactory.NewVirtualMachinesClient()
	disksClient = computeClientFactory.NewDisksClient()

	log.Info("Starting to create virtual machine...")
	// check if resource group exists, this handles when the program exits unexpectedly and cleanup cannot be called
	
	_, err = resourceGroupClient.Get(ctx, resourceGroupName, nil)
	if err == nil {
		log.Info("Resource group already exists: %s", resourceGroupName)
		log.Info("Deleting existing resource group...")
		err = deleteResourceGroup(ctx)
	}
	
	//* Anything below this point SHOULD not have an existing vm, network, etc....

	resourceGroup, err := createResourceGroup(ctx)
	if err != nil {
		log.Fatal("cannot create resource group:%+v", err)
	}
	log.Info("Created resource group: %s", *resourceGroup.ID)

	virtualNetwork, err := createVirtualNetwork(ctx)
	if err != nil {
		log.Fatal("cannot create virtual network:%+v", err)
	}
	log.Info("Created virtual network: %s", *virtualNetwork.ID)

	subnet, err := createSubnets(ctx)
	if err != nil {
		log.Fatal("cannot create subnet:%+v", err)
	}
	log.Info("Created subnet: %s", *subnet.ID)

	publicIP, err := createPublicIP(ctx)
	if err != nil {
		log.Fatal("cannot create public IP address:%+v", err)
	}
	log.Info("Created public IP address: %s", *publicIP.ID)

	// network security group
	nsg, err := createNetworkSecurityGroup(ctx)
	if err != nil {
		log.Fatal("cannot create network security group:%+v", err)
	}
	log.Info("Created network security group: %s", *nsg.ID)

	netWorkInterface, err := createNetWorkInterface(ctx, *subnet.ID, *publicIP.ID, *nsg.ID)
	if err != nil {
		log.Fatal("cannot create network interface:%+v", err)
	}
	log.Info("Created network interface: %s", *netWorkInterface.ID)

	networkInterfaceID := netWorkInterface.ID
	virtualMachine, err := createVirtualMachine(ctx, *networkInterfaceID)
	if err != nil {
		log.Fatal("cannot create virual machine:%+v", err)
	}
	log.Info("Created network virual machine: %s", *virtualMachine.ID)

	log.Info("Virtual machine created successfully!")

	return publicIP
}

func cleanup() {
	ctx := context.Background()

	log.Info("start deleting virtual machine...")
	err := deleteVirtualMachine(ctx)
	if err != nil {
		log.Fatal("cannot delete virtual machine:%+v", err)
	}
	log.Info("deleted virtual machine")

	err = deleteDisk(ctx)
	if err != nil {
		log.Fatal("cannot delete disk:%+v", err)
	}
	log.Info("deleted disk")

	err = deleteNetWorkInterface(ctx)
	if err != nil {
		log.Fatal("cannot delete network interface:%+v", err)
	}
	log.Info("deleted network interface")

	err = deleteNetworkSecurityGroup(ctx)
	if err != nil {
		log.Fatal("cannot delete network security group:%+v", err)
	}
	log.Info("deleted network security group")

	err = deletePublicIP(ctx)
	if err != nil {
		log.Fatal("cannot delete public IP address:%+v", err)
	}
	log.Info("deleted public IP address")

	err = deleteSubnets(ctx)
	if err != nil {
		log.Fatal("cannot delete subnet:%+v", err)
	}
	log.Info("deleted subnet")

	err = deleteVirtualNetWork(ctx)
	if err != nil {
		log.Fatal("cannot delete virtual network:%+v", err)
	}
	log.Info("deleted virtual network")

	err = deleteResourceGroup(ctx)
	if err != nil {
		log.Fatal("cannot delete resource group:%+v", err)
	}
	log.Info("deleted resource group")
	log.Info("success deleted virtual machine.")
}

func connectionAzure() (azcore.TokenCredential, error) {
	// Load environment variables from .env file
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatal("Error loading .env file", err)
	// }

	// Retrieve Azure credentials from environment variables
	// subscriptionId = os.Getenv("AZURE_SUBSCRIPTION_ID")
	// tenantID := os.Getenv("AZURE_TENANT_ID")
	// clientID := os.Getenv("AZURE_CLIENT_ID")
	// clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	// Ensure all required environment variables are set
	// if len(subscriptionId) == 0 || len(tenantID) == 0 || len(clientID) == 0 || len(clientSecret) == 0 {
	// 	log.Fatal("AZURE_TENANT_ID, AZURE_CLIENT_ID, and AZURE_CLIENT_SECRET must be set in the environment variables.")
	// }
	// cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	// if err != nil {
	// 	return nil, err
	// }
	// return cred, nil
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		log.Fatalf("Failed to authenticate with Azure CLI: %v", err)
	}
	return cred, nil
}

func createResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error) {

	parameters := armresources.ResourceGroup{
		Location: to.Ptr(location),
	}

	resp, err := resourceGroupClient.CreateOrUpdate(ctx, resourceGroupName, parameters, nil)
	if err != nil {
		return nil, err
	}

	return &resp.ResourceGroup, nil
}

func deleteResourceGroup(ctx context.Context) error {

	pollerResponse, err := resourceGroupClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func createVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {

	parameters := armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					to.Ptr("10.1.1.0/24"), // holds 256 IP addresses 10.1.1.255
				},
			},
			//Subnets: []*armnetwork.Subnet{
			//	{
			//		Name: to.Ptr(subnetName+"3"),
			//		Properties: &armnetwork.SubnetPropertiesFormat{
			//			AddressPrefix: to.Ptr("10.1.0.0/24"),
			//		},
			//	},
			//},
		},
	}

	pollerResponse, err := virtualNetworksClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.VirtualNetwork, nil
}

func deleteVirtualNetWork(ctx context.Context) error {

	pollerResponse, err := virtualNetworksClient.BeginDelete(ctx, resourceGroupName, vnetName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func createSubnets(ctx context.Context) (*armnetwork.Subnet, error) {

	parameters := armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.1.1.0/24"),
		},
	}

	pollerResponse, err := subnetsClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, subnetName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.Subnet, nil
}

func deleteSubnets(ctx context.Context) error {

	pollerResponse, err := subnetsClient.BeginDelete(ctx, resourceGroupName, vnetName, subnetName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func createNetworkSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	parameters := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				// Inbound SSH/SCP port 22
				{
					Name: to.Ptr("inbound_22"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("22"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](100),
						Description:              to.Ptr("Allow inbound SSH/SCP traffic on port 22"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				// Outbound SSH/SCP port 22
				{
					Name: to.Ptr("outbound_22"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("22"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](100),
						Description:              to.Ptr("Allow outbound SSH/SCP traffic on port 22"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				// Inbound HTTP port 80
				{
					Name: to.Ptr("inbound_80"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("80"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](110),
						Description:              to.Ptr("Allow inbound HTTP traffic on port 80"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				// Outbound HTTP port 80
				{
					Name: to.Ptr("outbound_80"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("80"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](110),
						Description:              to.Ptr("Allow outbound HTTP traffic on port 80"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				// Inbound HTTPS port 443
				{
					Name: to.Ptr("inbound_443"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("443"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](120),
						Description:              to.Ptr("Allow inbound HTTPS traffic on port 443"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				// Outbound HTTPS port 443
				{
					Name: to.Ptr("outbound_443"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("443"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](120),
						Description:              to.Ptr("Allow outbound HTTPS traffic on port 443"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				// Inbound TCP port 9001
				{
					Name: to.Ptr("inbound_9001"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("9001"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](130),
						Description:              to.Ptr("Allow inbound TCP traffic on port 9001"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				// Outbound TCP port 9001
				{
					Name: to.Ptr("outbound_9001"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRange:     to.Ptr("9001"),
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](130),
						Description:              to.Ptr("Allow outbound TCP traffic on port 9001"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				// Inbound WebRTC UDP ports 16384-32767
				{
					Name: to.Ptr("inbound_webrtc"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRanges:    []*string{to.Ptr("16384-32767")},
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolUDP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](140),
						Description:              to.Ptr("Allow inbound WebRTC traffic on UDP ports 16384-32767"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				// Outbound WebRTC UDP ports 16384-32767
				{
					Name: to.Ptr("outbound_webrtc"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
						DestinationPortRanges:    []*string{to.Ptr("16384-32767")},
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolUDP),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr[int32](140),
						Description:              to.Ptr("Allow outbound WebRTC traffic on UDP ports 16384-32767"),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
			},
		},
	}

	pollerResponse, err := securityGroupsClient.BeginCreateOrUpdate(ctx, resourceGroupName, nsgName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.SecurityGroup, nil
}

func deleteNetworkSecurityGroup(ctx context.Context) error {

	pollerResponse, err := securityGroupsClient.BeginDelete(ctx, resourceGroupName, nsgName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func createPublicIP(ctx context.Context) (*armnetwork.PublicIPAddress, error) {

	parameters := armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic), // Static or Dynamic
		},
	}

	pollerResponse, err := publicIPAddressesClient.BeginCreateOrUpdate(ctx, resourceGroupName, publicIPName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.PublicIPAddress, err
}

func deletePublicIP(ctx context.Context) error {

	pollerResponse, err := publicIPAddressesClient.BeginDelete(ctx, resourceGroupName, publicIPName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func createNetWorkInterface(ctx context.Context, subnetID string, publicIPID string, networkSecurityGroupID string) (*armnetwork.Interface, error) {

	parameters := armnetwork.Interface{
		Location: to.Ptr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			//NetworkSecurityGroup:
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipConfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(subnetID),
						},
						PublicIPAddress: &armnetwork.PublicIPAddress{
							ID: to.Ptr(publicIPID),
						},
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: to.Ptr(networkSecurityGroupID),
			},
		},
	}

	pollerResponse, err := interfacesClient.BeginCreateOrUpdate(ctx, resourceGroupName, nicName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.Interface, err
}

func deleteNetWorkInterface(ctx context.Context) error {

	pollerResponse, err := interfacesClient.BeginDelete(ctx, resourceGroupName, nicName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func createVirtualMachine(ctx context.Context, networkInterfaceID string) (*armcompute.VirtualMachine, error) {
	//require ssh key for authentication on linux
	err := security.GenerateSSHKey()
	if err != nil {
		return nil, err
	}

	sshPublicKeyPath := config.SSHPublicKeyPath
	var sshBytes []byte
	_, err = os.Stat(sshPublicKeyPath)
	if err == nil {
		sshBytes, err = os.ReadFile(sshPublicKeyPath)
		if err != nil {
			return nil, err
		}
	}

	parameters := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Identity: &armcompute.VirtualMachineIdentity{
			Type: to.Ptr(armcompute.ResourceIdentityTypeNone),
		},
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					// search image reference
					// az vm image list --output table
					// Offer:     to.Ptr("WindowsServer"),
					// Publisher: to.Ptr("MicrosoftWindowsServer"),
					// SKU:       to.Ptr("2019-Datacenter"),
					// Version:   to.Ptr("latest"),
					//require ssh key for authentication on linux
					Offer:     to.Ptr("UbuntuServer"),
					Publisher: to.Ptr("Canonical"),
					SKU:       to.Ptr("24.04.1-LTS"),
					Version:   to.Ptr("latest"),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(diskName),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					Caching:      to.Ptr(armcompute.CachingTypesReadWrite),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS), // OSDisk type Standard/Premium HDD/SSD
					},
					DiskSizeGB: to.Ptr[int32](128), // default 127G
				},
			},
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_NC24ads_A100_v4")), // VM size include vCPUs,RAM,Data Disks,Temp storage.
			},
			OSProfile: &armcompute.OSProfile{ //
				ComputerName:  to.Ptr("deepfake-vm"),
				AdminUsername: to.Ptr(config.SSHUsername),
				// AdminPassword: to.Ptr(""), //! Replace with SSH key
				//require ssh key for authentication on linux
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/root/.ssh/authorized_keys"),
								KeyData: to.Ptr(string(sshBytes)),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(networkInterfaceID),
					},
				},
			},
			SecurityProfile: &armcompute.SecurityProfile{
				UefiSettings: &armcompute.UefiSettings{
					SecureBootEnabled: to.Ptr(false),
				},
			},
		},
	}

	pollerResponse, err := virtualMachinesClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, parameters, nil)
	if err != nil {
		return nil, err
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.VirtualMachine, nil
}

func deleteVirtualMachine(ctx context.Context) error {

	pollerResponse, err := virtualMachinesClient.BeginDelete(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func deleteDisk(ctx context.Context) error {

	pollerResponse, err := disksClient.BeginDelete(ctx, resourceGroupName, diskName, nil)
	if err != nil {
		return err
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func allocateVM() *armnetwork.PublicIPAddress {
	//create virtual machine
	publicIP := createVM()
	defer cleanup()
	return publicIP
}

// func main() {
// 	subscriptionId = os.Getenv("AZURE_SUBSCRIPTION_ID")
// 	if len(subscriptionId) == 0 {
// 		log.Fatal("AZURE_SUBSCRIPTION_ID is not set.")
// 	}
// 	//create virtual machine
// 	createVM()

// 	keepResource := os.Getenv("KEEP_RESOURCE")
// 	if len(keepResource) == 0 {
// 		//delete virtual machine
// 		cleanup()
// 	}
// }