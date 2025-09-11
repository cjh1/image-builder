package installer

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
	"github.com/containers/buildah"
	"github.com/containers/storage"
)

// Installer handles package installation and repository management
type Installer struct {
	PkgMan      string
	ContainerID string
	MountPoint  string
	Logger      *log.Logger
	TempDir     string
}

// runCommandInContainer runs a command inside a container using buildah API
func (i *Installer) runCommandInContainer(cmd []string) (string, error) {
	i.Logger.Printf("Running command in container using buildah API: %v", cmd)
	
	// Get buildah store
	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return "", fmt.Errorf("failed to get default store options: %w", err)
	}
	
	store, err := storage.GetStore(storeOpts)
	if err != nil {
		return "", fmt.Errorf("failed to get buildah store: %w", err)
	}
	
	// Get the builder for this container
	builder, err := buildah.OpenBuilder(store, i.ContainerID)
	if err != nil {
		return "", fmt.Errorf("failed to open builder for container %s: %w", i.ContainerID, err)
	}
	
	// Prepare command execution
	var stdout, stderr bytes.Buffer
	options := buildah.RunOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		AddCapabilities: []string{"CAP_AUDIT_WRITE", "CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", 
			"CAP_FSETID", "CAP_KILL", "CAP_MKNOD", "CAP_NET_BIND_SERVICE", 
			"CAP_SETFCAP", "CAP_SETGID", "CAP_SETPCAP", "CAP_SETUID", "CAP_SYS_CHROOT"},
	}
	
	// Execute the command in the container
	if err := builder.Run(cmd, options); err != nil {
		return stdout.String(), fmt.Errorf("failed to run command in container: %w\nCommand: %v\nStdout: %s\nStderr: %s", 
			err, cmd, stdout.String(), stderr.String())
	}
	
	if stderr.Len() > 0 {
		i.Logger.Printf("Command stderr: %s", stderr.String())
	}
	
	return stdout.String(), nil
}

// NewInstaller creates a new installer for the specified package manager
func NewInstaller(pkgMan, containerID, mountPoint string) (*Installer, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("[INSTALLER-%s] ", pkgMan), log.LstdFlags)
	
	// Create temp directory for package manager logs, cache, etc.
	tmpDir, err := os.MkdirTemp("", "image-build-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	// Ensure /tmp exists in the mount
	if err := utils.CreateDir(filepath.Join(mountPoint, "tmp")); err != nil {
		return nil, fmt.Errorf("failed to create /tmp in container mount: %w", err)
	}
	
	// For DNF, create log directory
	if pkgMan == "dnf" {
		if err := utils.CreateDir(filepath.Join(tmpDir, "dnf/log")); err != nil {
			return nil, fmt.Errorf("failed to create dnf log directory: %w", err)
		}
	}
	
	logger.Printf("Temporary directory created at %s", tmpDir)
	
	return &Installer{
		PkgMan:      pkgMan,
		ContainerID: containerID,
		MountPoint:  mountPoint,
		Logger:      logger,
		TempDir:     tmpDir,
	}, nil
}

// InstallRepos installs repositories in the container
func (i *Installer) InstallRepos(repos []map[string]interface{}, repoDest, proxy string) error {
	if len(repos) == 0 {
		i.Logger.Println("No repositories specified to install")
		return nil
	}
	
	i.Logger.Printf("Installing repositories to %s", i.ContainerID)
	
	// Create repo destination directory if it doesn't exist
	repoDir := filepath.Join(i.MountPoint, repoDest)
	if err := utils.CreateDir(repoDir); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}
	
	for _, repo := range repos {
		alias, ok := repo["alias"].(string)
		if !ok {
			return fmt.Errorf("repository missing alias")
		}
		
		url, ok := repo["url"].(string)
		if !ok {
			return fmt.Errorf("repository %s missing url", alias)
		}
		
		i.Logger.Printf("Installing repo: %s: %s", alias, url)
		
		switch i.PkgMan {
		case "zypper":
			// For zypper, we need to run the command directly in the container
			args := []string{"zypper", "-D", repoDest, "addrepo", "-f", "-p"}
			
			if priority, ok := repo["priority"].(int); ok {
				args = append(args, fmt.Sprintf("%d", priority))
			} else {
				args = append(args, "90")
			}
			
			args = append(args, url, alias)
			
			output, err := i.runCommandInContainer(args)
			if err != nil {
				return fmt.Errorf("failed to add zypper repo %s: %w", alias, err)
			}
			
			i.Logger.Printf("Added repo output: %s", output)
			
			// Handle GPG key if specified
			if gpgKey, ok := repo["gpg_key"].(string); ok && gpgKey != "" {
				i.Logger.Printf("Adding GPG key for repo %s", alias)
				
				// TODO: Import GPG key using zypper
			}
		case "dnf":
			// Create the repo file directly in the mount point
			repoFile := filepath.Join(repoDir, fmt.Sprintf("%s.repo", alias))
			
			content := fmt.Sprintf("[%s]\nname=%s\nbaseurl=%s\nenabled=1\ngpgcheck=0\n", 
				alias, alias, url)
			
			if priority, ok := repo["priority"].(int); ok {
				content += fmt.Sprintf("priority=%d\n", priority)
			}
			
			if err := os.WriteFile(repoFile, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write repo file %s: %w", repoFile, err)
			}
			
			// Handle GPG key if specified
			if gpgKey, ok := repo["gpg_key"].(string); ok && gpgKey != "" {
				i.Logger.Printf("Adding GPG key for repo %s", alias)
				
				// TODO: Import GPG key using rpm
			}
		default:
			return fmt.Errorf("unsupported package manager: %s", i.PkgMan)
		}
	}
	
	return nil
}

// InstallPackages installs the specified packages
func (i *Installer) InstallPackages(packages []string) error {
	if len(packages) == 0 {
		i.Logger.Println("No packages specified to install")
		return nil
	}
	
	i.Logger.Printf("Installing %d packages", len(packages))
	
	var args []string
	switch i.PkgMan {
	case "zypper":
		args = []string{"zypper", "-n", "install", "--no-recommends"}
		args = append(args, packages...)
	case "dnf":
		args = []string{"dnf", "-y", "install"}
		args = append(args, packages...)
	default:
		return fmt.Errorf("unsupported package manager: %s", i.PkgMan)
	}
	
	i.Logger.Printf("Running command in container: %v", args)
	output, err := i.runCommandInContainer(args)
	if err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	
	i.Logger.Printf("Command output: %s", output)
	return nil
}

// RemovePackages removes the specified packages
func (i *Installer) RemovePackages(packages []string) error {
	if len(packages) == 0 {
		i.Logger.Println("No packages specified to remove")
		return nil
	}
	
	i.Logger.Printf("Removing %d packages", len(packages))
	
	var args []string
	switch i.PkgMan {
	case "zypper":
		args = []string{"zypper", "remove", "-y"}
		args = append(args, packages...)
	case "dnf":
		args = []string{"dnf", "-y", "remove"}
		args = append(args, packages...)
	default:
		return fmt.Errorf("unsupported package manager: %s", i.PkgMan)
	}
	
	i.Logger.Printf("Running command in container: %v", args)
	output, err := i.runCommandInContainer(args)
	if err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}
	
	i.Logger.Printf("Command output: %s", output)
	return nil
}

// InstallGroups installs the specified package groups
func (i *Installer) InstallGroups(groups []string) error {
	if len(groups) == 0 {
		i.Logger.Println("No package groups specified to install")
		return nil
	}
	
	i.Logger.Printf("Installing %d package groups", len(groups))
	
	var args []string
	switch i.PkgMan {
	case "dnf":
		args = []string{"dnf", "-y", "groupinstall"}
		args = append(args, groups...)
	case "zypper":
		// Zypper doesn't have a direct equivalent to dnf's groupinstall
		i.Logger.Println("Package groups not directly supported by zypper, installing as patterns")
		args = []string{"zypper", "install", "-y", "--no-recommends", "-t", "pattern"}
		args = append(args, groups...)
	default:
		return fmt.Errorf("unsupported package manager: %s", i.PkgMan)
	}
	
	i.Logger.Printf("Running command in container: %v", args)
	output, err := i.runCommandInContainer(args)
	if err != nil {
		return fmt.Errorf("failed to install package groups: %w", err)
	}
	
	i.Logger.Printf("Command output: %s", output)
	return nil
}

// Cleanup removes temporary files
func (i *Installer) Cleanup() {
	if i.TempDir != "" {
		i.Logger.Printf("Cleaning up temporary directory %s", i.TempDir)
		os.RemoveAll(i.TempDir)
	}
}
