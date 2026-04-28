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

package vitessbackupschedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestCreateJobVtbackupAppliesTabletPoolExtraLabelsToJobTemplate protects the
// scheduled vtbackup path, where a Pod object is converted into a Job template.
func TestCreateJobVtbackupAppliesTabletPoolExtraLabelsToJobTemplate(t *testing.T) {
	scheme := newScheme()
	vts := newVtbackupExtraLabelsShard()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vts).Build(),
		scheme: scheme,
	}

	vbsc := newVtbackupExtraLabelsSchedule()
	strategy := vbsc.Spec.Strategy[0]

	job, err := r.createJob(
		t.Context(),
		vbsc,
		strategy,
		time.Unix(1700000000, 0),
		planetscalev2.VitessKeyRange{},
	)
	require.NoError(t, err)

	require.Equal(t, "platform", job.Spec.Template.Labels["backup.planetscale.com/team"])
}

// newVtbackupExtraLabelsSchedule returns the smallest schedule object needed to
// build a vtbackup Job through the controller path.
func newVtbackupExtraLabelsSchedule() planetscalev2.VitessBackupSchedule {
	return planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			UID:               types.UID("vitessbackupschedule-uid"),
			ResourceVersion:   "1",
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "example",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:         "daily",
				Schedule:     "0 0 * * *",
				BackupMethod: planetscalev2.BackupMethodVtbackup,
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{
						Name:     "commerce-x",
						Keyspace: "commerce",
						Shard:    "-",
					},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}
}

// newVtbackupExtraLabelsShard returns a ready shard whose first tablet pool
// carries the user labels that scheduled vtbackup should place on its pod.
func newVtbackupExtraLabelsShard() *planetscalev2.VitessShard {
	initBackup := true

	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: []planetscalev2.VitessShardTabletPool{
					{
						Cell:     "zone1",
						Type:     planetscalev2.ReplicaPoolType,
						Replicas: 1,
						Vttablet: planetscalev2.VttabletSpec{},
						Mysqld:   &planetscalev2.MysqldSpec{},
						DataVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("10Gi"),
								},
							},
						},
						ExtraLabels: map[string]string{
							"backup.planetscale.com/team": "platform",
						},
					},
				},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: &initBackup,
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/lite:mysql80",
				Vttablet: "vitess/lite:mysql80",
				Vtorc:    "vitess/lite:mysql80",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:mysql80",
				},
			},
			KeyRange: planetscalev2.VitessKeyRange{},
			BackupLocations: []planetscalev2.VitessBackupLocation{
				{
					Name: "",
				},
			},
		},
		Status: readyShardStatus(),
	}
}
