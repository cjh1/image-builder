package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RunAnsiblePlaybooks executes Ansible playbooks against containers
func RunAnsiblePlaybooks(containers map[string]map[string]interface{}, inventoryPath string, verbosity int) error {
	// Create a temporary inventory file if one was not provided
	tmpInventory := false
	if inventoryPath == "" {
		tmpDir, err := os.MkdirTemp("", "ansible-inventory-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory for inventory: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		
		inventoryPath = tmpDir
		tmpInventory = true
	}
	
	// Create inventory directories and files if needed
	if tmpInventory {
		// Create host_vars directory
		hostVarsDir := filepath.Join(inventoryPath, "host_vars")
		if err := os.MkdirAll(hostVarsDir, 0755); err != nil {
			return fmt.Errorf("failed to create host_vars directory: %w", err)
		}
		
		// Create inventory file
		inventoryFile := filepath.Join(inventoryPath, "inventory")
		inventoryContent := []string{"[containers]"}
		
		for containerName := range containers {
			inventoryContent = append(inventoryContent, containerName)
		}
		
		// Add container groups
		for containerName, config := range containers {
			if groups, ok := config["ansible_groups"].([]string); ok && len(groups) > 0 {
				for _, group := range groups {
					groupExists := false
					for _, line := range inventoryContent {
						if line == fmt.Sprintf("[%s]", group) {
							groupExists = true
							break
						}
					}
					
					if !groupExists {
						inventoryContent = append(inventoryContent, fmt.Sprintf("[%s]", group))
					}
					
					// Add container to group
					groupFound := false
					for i, line := range inventoryContent {
						if line == fmt.Sprintf("[%s]", group) {
							inventoryContent = append(inventoryContent[:i+1], append([]string{containerName}, inventoryContent[i+1:]...)...)
							groupFound = true
							break
						}
					}
					
					if !groupFound {
						inventoryContent = append(inventoryContent, containerName)
					}
				}
			}
			
			// Create host vars file
			if vars, ok := config["ansible_vars"].(map[string]interface{}); ok && len(vars) > 0 {
				varsData, err := yaml.Marshal(vars)
				if err != nil {
					return fmt.Errorf("failed to marshal host vars for container %s: %w", containerName, err)
				}
				
				hostVarsFile := filepath.Join(hostVarsDir, containerName+".yml")
				if err := os.WriteFile(hostVarsFile, varsData, 0644); err != nil {
					return fmt.Errorf("failed to write host vars file for container %s: %w", containerName, err)
				}
			}
		}
		
		// Write inventory file
		if err := os.WriteFile(inventoryFile, []byte(strings.Join(inventoryContent, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to write inventory file: %w", err)
		}
	}
	
	// Run playbooks for each container
	for containerName, config := range containers {
		if playbooks, ok := config["ansible_pb"].([]string); ok && len(playbooks) > 0 {
			for _, playbook := range playbooks {
				// Build ansible-playbook command
				args := []string{
					"ansible-playbook",
					"-i", inventoryPath,
					"--connection=buildah",
					"--limit", containerName,
				}
				
				// Add verbosity flags
				for i := 0; i < verbosity; i++ {
					args = append(args, "-v")
				}
				
				// Add playbook path
				args = append(args, playbook)
				
				// Execute ansible-playbook command
				cmd := exec.Command(args[0], args[1:]...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to run ansible-playbook: %w\nStdout: %s\nStderr: %s", 
						err, stdout.String(), stderr.String())
				}
				
				fmt.Printf("Playbook %s executed successfully for container %s\n", playbook, containerName)
			}
		}
	}
	
	return nil
}
