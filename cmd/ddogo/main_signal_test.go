//go:build darwin || linux

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestInterruptStopsBlockingInput(t *testing.T) {
	if os.Getenv("DDOGO_SIGNAL_TEST_HELPER") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"ddogo"}, os.Args[i+1:]...)
				main()
				os.Exit(0)
			}
		}
		os.Exit(2)
	}

	// A FIFO lets us know main has reached input reading before interrupting;
	// an arbitrary startup sleep could miss the signal-handler regression.
	input := filepath.Join(t.TempDir(), "request.json")
	if err := syscall.Mkfifo(input, 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // Re-executes this test binary with fixed arguments; no shell or external command input.
	cmd := exec.Command(executable, "-test.run=^TestInterruptStopsBlockingInput$", "--",
		"--dd-api-key", "test-api", "--dd-app-key", "test-app", "--site", "datadoghq.com",
		"monitors", "create", "--request-file", input)
	cmd.Env = append(os.Environ(), "DDOGO_SIGNAL_TEST_HELPER=1")
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = cmd.Process.Kill()
			<-done
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		fd, err := syscall.Open(input, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			// Keep the writer open so the read cannot finish.
			defer func() { _ = syscall.Close(fd) }()
			break
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			waited = true
			t.Fatalf("command exited before reading input: %v / %s", err, &diagnostics)
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("command did not open input")
		}
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		waited = true
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected interrupt termination, got %v / %s", err, &diagnostics)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("command did not use normal interrupt termination: %v / %s", err, &diagnostics)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl-C was swallowed during blocking input")
	}
}
