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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	// for auth in kubernetes cluster
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	ErrNotBound           = errors.New("new PVC did not get bound")
	ErrAcceptationNotYes  = errors.New("conformation was not successful please type in yes to continue")
	ErrAcceptationUnknown = errors.New("please type yes or no")
)

type renamePVCOptions struct {
	streams     genericclioptions.IOStreams
	configFlags *genericclioptions.ConfigFlags

	confirm bool
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
		Use:          fmt.Sprintf("%v [pvc name] [new pvc name]", command),
		Short:        "Rename persistent volume claims",
		Example:      fmt.Sprintf("%v oldPvcName newPvcName", command),
		Args:         cobra.ExactArgs(2), //nolint: gomnd // needs always 2 inputs
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return o.run(args[0], args[1])
		},
	}
	cmd.Flags().BoolVarP(&o.confirm, "yes", "y", false, "Skips conformation if flag is set")
	o.configFlags.AddFlags(cmd.Flags())
	return cmd
}

// rename the PVC from oldName to newName
// renaming of objects in Kubernetes is not supported therefore it is done with recreating
// to no lose the PersistentVolume it is done with the following steps:
// 1. create the new PVC with the new name
// 2. change the claimRef in the PersistentVolume to the new PVC
// 3. wait until the new PVC is bound to the PersistentVolume
// 4. delete old PVC
func (o *renamePVCOptions) run(oldName, newName string) error {
	ctx := context.Background()

	// get current namespace
	namespace, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// check conformation
	if err := o.confirmCheck(oldName, newName, namespace); err != nil {
		return err
	}

	// init kubernetes client
	k8sClient, err := o.getK8sClient()
	if err != nil {
		return err
	}

	// get old pvc
	var oldPvc *v1.PersistentVolumeClaim
	oldPvc, err = k8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, oldName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// check if pvc is mounted
	err = checkIfMounted(ctx, k8sClient, oldPvc)
	if err != nil {
		return err
	}

	return o.rename(ctx, k8sClient, oldPvc, newName, namespace)
}

func (o renamePVCOptions) confirmCheck(oldName, newName, namespace string) error {
	_, err := fmt.Fprintf(o.streams.Out, "Rename PVC from '%s' to '%s' in namespace '%v'? (yes or no) ", oldName, newName, namespace)
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
		return ErrAcceptationNotYes
	default:
		return ErrAcceptationUnknown
	}
}

func (o renamePVCOptions) getK8sClient() (*kubernetes.Clientset, error) {
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func checkIfMounted(ctx context.Context, k8sClient *kubernetes.Clientset, pvc *v1.PersistentVolumeClaim) error {
	podList, err := k8sClient.CoreV1().Pods(pvc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for pod := range podList.Items {
		for vol := range podList.Items[pod].Spec.Volumes {
			if podList.Items[pod].Spec.Volumes[vol].PersistentVolumeClaim != nil &&
				podList.Items[pod].Spec.Volumes[vol].PersistentVolumeClaim.ClaimName == pvc.Name {
				return fmt.Errorf("volume is mounted in pod '%s'", podList.Items[pod].Name)
			}
		}
	}
	return nil
}

func waitUntilPvcIsBound(ctx context.Context, k8sClient *kubernetes.Clientset, pvc *v1.PersistentVolumeClaim) error {
	for i := 0; i <= 60; i++ {
		checkPVC, err := k8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if checkPVC.Status.Phase == v1.ClaimBound {
			return nil
		}
		time.Sleep(time.Second)
	}
	return ErrNotBound
}

func (o renamePVCOptions) rename(ctx context.Context,
	k8sClient *kubernetes.Clientset,
	oldPvc *v1.PersistentVolumeClaim,
	newPvcName string,
	namespace string,
) error {
	// get new pvc with old PVC inputs
	newPvc := oldPvc.DeepCopy()
	newPvc.Status = v1.PersistentVolumeClaimStatus{}
	newPvc.Name = newPvcName
	newPvc.UID = ""
	newPvc.CreationTimestamp = metav1.Now()
	newPvc.SelfLink = ""
	newPvc.ResourceVersion = ""

	// get pv
	var pv *v1.PersistentVolume
	pv, err := k8sClient.CoreV1().PersistentVolumes().Get(ctx, oldPvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// create new pvc
	newPvc, err = k8sClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, newPvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "New PVC with name '%s' created\n", newPvc.Name)

	// update pv to new pvc
	pv.Spec.ClaimRef = &v1.ObjectReference{
		Kind:            newPvc.Kind,
		Namespace:       newPvc.Namespace,
		Name:            newPvc.Name,
		UID:             newPvc.UID,
		APIVersion:      newPvc.APIVersion,
		ResourceVersion: newPvc.ResourceVersion,
	}
	pv, err = k8sClient.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "ClaimRef of PV '%s' is updated to new PVC '%s'\n", pv.Name, newPvc.Name)

	// wait until newPvc is bound
	err = waitUntilPvcIsBound(ctx, k8sClient, newPvc)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "New PVC '%s' is bound to PV '%s'\n", newPvc.Name, pv.Name)

	// delete oldPVC
	err = k8sClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, oldPvc.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.streams.Out, "Old PVC '%s' is deleted\n", oldPvc.Name)

	return nil
}
