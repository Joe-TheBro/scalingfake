package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

func chooseResourceGroupLocation() (string, error) {
	azureLocations := []map[string]string{
		{"name": "centralus", "city": "Chicago"},
		{"name": "westcentralus", "city": "Wyoming"},
		{"name": "westus2", "city": "Oregon"},
		{"name": "westus", "city": "Los Angeles"},
		{"name": "westus3", "city": "Arizona"},
		{"name": "southcentralus", "city": "Texas"},
		{"name": "canadacentral", "city": "Maine"},
		{"name": "eastus", "city": "NYC"},
	}

	fmt.Println("Choose a location for the resource group:")
	for i, location := range azureLocations {
		fmt.Printf("%d) %s (%s)\n", i+1, location["city"], location["name"])
	}
	var choice int
	_, err := fmt.Scan(&choice)
	if err != nil {
		return "", err
	}
	if choice < 1 || choice > len(azureLocations) {
		fmt.Println("Invalid choice.")
		fmt.Println("Please choose a location from the list:")
		return chooseResourceGroupLocation()
	}
	return azureLocations[choice-1]["name"], nil
}

func allocateVM() {
	const (
		resourceGroupName = "GoVMQuickstart"
		deploymentName    = "VMDeployQuickstart"
		templateFile      = "vm-quickstart-template.json"
		parametersFile    = "vm-quickstart-params.json"
	)

	type clientInfo struct {
		SubscriptionID string
		VMUsername     string
		VMPassword     string
	}

	var (
		ctx        = context.Background()
		clientData clientInfo
		authorizer autorest.Authorizer
		err        error
	)

	resourceGroupLocation, err := chooseResourceGroupLocation()
	if err != nil {
		log.Fatalf("Failed to choose resource group location: %v", err)
	}

	// Authenticate
	authorizer, err = auth.NewAuthorizerFromFile(azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		log.Fatalf("Failed to get OAuth config: %v", err)
	}

	// Load client information
	authInfo, err := readJSON(os.Getenv("AZURE_AUTH_LOCATION"))
	clientData.SubscriptionID = (*authInfo)["subscriptionId"].(string)
	clientData.VMUsername = (*authInfo)["vmUsername"].(string)
	clientData.VMPassword = (*authInfo)["vmPassword"].(string)

	// Create a resource group
	group, err := createGroup(resourceGroupName, resourceGroupLocation)
	if err != nil {
		log.Fatalf("Failed to create group: %v", err)
	}
	log.Printf("Created resource group: %s", *group.Name)

	// Deploy the VM
	vm, err := createVM(ctx, authorizer, resourceGroupName, clientData)
	if err != nil {
		log.Fatalf("Failed to create VM: %v", err)
	}

	// Get VM public IP
	publicIP, err := getPublicIP(ctx, authorizer, resourceGroupName, *vm.Name)
	if err != nil {
		log.Fatalf("Failed to get VM public IP: %v", err)
	}
	log.Printf("VM Public IP: %s", publicIP)

	// Upload files to the VM
	err = uploadFiles(publicIP, clientData.VMUsername, clientData.VMPassword, "/path/to/local/file", "/path/to/remote/file")
	if err != nil {
		log.Fatalf("Failed to upload files: %v", err)
	}

	// Run shell script on the VM
	err = runRemoteShellScript(publicIP, clientData.VMUsername, clientData.VMPassword, "/path/to/remote/script.sh")
	if err != nil {
		log.Fatalf("Failed to run shell script: %v", err)
	}

	log.Println("VM setup complete.")
}

func createGroup(resourceGroupName, resourceGroupLocation string) (*resources.Group, error) {
	// Add logic to create resource group
	return &resources.Group{}, nil
}

func createVM(ctx context.Context, authorizer autorest.Authorizer, resourceGroupName string, clientData clientInfo) (*compute.VirtualMachine, error) {
	// Add logic to create a VM and return its details
	return &compute.VirtualMachine{}, nil
}

func getPublicIP(ctx context.Context, authorizer autorest.Authorizer, resourceGroupName, vmName string) (string, error) {
	// Add logic to retrieve the public IP of the created VM
	return "xxx.xxx.xxx.xxx", nil
}
