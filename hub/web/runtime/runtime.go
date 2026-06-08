package runtime

import (
	"context"

	"github.com/drunkleen/l-ui/internal/database/model"
)

type Runtime interface {
	Name() string

	AddInbound(ctx context.Context, ib *model.Inbound) error
	DelInbound(ctx context.Context, ib *model.Inbound) error
	UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error

	AddUser(ctx context.Context, ib *model.Inbound, userMap map[string]any) error
	RemoveUser(ctx context.Context, ib *model.Inbound, email string) error

	// Per-client operations that route through the node's clients API on
	// Remote (instead of pushing the whole inbound) so the node applies
	// per-user xray API calls without a DelInbound+AddInbound cycle.
	UpdateUser(ctx context.Context, ib *model.Inbound, email string, payload model.Client) error
	DeleteUser(ctx context.Context, ib *model.Inbound, email string) error
	AddClient(ctx context.Context, ib *model.Inbound, client model.Client) error

	RestartXray(ctx context.Context) error

	ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error
	ResetAllTraffics(ctx context.Context) error
}

type UfwRule struct {
	Number   int    `json:"number"`
	Port     string `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Action   string `json:"action"`
	Comment  string `json:"comment,omitempty"`
}

type UfwStatus struct {
	Active    bool      `json:"active"`
	Installed bool      `json:"installed"`
	Rules     []UfwRule `json:"rules"`
}

// agentFirewallRule mirrors agent/service.FirewallRule for decoding agent responses.
type agentFirewallRule struct {
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"`
	Comment  string `json:"comment,omitempty"`
}

// agentFirewallStatus mirrors agent/service.FirewallStatus for decoding agent responses.
type agentFirewallStatus struct {
	Active    bool                `json:"active"`
	Installed bool                `json:"installed"`
	Rules     []agentFirewallRule `json:"rules"`
}
