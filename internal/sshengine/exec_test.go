package sshengine

import (
	"go-ssh/config"
	"testing"
)

func TestCanRunOneOff(t *testing.T) {
	cases := []struct {
		name string
		host config.Host
		want bool
	}{
		{"structured host", config.Host{HostAddr: "example.com"}, true},
		{"bare ssh user@host", config.Host{Command: "ssh user@example.com"}, true},
		{"bare ssh with port and identity flags", config.Host{Command: "ssh -p 31061 -i /path/to/key c0072843@127.0.0.1"}, true},
		{"bare ssh, host only, no user", config.Host{Command: "ssh devserver"}, true},
		{"embedded remote command", config.Host{Command: `ssh -t jumphost@bastion "ssh -t deploy@web1 'cd /var/www && exec bash'"`}, false},
		{"multi-step commands with automation", config.Host{Commands: []string{"ssh -p 31061 c0072843@127.0.0.1", "EXPECT:password", "SENDPASS:tcell-alaca"}}, false},
		{"empty host", config.Host{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanRunOneOff(tc.host); got != tc.want {
				t.Errorf("CanRunOneOff(%+v) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestAppendRemoteCommand(t *testing.T) {
	full, err := appendRemoteCommand("ssh -p 31061 c0072843@127.0.0.1", "ls -la")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ssh -o StrictHostKeyChecking=accept-new -p 31061 c0072843@127.0.0.1 'ls -la'"
	if full != want {
		t.Errorf("got %q, want %q", full, want)
	}

	if _, err := appendRemoteCommand(`ssh bastion "exec bash"`, "ls -la"); err == nil {
		t.Error("expected error for command with an embedded remote command")
	}
}
