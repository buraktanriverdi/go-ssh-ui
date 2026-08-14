package sshengine

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"go-ssh/config"

	"golang.org/x/crypto/ssh"
)

// pooledClient is a dialed jump chain: chain[0] is the first hop's client,
// chain[len-1] is the final target's client (what sessions actually use).
// Every client in between is only alive because the one after it tunnels
// through it, so they must all be closed together, innermost (final) first.
type pooledClient struct {
	chain    []*ssh.Client
	refCount int
}

func (pc *pooledClient) client() *ssh.Client {
	return pc.chain[len(pc.chain)-1]
}

func (pc *pooledClient) closeAll() {
	for i := len(pc.chain) - 1; i >= 0; i-- {
		_ = pc.chain[i].Close()
	}
}

func hostPort(h config.Host) int {
	if h.Port > 0 {
		return h.Port
	}
	return 22
}

func hostAddr(h config.Host) string {
	return net.JoinHostPort(h.HostAddr, strconv.Itoa(hostPort(h)))
}

// chainKey identifies a dial chain (jump hosts + target) so sessions to the
// same host reuse one authenticated connection instead of re-dialing. Two
// different targets that happen to share a first jump host are NOT
// deduplicated at the per-hop level in this version - only whole-chain
// reuse (repeat connections to the very same target) is pooled, which is
// what multiple tabs to one host need.
func chainKey(host config.Host) string {
	key := ""
	for _, hop := range host.JumpVia {
		key += hop + ">"
	}
	if host.ID != "" {
		return key + host.ID
	}
	return key + hostAddr(host) + "@" + host.User
}

func (m *Manager) dialHop(sessionID string, h config.Host, through *ssh.Client) (*ssh.Client, error) {
	methods, err := m.authMethods(sessionID, h)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", h.Name, err)
	}
	hkcb, err := m.hostKeyCallback(sessionID)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            h.User,
		Auth:            methods,
		HostKeyCallback: hkcb,
		Timeout:         15 * time.Second,
	}
	addr := hostAddr(h)

	if through == nil {
		client, err := ssh.Dial("tcp", addr, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s (%s) bağlantısı başarısız: %w", h.Name, addr, err)
		}
		return client, nil
	}

	conn, err := through.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%s üzerinden %s'a ulaşılamadı: %w", h.Name, addr, err)
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%s: kimlik doğrulama başarısız: %w", h.Name, err)
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

// dialChain resolves host.JumpVia (host IDs) to config.Host records via
// m.hosts, dials each hop in order, and returns a client for the final
// target plus its pool key - reusing (and ref-counting) a pooled chain when
// one already exists for this exact target.
func (m *Manager) dialChain(sessionID string, host config.Host) (*ssh.Client, string, error) {
	key := chainKey(host)

	m.mu.Lock()
	if pc, ok := m.clients[key]; ok {
		pc.refCount++
		m.mu.Unlock()
		return pc.client(), key, nil
	}
	m.mu.Unlock()

	var chain []*ssh.Client
	cleanup := func() {
		for i := len(chain) - 1; i >= 0; i-- {
			_ = chain[i].Close()
		}
	}

	var current *ssh.Client
	for _, hopID := range host.JumpVia {
		hop, ok := m.hosts.FindHostByID(hopID)
		if !ok {
			cleanup()
			return nil, "", fmt.Errorf("jump host bulunamadı: %s", hopID)
		}
		next, err := m.dialHop(sessionID, hop, current)
		if err != nil {
			cleanup()
			return nil, "", err
		}
		current = next
		chain = append(chain, current)
	}

	final, err := m.dialHop(sessionID, host, current)
	if err != nil {
		cleanup()
		return nil, "", err
	}
	chain = append(chain, final)

	m.mu.Lock()
	m.clients[key] = &pooledClient{chain: chain, refCount: 1}
	m.mu.Unlock()

	return final, key, nil
}

// releaseClient drops a reference to a pooled chain, closing every client
// in it once nothing else is using it.
func (m *Manager) releaseClient(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pc, ok := m.clients[key]
	if !ok {
		return
	}
	pc.refCount--
	if pc.refCount <= 0 {
		delete(m.clients, key)
		pc.closeAll()
	}
}
