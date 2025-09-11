package arguments

import (
	"flag"
	"fmt"
	"strings"
)

// ProcessedArgs represents the processed command line and config arguments
type ProcessedArgs struct {
	LogLevel          string
	Config            string
	LayerType         string
	PackageManager    string
	AnsibleGroups     []string
	AnsiblePlaybook   []string
	AnsibleInv        string
	AnsibleVars       map[string]string
	AnsibleVerbosity  int
	Name              string
	Parent            string
	RegistryHost      string
	RegistryNamespace string
	RegistryOptsPull  []string
	RegistryOptsPush  []string
}

// ParseCommandLine parses command line arguments
func ParseCommandLine() (*ProcessedArgs, error) {
	var args ProcessedArgs
	
	// Define command line flags
	logLevel := flag.String("log_level", "info", "Logging level")
	config := flag.String("config", "config.yaml", "Configuration file")
	layerType := flag.String("layer_type", "", "Layer type (base, ansible)")
	pkgMan := flag.String("pkg_man", "", "Package manager (dnf, zypper)")
	groupList := flag.String("group_list", "", "Ansible group list (comma separated)")
	playbooks := flag.String("pb", "", "Ansible playbooks (comma separated)")
	inventory := flag.String("inventory", "", "Ansible inventory")
	vars := flag.String("vars", "", "Ansible variables (key=value,key2=value2)")
	ansibleVerbosity := flag.Int("ansible_verbosity", 0, "Ansible verbosity (0-4)")
	name := flag.String("name", "image", "Image name")
	parent := flag.String("parent", "", "Parent image")
	registryOptsPull := flag.String("registry_opts_pull", "", "Registry options for pull (comma separated)")
	registryOptsPush := flag.String("registry_opts_push", "", "Registry options for push (comma separated)")
	registryHost := flag.String("registry_host", "", "Registry hostname")
	registryNamespace := flag.String("registry_namespace", "", "Registry namespace")
	
	flag.Parse()
	
	// Process arguments
	args.LogLevel = *logLevel
	args.Config = *config
	args.LayerType = *layerType
	args.PackageManager = *pkgMan
	args.Name = *name
	args.Parent = *parent
	args.AnsibleVerbosity = *ansibleVerbosity
	
	// Process lists
	if *groupList != "" {
		args.AnsibleGroups = strings.Split(*groupList, ",")
	}
	
	if *playbooks != "" {
		args.AnsiblePlaybook = strings.Split(*playbooks, ",")
	}
	
	if *registryOptsPull != "" {
		args.RegistryOptsPull = strings.Split(*registryOptsPull, ",")
	}
	
	if *registryOptsPush != "" {
		args.RegistryOptsPush = strings.Split(*registryOptsPush, ",")
	}
	
	args.RegistryHost = *registryHost
	args.RegistryNamespace = *registryNamespace
	args.AnsibleInv = *inventory
	
	// Process vars
	args.AnsibleVars = make(map[string]string)
	if *vars != "" {
		for _, pair := range strings.Split(*vars, ",") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				args.AnsibleVars[kv[0]] = kv[1]
			}
		}
	}
	
	return &args, validateArgs(&args)
}

// ProcessArgs processes command line arguments and configuration options
func ProcessArgs(args *ProcessedArgs, configOptions map[string]interface{}) (*ProcessedArgs, error) {
	// If layer_type not specified in args, get from config
	if args.LayerType == "" {
		if layerType, ok := configOptions["layer_type"].(string); ok {
			args.LayerType = layerType
		}
	}
	
	// Validate that layer_type is provided
	if args.LayerType == "" {
		return nil, fmt.Errorf("'layer_type' required in config file or as an argument")
	}
	
	// Process parent image
	if args.Parent == "" {
		if parent, ok := configOptions["parent"].(string); ok {
			args.Parent = parent
		}
	}
	
	// If base layer, package manager is required
	if args.LayerType == "base" {
		if args.PackageManager == "" {
			if pkgMan, ok := configOptions["pkg_manager"].(string); ok {
				args.PackageManager = pkgMan
			}
		}
		
		if args.PackageManager == "" {
			return nil, fmt.Errorf("'pkg_man' required when 'layer_type' is base")
		}
	}
	
	// Process name field
	if args.Name == "image" { // Default value
		if name, ok := configOptions["name"].(string); ok {
			args.Name = name
		}
	}
	
	// If ansible layer, process ansible options
	if args.LayerType == "ansible" {
		// Process ansible groups from config if not provided in args
		if len(args.AnsibleGroups) == 0 {
			if groups, ok := configOptions["groups"].([]interface{}); ok {
				for _, group := range groups {
					if groupStr, ok := group.(string); ok {
						args.AnsibleGroups = append(args.AnsibleGroups, groupStr)
					}
				}
			}
		}
		
		// Process ansible playbooks from config if not provided in args
		if len(args.AnsiblePlaybook) == 0 {
			if playbooks, ok := configOptions["playbooks"].([]interface{}); ok {
				for _, pb := range playbooks {
					if pbStr, ok := pb.(string); ok {
						args.AnsiblePlaybook = append(args.AnsiblePlaybook, pbStr)
					}
				}
			}
		}
		
		// Process inventory from config if not provided in args
		if args.AnsibleInv == "" {
			if inv, ok := configOptions["inventory"].(string); ok {
				args.AnsibleInv = inv
			}
		}
		
		// Process vars from config if not provided in args
		if len(args.AnsibleVars) == 0 {
			if vars, ok := configOptions["vars"].(map[string]interface{}); ok {
				for k, v := range vars {
					args.AnsibleVars[k] = fmt.Sprintf("%v", v)
				}
			}
		}
		
		// Validate ansible verbosity
		if args.AnsibleVerbosity < 0 || args.AnsibleVerbosity > 4 {
			return nil, fmt.Errorf("invalid ansible_verbosity: must be between 0 and 4")
		}
	}
	
	return args, nil
}

// validateArgs performs basic validation on the provided arguments
func validateArgs(args *ProcessedArgs) error {
	// Add validation logic as needed
	return nil
}
