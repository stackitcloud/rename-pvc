package renamepvc

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

func getK8sClient(configFlags *genericclioptions.ConfigFlags) (*kubernetes.Clientset, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func createNewPVC(oldPvc *corev1.PersistentVolumeClaim, newName, targetNamespace string) *corev1.PersistentVolumeClaim {
	newPvc := oldPvc.DeepCopy()
	newPvc.Status = corev1.PersistentVolumeClaimStatus{}
	newPvc.Name = newName
	newPvc.UID = ""
	newPvc.CreationTimestamp = metav1.Now()
	newPvc.SelfLink = ""
	newPvc.ResourceVersion = ""
	newPvc.Namespace = targetNamespace
	return newPvc
}

func createClaimRef(pvc *corev1.PersistentVolumeClaim) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:            pvc.Kind,
		Namespace:       pvc.Namespace,
		Name:            pvc.Name,
		UID:             pvc.UID,
		APIVersion:      pvc.APIVersion,
		ResourceVersion: pvc.ResourceVersion,
	}
}
