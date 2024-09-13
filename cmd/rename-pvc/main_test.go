package main

import (
	"bytes"
	"context"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"testing"
	"time"
)

type mockIOStreams struct {
	In     io.Reader
	Out    *bytes.Buffer
	ErrOut *bytes.Buffer
}

func mockCommand(streams genericclioptions.IOStreams) *cobra.Command {
	return &cobra.Command{
		Use: "mock-rename-pvc",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Simulate some work
			time.Sleep(100 * time.Millisecond)
			_, err := io.WriteString(streams.Out, "Mock command executed")
			return err
		},
	}
}

func TestRun(t *testing.T) {
	mockStreams := &mockIOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}

	mockRun := func() error {
		cmd := mockCommand(genericclioptions.IOStreams{
			In:     mockStreams.In,
			Out:    mockStreams.Out,
			ErrOut: mockStreams.ErrOut,
		})
		return cmd.Execute()
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- mockRun()
	}()

	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Errorf("run() returned an unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("run() did not complete within the expected time")
	}

	if !bytes.Contains(mockStreams.Out.Bytes(), []byte("Mock command executed")) {
		t.Error("Expected output not found")
	}
}
