// Copyright (c) 2011 Ross Light.
// Copyright (c) 2017, 2020 Anner van Hardenbroek.

package ftp

import (
	"context"
	"io"
)

// Text sends a command and opens a new passive data connection in ASCII mode.
func (c *Client) Text(ctx context.Context, command string) (Reply, io.ReadWriteCloser, error) {
	return c.transfer(ctx, command, "A")
}

// Binary sends a command and opens a new passive data connection in image mode.
func (c *Client) Binary(ctx context.Context, command string) (Reply, io.ReadWriteCloser, error) {
	return c.transfer(ctx, command, "I")
}

// transfer sends a command and opens a new passive data connection.
func (c *Client) transfer(ctx context.Context, command, dataType string) (Reply, io.ReadWriteCloser, error) {
	// Set type
	if reply, err := c.sendCommand(ctx, "TYPE "+dataType); err != nil {
		return Reply{}, nil, err
	} else if !reply.PositiveComplete() {
		return Reply{}, nil, reply
	}

	// Open data connection
	conn, err := c.openPassive(ctx)
	if err != nil {
		return Reply{}, nil, err
	}
	defer func(conn io.Closer) {
		if err != nil {
			conn.Close()
		}
	}(conn)

	// Send command
	reply, err := c.sendCommand(ctx, command)
	if err != nil {
		return Reply{}, nil, err
	} else if !reply.Positive() {
		return Reply{}, nil, reply
	}
	return reply, &transferConn{conn, c, ctx}, nil
}

type transferConn struct {
	rwc io.ReadWriteCloser
	c   *Client
	ctx context.Context
}

func (tc *transferConn) Read(p []byte) (n int, err error) {
	if tc.ctx.Done() == nil {
		return tc.rwc.Read(p)
	}
	select {
	default:
		return tc.rwc.Read(p)
	case <-tc.ctx.Done():
		return 0, tc.ctx.Err()
	}
}

func (tc *transferConn) Write(p []byte) (n int, err error) {
	if tc.ctx.Done() == nil {
		return tc.rwc.Write(p)
	}
	select {
	default:
		return tc.rwc.Write(p)
	case <-tc.ctx.Done():
		return 0, tc.ctx.Err()
	}
}

func (tc *transferConn) Close() error {
	if err := tc.rwc.Close(); err != nil {
		return err
	}
	if reply, err := tc.c.readResponse(); err != nil {
		return err
	} else if !reply.PositiveComplete() {
		return reply
	}
	return nil
}
