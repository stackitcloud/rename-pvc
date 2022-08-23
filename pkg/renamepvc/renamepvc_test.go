package renamepvc

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func TestPVCRename(t *testing.T) {
	ctx := context.Background()

	t.Run("Test rename-pvc in same namespace - successfully", func(t *testing.T) {
		o, err := initTestSetup(ctx,
			"test1-old", "test1",
			"test1-new", "test1")
		if err != nil {
			t.Errorf("testsetup failed - got error %q", err)
		}

		err = o.run(context.Background())
		if err != nil {
			t.Errorf("rename failed - got error %q", err)
		}

		err = checkRename(ctx, &o)
		if err != nil {
			t.Errorf("rename check failed - got error %q", err)
		}
	})

	t.Run("Test rename-pvc in different namespace - successfully", func(t *testing.T) {
		o, err := initTestSetup(ctx,
			"test2-old", "test2-old",
			"test2-new", "test2")
		if err != nil {
			t.Errorf("testsetup failed - got error %q", err)
		}

		err = o.run(context.Background())
		if err != nil {
			t.Errorf("rename failed - got error %q", err)
		}

		err = checkRename(ctx, &o)
		if err != nil {
			t.Errorf("rename check failed -got error %q", err)
		}
	})

	t.Run("Test rename-pvc with running pod - fail", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "test3"},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{
					Name: "test",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test3-old", ReadOnly: false,
						},
					},
				}},
			},
		}

		o, err := initTestSetup(ctx,
			"test3-old", "test3",
			"test3-new", "test3",
			&pod,
		)
		if err != nil {
			t.Errorf("testsetup failed - got error %q", err)
		}

		err = o.run(ctx)
		if !errors.Is(err, ErrVolumeMounted) {
			t.Errorf("expect %q error - but got %q", ErrVolumeMounted, err)
		}
	})

	t.Run("Test rename-pvc with already existing newPvc - fail", func(t *testing.T) {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "test4-new", Namespace: "test4"},
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeName: "test",
			},
		}

		o, err := initTestSetup(ctx,
			"test4-old", "test4",
			"test4-new", "test4",
			&pvc,
		)
		if err != nil {
			t.Errorf("testsetup failed - got error %q", err)
		}

		ll, _ := o.k8sClient.CoreV1().PersistentVolumeClaims("default").List(ctx, metav1.ListOptions{})
		_ = ll
		err = o.run(ctx)
		if !k8sErrors.IsAlreadyExists(err) {
			t.Errorf("expect already exists error - but got %q", err)
		}
	})
}

func TestConfirmCheck(t *testing.T) {
	t.Run("Test confirmCheck with y", func(t *testing.T) {
		streams, in, _, _ := genericclioptions.NewTestIOStreams()
		o := renamePVCOptions{streams: streams}
		in.WriteString("y\n")
		if err := o.confirmCheck(); err != nil {
			t.Errorf("confirmCheck - got error %q", err)
		}
	})
	t.Run("Test confirmCheck with skip flag", func(t *testing.T) {
		streams, _, _, _ := genericclioptions.NewTestIOStreams()
		o := renamePVCOptions{streams: streams, confirm: true}
		if err := o.confirmCheck(); err != nil {
			t.Errorf("confirmCheck - got error %q", err)
		}
	})
	t.Run("Test confirmCheck with n", func(t *testing.T) {
		streams, in, _, _ := genericclioptions.NewTestIOStreams()
		o := renamePVCOptions{streams: streams}
		in.WriteString("n\n")
		if err := o.confirmCheck(); !errors.Is(err, ErrConfirmationNotSuccessful) {
			t.Errorf("expect %q - got error %q", ErrConfirmationNotSuccessful, err)
		}
	})
	t.Run("Test confirmCheck with unknown", func(t *testing.T) {
		streams, in, _, _ := genericclioptions.NewTestIOStreams()
		o := renamePVCOptions{streams: streams}
		in.WriteString("unknown\n")
		if err := o.confirmCheck(); !errors.Is(err, ErrConfirmationUnknown) {
			t.Errorf("expect %q - got error %q", ErrConfirmationUnknown, err)
		}
	})
}

// initTestSetup returns renamePVCOptions initialized with a kubernetes fake client and created oldPvc for test
func initTestSetup(
	ctx context.Context,
	oldName, sourceNamespace string,
	newName, targetNamespace string,
	extraObjects ...runtime.Object,
) (renamePVCOptions, error) {
	// init client
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	client := fake.NewSimpleClientset(extraObjects...)
	// the fake client did not set volumes to bound
	// add reactor to set every volume to Phase bound
	client.PrependReactor(
		"create",
		"persistentvolumeclaims",
		func(action k8sTesting.Action) (bool, runtime.Object, error) {
			obj := action.(k8sTesting.CreateAction).GetObject()
			pvc, ok := obj.(*corev1.PersistentVolumeClaim)
			if !ok {
				return false, obj, nil
			}
			pvc.Status.Phase = corev1.ClaimBound
			return false, obj, nil
		})

	o := renamePVCOptions{
		streams:         streams,
		k8sClient:       client,
		confirm:         true,
		oldName:         oldName,
		newName:         newName,
		sourceNamespace: sourceNamespace,
		targetNamespace: targetNamespace,
	}

	// create pvc
	pvName := sourceNamespace + "-" + oldName + "-pv"
	pvc, err := o.k8sClient.CoreV1().PersistentVolumeClaims(sourceNamespace).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldName,
			Namespace: sourceNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: pvName,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return renamePVCOptions{}, err
	}

	// create pv
	_, err = o.k8sClient.CoreV1().PersistentVolumes().Create(ctx, &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Kind:            pvc.Kind,
				Namespace:       pvc.Namespace,
				Name:            pvc.Name,
				UID:             pvc.UID,
				APIVersion:      pvc.APIVersion,
				ResourceVersion: pvc.ResourceVersion,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return renamePVCOptions{}, err
	}
	return o, nil
}

func checkRename(ctx context.Context, o *renamePVCOptions) error {
	newPVC, err := o.k8sClient.CoreV1().PersistentVolumeClaims(o.targetNamespace).Get(ctx, o.newName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	pv, err := o.k8sClient.CoreV1().PersistentVolumes().Get(ctx, newPVC.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if pv.Spec.ClaimRef.Name != newPVC.Name &&
		pv.Spec.ClaimRef.Namespace != newPVC.Namespace {
		return errors.New("pv claimRef wrong") // nolint: goerr113 // in test okay
	}

	_, err = o.k8sClient.CoreV1().PersistentVolumeClaims(o.targetNamespace).Get(ctx, o.newName, metav1.GetOptions{})
	if !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}
