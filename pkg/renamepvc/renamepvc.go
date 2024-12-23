package renamepvc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	// for auth in kubernetes cluster
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	ErrNotBound                  = errors.New("new PVC did not get bound")
	ErrConfirmationNotSuccessful = errors.New("confirmation was not successful please type in yes to continue")
	ErrConfirmationUnknown       = errors.New("please type yes or no")
	ErrVolumeMounted             = errors.New("volume currently mounted")
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

// NewCmdRenamePVC returns the cobra command for the pvc rename
func NewCmdRenamePVC(streams genericclioptions.IOStreams) *cobra.Command {
	o := renamePVCOptions{
		streams:     streams,
		configFlags: genericclioptions.NewConfigFlags(false),
	}

	command := os.Args[0]
	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		command = "kubectl rename-pvc"
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%v [pvc name] [new pvc name]", command),
		Short: "Rename a PersistentVolumeClaim (PVC)",
		Long: `rename-pvc renames an existing PersistentVolumeClaim (PVC) by creating a new PVC
with the same spec and rebinding the existing PersistentClaim (PV) to the newly created PVC.
Afterwards the old PVC is automatically deleted.`,
		Example:      fmt.Sprintf("%v oldPvcName newPvcName", command),
		Args:         cobra.ExactArgs(2), //nolint: mnd // needs always 2 inputs
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			var err error
			o.sourceNamespace, _, err = o.configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
			}

			if o.targetNamespace == "" {
				o.targetNamespace = o.sourceNamespace
			}

			o.oldName = args[0]
			o.newName = args[1]

			o.k8sClient, err = getK8sClient(o.configFlags)
			if err != nil {
				return err
			}
			return o.run(c.Context())
		},
	}
	cmd.Flags().BoolVarP(&o.confirm, "yes", "y", false, "Skips confirmation if flag is set")
	cmd.Flags().StringVarP(&o.targetNamespace, "target-namespace", "N", "",
		"Defines in which namespace the new PVC should be created. By default the source PVC's namespace is used.")
	o.configFlags.AddFlags(cmd.Flags())
	return cmd
}

// run manages the workflow for renaming a pvc from oldName to newName
func (o *renamePVCOptions) run(ctx context.Context) error {
	if err := o.confirmCheck(); err != nil {
		return err
	}

	oldPvc, err := o.k8sClient.CoreV1().PersistentVolumeClaims(o.sourceNamespace).Get(ctx, o.oldName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = o.checkIfMounted(ctx, oldPvc)
	if err != nil {
		return err
	}

	return o.rename(ctx, oldPvc)
}

func (o *renamePVCOptions) confirmCheck() error {
	if o.confirm {
		return nil
	}
	_, err := fmt.Fprintf(o.streams.Out,
		"Rename PVC from '%s' in namespace '%s' to '%s' in namespace '%v'? (yes or no) ",
		o.oldName, o.sourceNamespace, o.newName, o.targetNamespace)
	if err != nil {
		return err
	}
	input, err := bufio.NewReader(o.streams.In).ReadString('\n')
	if err != nil {
		return err
	}
	switch strings.TrimSuffix(input, "\n") {
	case "y", "yes":
		return nil
	case "n", "no":
		return ErrConfirmationNotSuccessful
	default:
		return ErrConfirmationUnknown
	}
}

func getK8sClient(configFlags *genericclioptions.ConfigFlags) (*kubernetes.Clientset, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// checkIfMounted returns an error if the volume is mounted in a pod
func (o *renamePVCOptions) checkIfMounted(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	podList, err := o.k8sClient.CoreV1().Pods(pvc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for pod := range podList.Items {
		for vol := range podList.Items[pod].Spec.Volumes {
			volume := &podList.Items[pod].Spec.Volumes[vol]
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return fmt.Errorf("%w in pod \"%s\"", ErrVolumeMounted, podList.Items[pod].Name)
			}
		}
	}
	return nil
}

// waitUntilPvcIsBound waits util the pvc is in state Bound, with a timeout of 60 sec
func (o *renamePVCOptions) waitUntilPvcIsBound(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	for i := 0; i <= 60; i++ {
		checkPVC, err := o.k8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		if checkPVC.Status.Phase == corev1.ClaimBound {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return ErrNotBound
}

// rename the oldPvc to newName
func (o *renamePVCOptions) rename(
	ctx context.Context,
	oldPvc *corev1.PersistentVolumeClaim,
) error {
	// get new pvc with old PVC inputs
	newPvc := oldPvc.DeepCopy()
	newPvc.Status = corev1.PersistentVolumeClaimStatus{}
	newPvc.Name = o.newName
	newPvc.UID = ""
	newPvc.CreationTimestamp = metav1.Now()
	newPvc.SelfLink = "" //nolint: staticcheck // to keep compatibility with older versions
	newPvc.ResourceVersion = ""
	newPvc.Namespace = o.targetNamespace

	pv, err := o.k8sClient.CoreV1().PersistentVolumes().Get(ctx, oldPvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newPvc, err = o.k8sClient.CoreV1().PersistentVolumeClaims(o.targetNamespace).Create(ctx, newPvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "New PVC with name '%s' created\n", newPvc.Name)

	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Kind:            newPvc.Kind,
		Namespace:       newPvc.Namespace,
		Name:            newPvc.Name,
		UID:             newPvc.UID,
		APIVersion:      newPvc.APIVersion,
		ResourceVersion: newPvc.ResourceVersion,
	}
	pv, err = o.k8sClient.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "ClaimRef of PV '%s' is updated to new PVC '%s'\n", pv.Name, newPvc.Name)

	err = o.waitUntilPvcIsBound(ctx, newPvc)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "New PVC '%s' is bound to PV '%s'\n", newPvc.Name, pv.Name)

	err = o.k8sClient.CoreV1().PersistentVolumeClaims(o.sourceNamespace).Delete(ctx, oldPvc.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "Old PVC '%s' is deleted\n", oldPvc.Name)

	return nil
}
