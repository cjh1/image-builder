package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetOS determines the OS details from the mounted container
func GetOS(mdir string) (string, map[string]string, error) {
	osReleaseFile := filepath.Join(mdir, "etc/os-release")
	file, err := os.Open(osReleaseFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open os-release file: %w", err)
	}
	defer file.Close()

	osDict := make(map[string]string)
	osVersion := ""
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				// Remove quotes if present
				value := strings.Trim(parts[1], "\"")
				osDict[parts[0]] = value
			}
		}
	}

	if scanner.Err() != nil {
		return "", osDict, fmt.Errorf("error reading os-release file: %w", scanner.Err())
	}

	// Determine OS version string
	if id, ok := osDict["ID"]; ok && id != "" {
		if versionID, ok := osDict["VERSION_ID"]; ok && versionID != "" {
			osVersion = id + versionID
		}
	} else if idLike, ok := osDict["ID_LIKE"]; ok && idLike != "" {
		// Fallback to ID_LIKE
		osVersion = idLike
	}

	return osVersion, osDict, nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// CreateDir creates a directory if it doesn't exist
func CreateDir(path string) error {
	if !DirExists(path) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}
