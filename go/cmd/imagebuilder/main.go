package main

import (
	"log"
	"os"

	"github.com/OpenCHAMI/image-builder/go/pkg/arguments"
	"github.com/OpenCHAMI/image-builder/go/pkg/config"
	"github.com/OpenCHAMI/image-builder/go/pkg/layer"
	"github.com/containers/storage/pkg/reexec"
)

func init() {
	// Initialize reexec package
	if reexec.Init() {
		os.Exit(0)
	}
}

func main() {
	// Configure logging
	log.SetPrefix("[MAIN] ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	
	// Parse command-line arguments
	args, err := arguments.ParseCommandLine()
	if err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	}
	
	// Load configuration file
	cfg, err := config.LoadConfig(args.Config)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	
	// Process arguments with config options
	processedArgs, err := arguments.ProcessArgs(args, cfg.Options)
	if err != nil {
		log.Fatalf("Error processing arguments: %v", err)
	}
	
	// Convert to layer.ProcessedArgs
	layerArgs := &layer.ProcessedArgs{
		Name:            processedArgs.Name,
		Parent:          processedArgs.Parent,
		LayerType:       processedArgs.LayerType,
		PkgMan:          processedArgs.PackageManager,
		RegistryOptsPull: processedArgs.RegistryOptsPull,
		AnsibleGroups:   processedArgs.AnsibleGroups,
		AnsiblePlaybooks: processedArgs.AnsiblePlaybook,
		AnsibleInventory: processedArgs.AnsibleInv,
	}
	
	// Convert ansible vars map
	ansibleVars := make(map[string]interface{})
	for k, v := range processedArgs.AnsibleVars {
		ansibleVars[k] = v
	}
	layerArgs.AnsibleVars = ansibleVars
	
	// Add registry options
	layerArgs.RegistryHost = processedArgs.RegistryHost
	layerArgs.RegistryNamespace = processedArgs.RegistryNamespace
	layerArgs.RegistryOptsPush = processedArgs.RegistryOptsPush
	layerArgs.AnsibleVerbosity = processedArgs.AnsibleVerbosity
	
	// Create and build the layer
	builder := layer.NewLayer(layerArgs, cfg)
	
	// Build the layer
	if err := builder.BuildLayer(); err != nil {
		log.Fatalf("Error building %s layer: %v", processedArgs.LayerType, err)
	}
	
	log.Println("Build completed successfully")
}
