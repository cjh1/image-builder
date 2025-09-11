package layer

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/storage"
)

// Publish publishes the built container image
func (l *Layer) Publish(containerName string) error {
	l.Logger.Printf("Publishing container: %s", containerName)
	
	// Check if we need to push to registry
	if l.Args.RegistryHost == "" {
		l.Logger.Printf("No registry host specified, skipping push")
		return nil
	}
	
	// Build the full image name with registry
	fullImageName := fmt.Sprintf("%s/%s", l.Args.RegistryHost, l.Args.Name)
	if l.Args.RegistryNamespace != "" {
		fullImageName = fmt.Sprintf("%s/%s/%s", l.Args.RegistryHost, l.Args.RegistryNamespace, l.Args.Name)
	}
	
	// Setup the context
	ctx := context.Background()
	
	// Get the buildah store
	storeOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return fmt.Errorf("failed to get default store options: %w", err)
	}
	
	store, err := storage.GetStore(storeOptions)
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}
	
	// Find the source image in the storage
	l.Logger.Printf("Finding source image: %s", containerName)
	images, err := store.Images()
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}
	
	var sourceImageID string
	for _, image := range images {
		for _, name := range image.Names {
			if name == containerName {
				sourceImageID = image.ID
				break
			}
		}
		if sourceImageID != "" {
			break
		}
	}
	
	if sourceImageID == "" {
		return fmt.Errorf("could not find image %s in storage", containerName)
	}
	
	// Tag the image
	l.Logger.Printf("Tagging image: %s as %s", containerName, fullImageName)
	if err := store.AddNames(sourceImageID, []string{fullImageName}); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}
	
	// Push the image to the registry
	l.Logger.Printf("Pushing image to registry: %s", fullImageName)
	
	// Set up push options with system context
	systemCtx := &types.SystemContext{}
	
	// Handle registry options
	for _, opt := range l.Args.RegistryOptsPush {
		if strings.Contains(opt, "--tls-verify=false") {
			systemCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		}
	}
	
	// Setup destination reference for push
	destRef, err := alltransports.ParseImageName("docker://" + fullImageName)
	if err != nil {
		return fmt.Errorf("failed to parse destination reference: %w", err)
	}
	
	// Setup push options
	pushOptions := buildah.PushOptions{
		ReportWriter:  l.Logger.Writer(),
		Store:         store,
		SystemContext: systemCtx,
	}
	
	// Push the image
	_, digest, err := buildah.Push(ctx, fullImageName, destRef, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	
	l.Logger.Printf("Successfully pushed image: %s (digest: %s)", fullImageName, digest)
	return nil
}
