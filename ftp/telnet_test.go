package ftp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDumbTelnetConn(t *testing.T) {
	cases := []struct {
		rin  []byte
		rout []byte
		win  []byte
		wout []byte
	}{
		{
			rin:  []byte{0x00, 0x7F},
			rout: []byte{0x00, 0x7F},
			win:  []byte{0x00, 0x7F},
			wout: []byte{0x00, 0x7F},
		},

		// Escape IAC
		{
			rin:  []byte{0xFF, 0xFF},
			rout: []byte{0xFF},
			win:  []byte{0xFF},
			wout: []byte{0xFF, 0xFF},
		},

		// Ignore Telnet Commands
		{
			rin: []byte{
				0x00,
				0xFF, 0xF1, // NOP
				0xFF, 0xF2, // Data Mark
				0xFF, 0xF3, // Break
				0xFF, 0xF4, // Interrupt Process
				0xFF, 0xF5, // Abort output
				0xFF, 0xF6, // Are You There
				0xFF, 0xF7, // Erase character
				0xFF, 0xF8, // Erase Line
				0xFF, 0xF9, // Go ahead
				0x7F,
			},
			rout: []byte{0x00, 0x7F},
			win:  []byte{0x00},
			wout: []byte{0x00},
		},

		// Telnet Options
		{
			rin: []byte{
				0x00,
				// TELNET TERMINAL TYPE OPTION https://tools.ietf.org/html/rfc884
				0xFF, 0xFD, 0x18, // IAC DO TERMINAL-TYPE
				0xFF, 0xFB, 0x18, // IAC WILL TERMINAL-TYPE
				0xFF, 0xFA, 0x18, 0x01, 0xFF, 0xF0, // IAC SB TERMINAL-TYPE SEND IAC SE
				0x7F,
			},
			rout: []byte{0x00, 0x7F},
			win:  []byte{0x00},
			wout: []byte{
				0xFF, 0xFC, 0x18, // IAC WON'T TERMINAL-TYPE
				0xFF, 0xFE, 0x18, // IAC DON'T TERMINAL-TYPE
				0x00,
			},
		},
	}

	for i, c := range cases {
		var rout, wout bytes.Buffer
		rw := newDumbTelnetConn(bytes.NewReader(c.rin), &wout)
		if _, err := rout.ReadFrom(rw); err != nil {
			t.Errorf("#%d unexpected read error: %v", i, err)
			continue
		}
		if _, err := rw.Write(c.win); err != nil {
			t.Errorf("#%d unexpected write error: %v", i, err)
			continue
		}
		if err := rw.Flush(); err != nil {
			t.Errorf("#%d unexpected flush error: %v", i, err)
		}
		if !reflect.DeepEqual(rout.Bytes(), c.rout) {
			t.Errorf("#%d want %v, got %v", i, c.rout, rout.Bytes())
		}
		if !reflect.DeepEqual(wout.Bytes(), c.wout) {
			t.Errorf("#%d want %v, got %v", i, c.wout, wout.Bytes())
		}
	}
}
