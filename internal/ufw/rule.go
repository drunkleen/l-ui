package ufw

// Rule represents a single UFW firewall rule.
type Rule struct {
	Number   int    `json:"number"`
	Port     string `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Action   string `json:"action"`
	Comment  string `json:"comment,omitempty"`
}

// Status represents the current UFW state.
type Status struct {
	Active    bool   `json:"active"`
	Installed bool   `json:"installed"`
	Rules     []Rule `json:"rules"`
}
