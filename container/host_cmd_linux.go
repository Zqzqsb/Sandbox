package container

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/zqzqsb/sandbox/pkg/unixsocket"
)

// Ping send ping message to container, wait for 3 second before timeout
func (c *container) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// avoid infinite wait (max 3s)
	const pingWait = 3 * time.Second
	c.socket.SetDeadline(time.Now().Add(pingWait))
	defer c.socket.SetDeadline(time.Time{})

	// send ping
	cmd := cmd{
		Cmd: cmdPing,
	}
	if err := c.sendCmd(cmd, unixsocket.Msg{}); err != nil {
		return fmt.Errorf("ping: %v", err)
	}
	// receive no error
	return c.recvAckReply("ping")
}

// conf send configuration to container (used by builder only)
func (c *container) conf(conf *containerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := cmd{
		Cmd:     cmdConf,
		ConfCmd: &confCmd{Conf: *conf},
	}
	if err := c.sendCmd(cmd, unixsocket.Msg{}); err != nil {
		return fmt.Errorf("conf: %v", err)
	}
	return c.recvAckReply("conf")
}

// Open open files in container
func (c *container) Open(p []OpenCmd) ([]*os.File, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()

	// send copyin
	cmd := cmd{
		Cmd:     cmdOpen,
		OpenCmd: p,
	}
	if err := c.sendCmd(cmd, unixsocket.Msg{}); err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	reply, msg, err := c.recvReply()
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("open: %v", reply.Error)
	}
	if len(msg.Fds) != len(p) {
		closeFds(msg.Fds)
		return nil, fmt.Errorf("open: unexpected number of fd %v / %v", len(msg.Fds), len(p))
	}

	ret := make([]*os.File, 0, len(p))
	for i, fd := range msg.Fds {
		syscall.CloseOnExec(fd)
		f := os.NewFile(uintptr(fd), p[i].Path)
		if f == nil {
			closeFds(msg.Fds)
			return nil, fmt.Errorf("open: failed NewFile %v", fd)
		}
		ret = append(ret, f)
	}
	return ret, nil
}

// Delete remove file from container
func (c *container) Delete(p string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := cmd{
		Cmd:       cmdDelete,
		DeleteCmd: &deleteCmd{Path: p},
	}
	if err := c.sendCmd(cmd, unixsocket.Msg{}); err != nil {
		return fmt.Errorf("delete: %v", err)
	}
	return c.recvAckReply("delete")
}

// Reset remove all from /tmp and /w
func (c *container) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := cmd{
		Cmd: cmdReset,
	}
	if err := c.sendCmd(cmd, unixsocket.Msg{}); err != nil {
		return fmt.Errorf("reset: %v", err)
	}
	return c.recvAckReply("reset")
}
