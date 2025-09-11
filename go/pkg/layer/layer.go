package layer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
	"github.com/OpenCHAMI/image-builder/go/pkg/installer"
	"github.com/OpenCHAMI/image-builder/go/pkg/utils"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

// Layer handles image layer building operations
type Layer struct {
	Args   *ProcessedArgs
	Config *config.ImageConfig
	Logger *log.Logger
}

// ProcessedArgs contains the processed arguments for the layer builder
type ProcessedArgs struct {
	Name              string
	Parent            string
	LayerType         string
	PkgMan            string
	RegistryHost      string
	RegistryNamespace string
	RegistryOptsPull  []string
	RegistryOptsPush  []string
	AnsibleGroups     []string
	AnsiblePlaybooks  []string
	AnsibleInventory  string
	AnsibleVars       map[string]interface{}
	AnsibleVerbosity  int
}

// NewLayer creates a new layer builder
func NewLayer(args *ProcessedArgs, cfg *config.ImageConfig) *Layer {
	return &Layer{
		Args:   args,
		Config: cfg,
		Logger: log.New(os.Stdout, "[LAYER] ", log.LstdFlags),
	}
}

// BuildBase builds a base layer using buildah API
func (l *Layer) BuildBase() error {
	// Make sure we're running as root for buildah
	unshare.MaybeReexecUsingUserNamespace(false)
	
	dtString := time.Now().Format("20060102150405")
	containerName := l.Args.Name + "-" + dtString
	
	l.Logger.Printf("Building base layer: %s from parent %s", containerName, l.Args.Parent)
	
	// Create buildah context
	ctx := context.Background()
	
	// Get buildah store
	storeOpts, err := storage.DefaultStoreOptions()
	if err != nil {
		return fmt.Errorf("failed to get default store options: %w", err)
	}
	
	store, err := storage.GetStore(storeOpts)
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}
	
	// Setup pull options for registry auth, etc.
	systemContext := &types.SystemContext{}
	
	// Create a container from the parent image
	l.Logger.Printf("Creating container from image: %s", l.Args.Parent)
	
	buildahOpts := buildah.BuilderOptions{
		FromImage:           l.Args.Parent,
		Container:           containerName,
		PullPolicy:          buildah.PullIfMissing,
		SignaturePolicyPath: "",
		SystemContext:       systemContext,
	}
	
	builder, err := buildah.NewBuilder(ctx, store, buildahOpts)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	
	// Set up cleanup in case of errors
	defer func() {
		if err != nil {
			_ = builder.Delete()
		}
	}()
	
	// Mount the container
	mountPoint, err := builder.Mount("")
	if err != nil {
		return fmt.Errorf("failed to mount container: %w", err)
	}
	
	// Set up unmount on function exit
	defer func() {
		_ = builder.Unmount()
	}()
	
	l.Logger.Printf("Container mounted at: %s", mountPoint)
	
	// Detect OS
	osVersion, _, err := utils.GetOS(mountPoint)
	if err != nil {
		return fmt.Errorf("failed to detect OS: %w", err)
	}
	l.Logger.Printf("Detected OS: %s", osVersion)
	
	// Create installer for package management
	inst, err := installer.NewInstaller(l.Args.PkgMan, containerName, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}
	defer inst.Cleanup()
	
	// Install repositories if specified
	repositories := make([]map[string]interface{}, 0)
	for _, repo := range l.Config.Repositories {
		repoMap := map[string]interface{}{
			"alias": repo.Alias,
			"url":   repo.URL,
		}
		if repo.Priority != 0 {
			repoMap["priority"] = repo.Priority
		}
		if repo.GPGKey != "" {
			repoMap["gpg_key"] = repo.GPGKey
		}
		repositories = append(repositories, repoMap)
	}
	
	if err := inst.InstallRepos(repositories, "/etc/yum.repos.d/", ""); err != nil {
		return fmt.Errorf("failed to install repositories: %w", err)
	}
	
	// Install packages
	if err := inst.InstallPackages(l.Config.Packages); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	
	// Install package groups
	if err := inst.InstallGroups(l.Config.PackageGroups); err != nil {
		return fmt.Errorf("failed to install package groups: %w", err)
	}
	
	// Remove specified packages
	if err := inst.RemovePackages(l.Config.RemovePackages); err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}
	
	// Run commands
	for _, cmd := range l.Config.Commands {
		l.Logger.Printf("Running command: %s", cmd)
		
		// Create a shell script to run the command
		scriptContent := "#!/bin/sh\nset -e\n" + cmd + "\n"
		scriptPath := filepath.Join(mountPoint, "tmp", "run_command.sh")
		
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("failed to create command script: %w", err)
		}
		
		// Run the command using buildah run
		runOptions := buildah.RunOptions{
			Hostname:  "",
			Terminal:  buildah.WithoutTerminal,
			Stdin:     nil,
			Stdout:    os.Stdout,
			Stderr:    os.Stderr,
		}
		
		if err := builder.Run([]string{"/tmp/run_command.sh"}, runOptions); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		
		// Clean up
		os.Remove(scriptPath)
	}
	
	// Copy files
	for _, copyFile := range l.Config.CopyFiles {
		src, hasSrc := copyFile["src"]
		dest, hasDest := copyFile["dest"]
		
		if !hasSrc || !hasDest {
			continue
		}
		
		l.Logger.Printf("Copying file from %s to %s", src, dest)
		
		// Resolve destination path
		destPath := filepath.Join(mountPoint, strings.TrimPrefix(dest, "/"))
		destDir := filepath.Dir(destPath)
		
		// Create destination directory if it doesn't exist
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
		}
		
		// Read source file
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", src, err)
		}
		
		// Write to destination
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write destination file %s: %w", destPath, err)
		}
	}
	
	// Commit the changes to a new image
	l.Logger.Printf("Committing changes to image: %s", containerName)
	
	// Create the commit options
	commitOptions := buildah.CommitOptions{
		Squash:        true,
		OmitTimestamp: false,
		SystemContext: systemContext,
		ReportWriter:  os.Stdout,
		// Add additional tags to the image
		AdditionalTags: []string{containerName},
	}
	
	// Use nil as ImageReference to let buildah generate a reference
	imageID, _, _, err := builder.Commit(ctx, nil, commitOptions)
	if err != nil {
		return fmt.Errorf("failed to commit container: %w", err)
	}
	
	l.Logger.Printf("Successfully built image: %s", imageID)
	return nil
}
