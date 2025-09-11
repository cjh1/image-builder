package layer

import (
	"context"
	"fmt"
	"os"

	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

// BuildAnsible builds a layer using Ansible
func (l *Layer) BuildAnsible() error {
	// Make sure we're running as root for buildah
	unshare.MaybeReexecUsingUserNamespace(false)
	
	containerName := l.Args.Name
	
	l.Logger.Printf("Building Ansible layer: %s from parent %s", containerName, l.Args.Parent)
	
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
	
	// Create container names map for Ansible
	containerNames := map[string]map[string]interface{}{
		containerName: {
			"ansible_groups": l.Args.AnsibleGroups,
			"ansible_pb":     l.Args.AnsiblePlaybooks,
			"ansible_vars":   l.Args.AnsibleVars,
		},
	}
	
	// Run Ansible playbooks
	l.Logger.Printf("Running Ansible playbooks: %v", l.Args.AnsiblePlaybooks)
	if err := utils.RunAnsiblePlaybooks(containerNames, l.Args.AnsibleInventory, l.Args.AnsibleVerbosity); err != nil {
		return fmt.Errorf("failed to run Ansible playbooks: %w", err)
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
	l.Logger.Printf("Successfully built Ansible image: %s", imageID)
	return nil
}
