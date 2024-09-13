package renamepvc

import (
	"bufio"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"strings"
)

type renamePVCOptions struct {
	streams     genericclioptions.IOStreams
	configFlags *genericclioptions.ConfigFlags
	k8sClient   kubernetes.Interface

	confirm         bool
	oldName         string
	newName         string
	sourceNamespace string
	targetNamespace string
}

func (o *renamePVCOptions) complete(args []string) error {
	var err error
	o.oldName = args[0]
	o.newName = args[1]

	o.sourceNamespace, _, err = o.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.targetNamespace == "" {
		o.targetNamespace = o.sourceNamespace
	}

	o.k8sClient, err = getK8sClient(o.configFlags)
	return err
}

func (o *renamePVCOptions) validate() error {
	if !o.confirm {
		return o.confirmCheck()
	}
	return nil
}

func (o *renamePVCOptions) confirmCheck() error {
	_, err := fmt.Fprintf(o.streams.Out,
		"Rename PVC from '%s' in namespace '%s' to '%s' in namespace '%s'? (yes or no) ",
		o.oldName, o.sourceNamespace, o.newName, o.targetNamespace)
	if err != nil {
		return err
	}

	input, err := bufio.NewReader(o.streams.In).ReadString('\n')
	if err != nil {
		return err
	}

	switch strings.TrimSpace(strings.ToLower(input)) {
	case "y", "yes":
		return nil
	case "n", "no":
		return ErrConfirmationNotSuccessful
	default:
		return ErrConfirmationUnknown
	}
}

func (o *renamePVCOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.confirm, "yes", "y", false, "Skips confirmation if flag is set")
	cmd.Flags().StringVarP(&o.targetNamespace, "target-namespace", "N", "",
		"Defines in which namespace the new PVC should be created. By default the source PVC's namespace is used.")
	o.configFlags.AddFlags(cmd.Flags())
}
