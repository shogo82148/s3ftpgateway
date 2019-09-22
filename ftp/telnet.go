package ftp

import (
	"bufio"
	"bytes"
	"io"
	"sync"
)

const (
	telnetCmdSe   = 240
	telnetCmdSb   = 250
	telnetCmdWill = 251
	telnetCmdWont = 252
	telnetCmdDo   = 253
	telnetCmdDont = 254
	telnetCmdIac  = 255
)

// dumbTelnetConn is a dumb telnet client.
// it ignores the telnet commands, and do not anything.
// https://tools.ietf.org/html/rfc854
// Most ftp clients may not send the telnet commands, while the ftp sever should handle them.
type dumbTelnetConn struct {
	r *bufio.Reader
	w *bufio.Writer

	// for reject options.
	mu      sync.Mutex
	willopt []byte
	doopt   []byte
}

func newDumbTelnetConn(r io.Reader, w io.Writer) *dumbTelnetConn {
	return &dumbTelnetConn{
		r:       bufio.NewReader(r),
		w:       bufio.NewWriter(w),
		willopt: []byte{},
		doopt:   []byte{},
	}
}

func (c *dumbTelnetConn) Read(buf []byte) (int, error) {
	i := 0
	for i < len(buf) {
		b, err := c.r.ReadByte()
		if err != nil {
			if err != io.EOF {
				return 0, err
			}
			return i, io.EOF
		}
		if b != telnetCmdIac {
			buf[i] = b
			i++
			if b == '\n' {
				// found the end of line, exit loop.
				break
			}
			continue
		}

		code, err := c.r.ReadByte()
		if err != nil {
			return 0, err
		}
		switch code {
		case telnetCmdSb:
			// One step of subnegotiation, used by either party.
			// https://tools.ietf.org/html/rfc855
			// IAC SB ABC <parameters> IAC SE
			if err := c.ignoreSubnegotiation(); err != nil {
				return i, err
			}
		case telnetCmdWill:
			if err := c.readWillOption(); err != nil {
				return i, err
			}
		case telnetCmdDo:
			if err := c.readDoOption(); err != nil {
				return i, err
			}
		case telnetCmdWont, telnetCmdDont:
			// Ignore the option.
			_, err := c.r.ReadByte()
			if err != nil {
				return i, err
			}
		case telnetCmdIac:
			buf[i] = 255
			i++
		default:
			// Do nothing, just ignore commands.
		}
	}
	return i, nil
}

func (c *dumbTelnetConn) ignoreSubnegotiation() error {
	// Ignore the option.
	if _, err := c.r.ReadByte(); err != nil {
		return err
	}

	// Search IAC SE sequence
	foundIac := false
	for {
		code, err := c.r.ReadByte()
		if err != nil {
			return err
		}
		if foundIac {
			if code == telnetCmdSe {
				return nil
			}
			foundIac = false
		} else if code == telnetCmdIac {
			foundIac = true
		}
	}
}

func (c *dumbTelnetConn) readWillOption() error {
	opt, err := c.r.ReadByte()
	if err != nil {
		return err
	}

	// send the reject response when next write.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.willopt = append(c.willopt, opt)
	return err
}

func (c *dumbTelnetConn) readDoOption() error {
	opt, err := c.r.ReadByte()
	if err != nil {
		return err
	}

	// send the reject response when next write.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.doopt = append(c.doopt, opt)
	return err
}

func (c *dumbTelnetConn) Write(buf []byte) (int, error) {
	if err := c.writeOption(); err != nil {
		return 0, err
	}

	m := 0
	for {
		if idx := bytes.IndexByte(buf, 255); idx >= 0 {
			n, err := c.w.Write(buf[:idx])
			if err != nil {
				return 0, err
			}
			m += n
			buf = buf[idx:]

			// Escape 255
			c.w.WriteByte(telnetCmdIac)
		}

		n, err := c.w.Write(buf)
		if err != nil {
			return 0, err
		}
		m += n
		break
	}
	return m, nil
}

func (c *dumbTelnetConn) writeOption() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, b := range c.willopt {
		_, err := c.w.Write([]byte{telnetCmdIac, telnetCmdWont, b})
		if err != nil {
			return err
		}
	}
	c.willopt = c.willopt[:0]

	for _, b := range c.doopt {
		_, err := c.w.Write([]byte{telnetCmdIac, telnetCmdDont, b})
		if err != nil {
			return err
		}
	}
	c.doopt = c.doopt[:0]

	return nil
}

func (c *dumbTelnetConn) Flush() error {
	if err := c.writeOption(); err != nil {
		return err
	}
	return c.w.Flush()
}
