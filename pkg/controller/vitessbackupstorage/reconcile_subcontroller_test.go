/*
Copyright 2026 PlanetScale Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vitessbackupstorage

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

// TestReconcileSubcontrollerUpdatesExistingPodWhenExtraLabelsAreAdded protects
// upgrades where a subcontroller pod already exists before backup extra labels
// are configured.
func TestReconcileSubcontrollerUpdatesExistingPodWhenExtraLabelsAreAdded(t *testing.T) {
	t.Setenv("PS_OPERATOR_POD_NAMESPACE", "default")
	t.Setenv("PS_OPERATOR_POD_NAME", "vitess-operator")

	vbs := &planetscalev2.VitessBackupStorage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-primary",
			Namespace: "default",
			UID:       types.UID("vitessbackupstorage-uid"),
			Labels: map[string]string{
				planetscalev2.ClusterLabel: "example",
			},
		},
		Spec: planetscalev2.VitessBackupStorageSpec{
			Location: planetscalev2.VitessBackupLocation{
				Name: "primary",
			},
			ExtraLabels: map[string]string{
				"backup.planetscale.com/team": "platform",
			},
		},
	}

	scheme := newSubcontrollerTestScheme()
	recorder := record.NewFakeRecorder(20)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vbs, newSubcontrollerTestParentPod()).
		Build()

	r := &ReconcileVitessBackupStorage{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}

	spec, err := r.newSubcontrollerPodSpec(t.Context(), vbs)
	require.NoError(t, err)

	podKey := client.ObjectKey{
		Namespace: "default",
		Name:      "example-primary-vitessbackupstorage-subcontroller",
	}
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   podKey.Namespace,
			Name:        podKey.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				planetscalev2.ComponentLabel: planetscalev2.VBSSubcontrollerComponentName,
				planetscalev2.ClusterLabel:   "example",
				vitessbackup.LocationLabel:   "primary",
			},
		},
		Spec: *spec.DeepCopy(),
	}
	updateSubcontrollerPod(existingPod, spec)

	require.NoError(t, k8sClient.Create(t.Context(), existingPod))

	_, err = r.reconcileSubcontroller(t.Context(), vbs)
	require.NoError(t, err)

	gotPod := &corev1.Pod{}
	require.NoError(t, k8sClient.Get(t.Context(), podKey, gotPod))
	require.Equal(t, "platform", gotPod.Labels["backup.planetscale.com/team"])
}

// newSubcontrollerTestParentPod returns the parent operator pod used by the fork
// code while building the child subcontroller pod spec.
func newSubcontrollerTestParentPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vitess-operator",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "vitess-operator",
			Containers: []corev1.Container{
				{
					Name:  "vitess-operator",
					Image: "planetscale/vitess-operator:test",
				},
			},
		},
	}
}

// newSubcontrollerTestScheme contains the Kubernetes types touched by the
// subcontroller reconciliation path.
func newSubcontrollerTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = planetscalev2.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}
