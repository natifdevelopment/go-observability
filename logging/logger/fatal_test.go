package logger

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestLogger_Fatal_Subprocess runs Fatal in a subprocess to verify it exits with code 1.
func TestLogger_Fatal_Subprocess(t *testing.T) {
	if os.Getenv("TEST_FATAL_SUB") == "1" {
		log, _, _ := newTestLogger(t, nil)
		log.Fatal(context.Background(), "fatal subprocess test")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLogger_Fatal_Subprocess")
	cmd.Env = append(os.Environ(), "TEST_FATAL_SUB=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
		t.Errorf("Fatal should exit with code 1, got %v", err)
	}
}

// TestLogger_FatalWithErr_Subprocess runs FatalWithErr in a subprocess.
func TestLogger_FatalWithErr_Subprocess(t *testing.T) {
	if os.Getenv("TEST_FATAL_ERR_SUB") == "1" {
		log, _, _ := newTestLogger(t, nil)
		log.FatalWithErr(context.Background(), "fatal err subprocess", fmt.Errorf("fatal"))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestLogger_FatalWithErr_Subprocess")
	cmd.Env = append(os.Environ(), "TEST_FATAL_ERR_SUB=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
		t.Errorf("FatalWithErr should exit with code 1, got %v", err)
	}
}
