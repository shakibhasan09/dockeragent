package model

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type CreateContainerResponse struct {
	ID       string   `json:"id"`
	Warnings []string `json:"warnings,omitempty"`
}

type ContainerSummary struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	ImageID string            `json:"image_id"`
	Command string            `json:"command"`
	Created int64             `json:"created"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Ports   []PortSummary     `json:"ports"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type PortSummary struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type ContainerListResponse struct {
	Containers []ContainerSummary `json:"containers"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Docker    string `json:"docker"`
	Timestamp string `json:"timestamp"`
}
