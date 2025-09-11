package layer

import (
	"fmt"
)

// BuildLayer builds a layer based on the layer type and publishes it if needed
func (l *Layer) BuildLayer() error {
	l.Logger.Printf("Building layer: %s of type %s", l.Args.Name, l.Args.LayerType)
	
	var (
		err error
		containerName string
	)
	
	// Build the layer based on the type
	switch l.Args.LayerType {
	case "base":
		err = l.BuildBase()
		containerName = l.Args.Name
	case "ansible":
		err = l.BuildAnsible()
		containerName = l.Args.Name
	default:
		return fmt.Errorf("unsupported layer type: %s", l.Args.LayerType)
	}
	
	if err != nil {
		return fmt.Errorf("failed to build layer: %w", err)
	}
	
	l.Logger.Printf("Successfully built layer: %s", l.Args.Name)
	
	// Publish the layer if needed
	if err := l.Publish(containerName); err != nil {
		return fmt.Errorf("failed to publish layer: %w", err)
	}
	
	return nil
}
