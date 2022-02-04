package main

import (
	"os"

	rename "github.com/stackitcloud/pvc-rename/pkg/renamepvc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	root := rename.NewCmdRenamePVC(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	return root.Execute()
}
