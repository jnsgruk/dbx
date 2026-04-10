package config

const (
	// User is the username created inside instances.
	User = "jon"

	// GitHubUser is the GitHub username used for ssh-import-id.
	GitHubUser = "jnsgruk"

	// Image is the default LXD image for new instances.
	Image = "ubuntu:questing"

	// Shell is the interactive shell launched by `dbx shell`.
	Shell = "fish"

	// LoginShell is the default login shell assigned to the user inside instances.
	LoginShell = "/usr/bin/bash"

	// RemapUID is the uid/gid the cloud-init user (ubuntu) is reassigned to
	// in order to free up the host uid for the new user.
	RemapUID = "1100"
)
