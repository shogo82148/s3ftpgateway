package ftp

import "testing"

func TestServer_choosePassivePort(t *testing.T) {
	t.Run("system-choose", func(t *testing.T) {
		s := &Server{}
		port, err := s.choosePassivePort()
		if err != nil {
			t.Error(err)
		}
		if port != 0 {
			t.Errorf("want 0, got %d", port)
		}
	})

	t.Run("disable passive mode", func(t *testing.T) {
		s := &Server{
			MaxPassivePort: -1,
		}
		_, err := s.choosePassivePort()
		if err != errPassiveModeIsDisabled {
			t.Errorf("want errPassiveModeIsDisabled, git %v", err)
		}
	})

	t.Run("random port", func(t *testing.T) {
		s := &Server{
			MinPassivePort: 10000,
			MaxPassivePort: 10001,
		}

		port1, err := s.choosePassivePort()
		if err != nil {
			t.Error(err)
		}
		if port1 != 10000 && port1 != 10001 {
			t.Errorf("want 10000 or 10001, got %d", port1)
		}

		port2, err := s.choosePassivePort()
		if err != nil {
			t.Error(err)
		}
		if port2 != 10000 && port2 != 10001 {
			t.Errorf("want 10000 or 10001, got %d", port2)
		}
		if port1 == port2 {
			t.Error("want port2 does not equal port1")
		}

		if _, err = s.choosePassivePort(); err != errEmptyPortNotFound {
			t.Errorf("want errEmptyPortNotFound, got %v", err)
		}

		s.releasePassivePort(port1)
		port3, err := s.choosePassivePort()
		if err != nil {
			t.Error(err)
		}
		if port3 != port1 {
			t.Errorf("want %d, got %d", port1, port3)
		}
	})
}
