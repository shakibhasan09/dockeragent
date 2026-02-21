package model

// CreateContainerRequest is the JSON body for POST /api/v1/containers.
type CreateContainerRequest struct {
	Image      string            `json:"image"`
	Name       string            `json:"name,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Env        []string          `json:"env,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	User       string            `json:"user,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`

	Ports       []PortMapping  `json:"ports,omitempty"`
	Networks    []string       `json:"networks,omitempty"`
	NetworkMode string         `json:"network_mode,omitempty"`
	Volumes     []VolumeMount  `json:"volumes,omitempty"`

	RestartPolicy *RestartPolicy  `json:"restart_policy,omitempty"`
	Resources     *ResourceLimits `json:"resources,omitempty"`
}

type PortMapping struct {
	HostPort      string `json:"host_port,omitempty"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"host_ip,omitempty"`
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Type     string `json:"type,omitempty"`
}

type RestartPolicy struct {
	Name          string `json:"name"`
	MaxRetryCount int    `json:"max_retry_count,omitempty"`
}

type ResourceLimits struct {
	CPUs      float64 `json:"cpus,omitempty"`
	MemoryMB  int64   `json:"memory_mb,omitempty"`
	CPUShares int64   `json:"cpu_shares,omitempty"`
}

// StopContainerRequest is the optional JSON body for POST /api/v1/containers/:id/stop.
type StopContainerRequest struct {
	Timeout *int   `json:"timeout,omitempty"`
	Signal  string `json:"signal,omitempty"`
}

// RemoveContainerQuery is extracted from query params for DELETE /api/v1/containers/:id.
type RemoveContainerQuery struct {
	Force         bool `query:"force"`
	RemoveVolumes bool `query:"v"`
}

// LogsQuery is extracted from query params for GET /api/v1/containers/:id/logs.
type LogsQuery struct {
	Follow     bool   `query:"follow"`
	Tail       string `query:"tail"`
	Since      string `query:"since"`
	Until      string `query:"until"`
	Timestamps bool   `query:"timestamps"`
}

// WriteFileRequest is the JSON body for POST /api/v1/files.
type WriteFileRequest struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Permission string `json:"permission,omitempty"`
}
