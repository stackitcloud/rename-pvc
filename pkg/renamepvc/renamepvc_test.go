package renamepvc

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestRenamePVCOptions_execute(t *testing.T) {
	tests := []struct {
		name           string
		oldPVCName     string
		newPVCName     string
		namespace      string
		initialObjects []runtime.Object
		expectedError  bool
	}{
		{
			name:       "Successful rename",
			oldPVCName: "old-pvc",
			newPVCName: "new-pvc",
			namespace:  "default",
			initialObjects: []runtime.Object{
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "old-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						VolumeName: "test-pv",
					},
				},
				&corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
					Spec: corev1.PersistentVolumeSpec{
						ClaimRef: &corev1.ObjectReference{
							Name:      "old-pvc",
							Namespace: "default",
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name:           "PVC not found",
			oldPVCName:     "non-existent-pvc",
			newPVCName:     "new-pvc",
			namespace:      "default",
			initialObjects: []runtime.Object{},
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake clientset with initial objects
			clientset := fake.NewSimpleClientset(tt.initialObjects...)

			// Mock PVC creation and binding process
			clientset.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(k8stesting.CreateAction)
				pvc := createAction.GetObject().(*corev1.PersistentVolumeClaim)
				pvc.Spec.VolumeName = "test-pv" // Ensure the new PVC has the correct VolumeName
				pvc.Status.Phase = corev1.ClaimBound
				return true, pvc, nil
			})

			clientset.PrependReactor("get", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				getAction := action.(k8stesting.GetAction)
				if getAction.GetName() == tt.newPVCName {
					pvc := &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      tt.newPVCName,
							Namespace: tt.namespace,
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							VolumeName: "test-pv",
						},
						Status: corev1.PersistentVolumeClaimStatus{
							Phase: corev1.ClaimBound,
						},
					}
					return true, pvc, nil
				}
				return false, nil, nil
			})

			// Create renamePVCOptions with a shorter timeout
			o := &renamePVCOptions{
				streams:         genericclioptions.NewTestIOStreamsDiscard(),
				k8sClient:       clientset,
				oldName:         tt.oldPVCName,
				newName:         tt.newPVCName,
				sourceNamespace: tt.namespace,
				targetNamespace: tt.namespace,
				confirm:         true,
			}

			// Override the waitUntilPvcIsBound function to use a shorter timeout
			originalWaitFunc := o.waitUntilPvcIsBound
			originalWaitFunc = func(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
				ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()
				return originalWaitFunc(ctx, pvc)
			}

			// Execute the rename operation
			err := o.execute(context.TODO())

			// Check if the error matches the expected outcome
			if (err != nil) != tt.expectedError {
				t.Errorf("execute() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if !tt.expectedError {
				// Check if the new PVC exists
				newPVC, err := clientset.CoreV1().PersistentVolumeClaims(tt.namespace).Get(context.TODO(), tt.newPVCName, metav1.GetOptions{})
				if err != nil {
					t.Errorf("New PVC not found: %v", err)
					return
				}

				// Check if the old PVC is deleted
				_, err = clientset.CoreV1().PersistentVolumeClaims(tt.namespace).Get(context.TODO(), tt.oldPVCName, metav1.GetOptions{})
				if err == nil {
					t.Errorf("Old PVC still exists")
					return
				}

				// Check if the new PVC has the correct spec
				if newPVC.Spec.VolumeName != "test-pv" {
					t.Errorf("New PVC has incorrect spec. Expected VolumeName 'test-pv', got '%s'", newPVC.Spec.VolumeName)
				}

				// Check if the PV's ClaimRef has been updated
				pv, err := clientset.CoreV1().PersistentVolumes().Get(context.TODO(), "test-pv", metav1.GetOptions{})
				if err != nil {
					t.Errorf("Failed to get PV: %v", err)
					return
				}
				if pv.Spec.ClaimRef.Name != tt.newPVCName {
					t.Errorf("PV ClaimRef not updated. Expected %s, got %s", tt.newPVCName, pv.Spec.ClaimRef.Name)
				}
			}
		})
	}
}

func TestRenamePVCOptions_checkIfMounted(t *testing.T) {
	tests := []struct {
		name           string
		pvcName        string
		namespace      string
		initialObjects []runtime.Object
		expectedError  bool
	}{
		{
			name:      "PVC not mounted",
			pvcName:   "test-pvc",
			namespace: "default",
			initialObjects: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-without-pvc",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{},
					},
				},
			},
			expectedError: false,
		},
		{
			name:      "PVC mounted",
			pvcName:   "test-pvc",
			namespace: "default",
			initialObjects: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-with-pvc",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "test-volume",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "test-pvc",
									},
								},
							},
						},
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake clientset with initial objects
			clientset := fake.NewSimpleClientset(tt.initialObjects...)

			// Create renamePVCOptions
			o := &renamePVCOptions{
				k8sClient: clientset,
			}

			// Create a dummy PVC
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.pvcName,
					Namespace: tt.namespace,
				},
			}

			// Check if the PVC is mounted
			err := o.checkIfMounted(context.TODO(), pvc)

			// Check if the error matches the expected outcome
			if (err != nil) != tt.expectedError {
				t.Errorf("checkIfMounted() error = %v, expectedError %v", err, tt.expectedError)
			}

			if tt.expectedError && err != nil {
				if !errors.Is(err, ErrVolumeMounted) {
					t.Errorf("Expected ErrVolumeMounted, got %v", err)
				}
			}
		})
	}
}
