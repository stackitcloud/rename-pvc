package renamepvc

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (o *renamePVCOptions) execute(ctx context.Context) error {
	oldPvc, err := o.k8sClient.CoreV1().PersistentVolumeClaims(o.sourceNamespace).Get(ctx, o.oldName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := o.checkIfMounted(ctx, oldPvc); err != nil {
		return err
	}

	return o.rename(ctx, oldPvc)
}

// rename renames the PVC.
func (o *renamePVCOptions) rename(ctx context.Context, oldPvc *corev1.PersistentVolumeClaim) error {
	newPvc, err := o.createNewPVC(ctx, oldPvc)
	if err != nil {
		return fmt.Errorf("failed to create new PVC: %w", err)
	}

	pv, err := o.updatePVClaimRef(ctx, oldPvc, newPvc)
	if err != nil {
		return fmt.Errorf("failed to update PV claim ref: %w", err)
	}

	if err := o.waitUntilPvcIsBound(ctx, newPvc); err != nil {
		return fmt.Errorf("failed to wait for PVC to be bound: %w", err)
	}

	if err := o.deleteOldPVC(ctx, oldPvc); err != nil {
		return fmt.Errorf("failed to delete old PVC: %w", err)
	}

	o.printSuccess(newPvc, pv)
	return nil
}

// waitUntilPvcIsBound waits until the new PVC is bound.
func (o *renamePVCOptions) waitUntilPvcIsBound(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	for i := 0; i < 60; i++ {
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

// createNewPVC creates a new PVC with the same spec as the old PVC but with a new name and namespace.
func (o *renamePVCOptions) createNewPVC(ctx context.Context, oldPvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	newPvc := createNewPVC(oldPvc, o.newName, o.targetNamespace)
	newPvc, err := o.k8sClient.CoreV1().PersistentVolumeClaims(o.targetNamespace).Create(ctx, newPvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(o.streams.Out, "New PVC with name '%s' created\n", newPvc.Name)
	return newPvc, nil
}

// updatePVClaimRef updates the ClaimRef of the PV to the new PVC.
func (o *renamePVCOptions) updatePVClaimRef(ctx context.Context, oldPvc, newPvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	pv, err := o.k8sClient.CoreV1().PersistentVolumes().Get(ctx, oldPvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pv.Spec.ClaimRef = createClaimRef(newPvc)
	pv, err = o.k8sClient.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(o.streams.Out, "ClaimRef of PV '%s' is updated to new PVC '%s'\n", pv.Name, newPvc.Name)
	return pv, nil
}

// deleteOldPVC deletes the old PVC.
func (o *renamePVCOptions) deleteOldPVC(ctx context.Context, oldPvc *corev1.PersistentVolumeClaim) error {
	err := o.k8sClient.CoreV1().PersistentVolumeClaims(o.sourceNamespace).Delete(ctx, oldPvc.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	fmt.Fprintf(o.streams.Out, "Old PVC '%s' is deleted\n", oldPvc.Name)
	return nil
}

// printSuccess prints a success message to the output stream.
func (o *renamePVCOptions) printSuccess(newPvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume) {
	fmt.Fprintf(o.streams.Out, "New PVC '%s' is bound to PV '%s'\n", newPvc.Name, pv.Name)
}
