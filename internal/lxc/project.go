package lxc

import "encoding/json"

func ProjectExists(name string) bool {
	_, err := Run("project", "show", name)
	return err == nil
}

// ProjectCreate creates a project that shares profiles, images, networks, and
// storage from the default project so base instances get a root disk and network.
func ProjectCreate(name string) error {
	_, err := Run("project", "create", name,
		"-c", "features.profiles=false",
		"-c", "features.images=false",
		"-c", "features.networks=false",
		"-c", "features.storage.volumes=false",
	)
	return err
}

func CopyFromProject(srcProject, srcName, destName string) error {
	_, err := Run("copy", srcName, destName, "--project", srcProject, "--target-project", "default")
	return err
}

func InstanceExistsInProject(project, name string) bool {
	_, err := Run(projectArgs(project, "info", name)...)
	return err == nil
}

// ListInstancesInProject returns the names of all instances in a project.
// Uses JSON format because lxc list --format=csv is broken with --project.
func ListInstancesInProject(project string) ([]string, error) {
	output, err := Run(projectArgs(project, "list", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	var instances []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &instances); err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, nil
	}
	names := make([]string, len(instances))
	for i, inst := range instances {
		names[i] = inst.Name
	}
	return names, nil
}
