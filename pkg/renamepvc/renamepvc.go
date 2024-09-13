package renamepvc

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"path/filepath"
	"strings"
	// for auth in kubernetes cluster
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	ErrNotBound                  = errors.New("new PVC did not get bound")
	ErrConfirmationNotSuccessful = errors.New("confirmation was not successful please type in yes to continue")
	ErrConfirmationUnknown       = errors.New("please type yes or no")
	ErrVolumeMounted             = errors.New("volume currently mounted")
)

// NewCmdRenamePVC returns the cobra command for the pvc rename
func NewCmdRenamePVC(streams genericclioptions.IOStreams) *cobra.Command {
	o := &renamePVCOptions{
		streams:     streams,
		configFlags: genericclioptions.NewConfigFlags(false),
	}

	cmd := &cobra.Command{
		Use:          fmt.Sprintf("%s [pvc name] [new pvc name]", getCommandName()),
		Short:        "Rename a PersistentVolumeClaim (PVC)",
		Long:         getLongDescription(),
		Example:      fmt.Sprintf("%s oldPvcName newPvcName", getCommandName()),
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE:         o.run,
	}

	o.addFlags(cmd)
	return cmd
}

func (o *renamePVCOptions) run(cmd *cobra.Command, args []string) error {
	if err := o.complete(args); err != nil {
		return err
	}
	if err := o.validate(); err != nil {
		return err
	}
	return o.execute(cmd.Context())
}

func (o *renamePVCOptions) checkIfMounted(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	podList, err := o.k8sClient.CoreV1().Pods(pvc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return fmt.Errorf("%w in pod %q", ErrVolumeMounted, pod.Name)
			}
		}
	}

	return nil
}

func getCommandName() string {
	command := os.Args[0]
	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		command = "kubectl rename-pvc"
	}
	return command
}

func getLongDescription() string {
	return `rename-pvc renames an existing PersistentVolumeClaim (PVC) by creating a new PVC
with the same spec and rebinding the existing PersistentClaim (PV) to the newly created PVC.
Afterwards the old PVC is automatically deleted.`
}
