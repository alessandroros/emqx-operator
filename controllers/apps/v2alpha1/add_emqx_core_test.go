package v2alpha1

import (
	"testing"

	appsv2alpha1 "github.com/emqx/emqx-operator/apis/apps/v2alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var coreLabels = map[string]string{
	"apps.emqx.io/instance":   "emqx",
	"apps.emqx.io/managed-by": "emqx-operator",
	"apps.emqx.io/db-role":    "core",
}

func TestGenerateStatefulSet(t *testing.T) {
	instance := &appsv2alpha1.EMQX{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "emqx",
			Namespace: "emqx",
		},
		Spec: appsv2alpha1.EMQXSpec{
			Image: "emqx/emqx:5.0",
			CoreTemplate: appsv2alpha1.EMQXCoreTemplate{
				Spec: appsv2alpha1.EMQXCoreTemplateSpec{
					Replicas: &[]int32{3}[0],
				},
			},
		},
	}
	instance.Default()
	assert.Nil(t, instance.ValidateCreate())

	t.Run("check metadata", func(t *testing.T) {
		emqx := instance.DeepCopy()
		emqx.Annotations = map[string]string{
			"kubectl.kubernetes.io/last-applied-configuration": "fake",
		}

		got := generateStatefulSet(emqx)
		assert.Equal(t, coreLabels, got.Labels)
		assert.NotContains(t, "kubectl.kubernetes.io/last-applied-configuration", got.Annotations)
	})

	t.Run("check sts spec", func(t *testing.T) {
		emqx := instance.DeepCopy()

		got := generateStatefulSet(emqx)
		assert.Equal(t, int32(3), *got.Spec.Replicas)
		assert.Equal(t, "emqx-headless", got.Spec.ServiceName)
		assert.Equal(t, coreLabels, got.Spec.Selector.MatchLabels)
		assert.Equal(t, appsv1.ParallelPodManagement, got.Spec.PodManagementPolicy)
	})

	t.Run("check sts template metadata", func(t *testing.T) {
		emqx := instance.DeepCopy()
		emqx.Spec.CoreTemplate.Spec.ExtraContainers = []corev1.Container{{Name: "fake-container"}}

		got := generateStatefulSet(emqx)
		assert.Equal(t, coreLabels, got.Spec.Template.ObjectMeta.Labels)
		assert.Equal(t, map[string]string{
			"apps.emqx.io/headless-service-name": "emqx-headless",
			"apps.emqx.io/manage-containers":     "emqx,fake-container",
		}, got.Spec.Template.ObjectMeta.Annotations)
	})

	t.Run("check sts template spec", func(t *testing.T) {
		emqx := instance.DeepCopy()

		emqx.Spec.CoreTemplate.Spec.Affinity = &corev1.Affinity{}
		emqx.Spec.CoreTemplate.Spec.ToleRations = []corev1.Toleration{{Key: "fake"}}
		emqx.Spec.CoreTemplate.Spec.NodeSelector = map[string]string{"fake": "fake"}
		emqx.Spec.CoreTemplate.Spec.NodeName = "fake"
		got := generateStatefulSet(emqx)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Affinity, got.Spec.Template.Spec.Affinity)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.ToleRations, got.Spec.Template.Spec.Tolerations)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.NodeSelector, got.Spec.Template.Spec.NodeSelector)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.NodeName, got.Spec.Template.Spec.NodeName)

		emqx.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "fake-secret"}}
		got = generateStatefulSet(emqx)
		assert.Equal(t, emqx.Spec.ImagePullSecrets, got.Spec.Template.Spec.ImagePullSecrets)

		emqx.Spec.CoreTemplate.Spec.PodSecurityContext = &corev1.PodSecurityContext{
			RunAsUser:  &[]int64{1001}[0],
			RunAsGroup: &[]int64{1001}[0],
			FSGroup:    &[]int64{1001}[0],
		}
		got = generateStatefulSet(emqx)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.PodSecurityContext, got.Spec.Template.Spec.SecurityContext)

		emqx.Spec.CoreTemplate.Spec.InitContainers = []corev1.Container{{Name: "fake-init-container"}}
		got = generateStatefulSet(emqx)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.InitContainers, got.Spec.Template.Spec.InitContainers)
	})

	t.Run("check sts template spec containers", func(t *testing.T) {
		emqx := instance.DeepCopy()

		emqx.Spec.CoreTemplate.Spec.ExtraContainers = []corev1.Container{{Name: "fake-container"}}
		got := generateStatefulSet(emqx)
		assert.Len(t, got.Spec.Template.Spec.Containers, 2)

		emqx.Spec.Image = "emqx/emqx:5.0"
		emqx.Spec.ImagePullPolicy = corev1.PullIfNotPresent
		emqx.Spec.CoreTemplate.Spec.Command = []string{"fake"}
		emqx.Spec.CoreTemplate.Spec.Args = []string{"fake"}
		emqx.Spec.CoreTemplate.Spec.Ports = []corev1.ContainerPort{{Name: "fake"}}
		emqx.Spec.CoreTemplate.Spec.Env = []corev1.EnvVar{{Name: "foo", Value: "bar"}}
		emqx.Spec.CoreTemplate.Spec.EnvFrom = []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "fake-config",
					},
				},
			},
		}
		emqx.Spec.CoreTemplate.Spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		}
		emqx.Spec.CoreTemplate.Spec.ContainerSecurityContext = &corev1.SecurityContext{
			RunAsUser:    &[]int64{1001}[0],
			RunAsGroup:   &[]int64{1001}[0],
			RunAsNonRoot: &[]bool{true}[0],
		}
		emqx.Spec.CoreTemplate.Spec.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/status",
					Port: intstr.FromInt(18083),
				},
			},
			InitialDelaySeconds: int32(10),
			PeriodSeconds:       int32(5),
			FailureThreshold:    int32(30),
		}
		emqx.Spec.CoreTemplate.Spec.Lifecycle = &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"emqx", "ctl", "cluster", "leave"},
				},
			},
		}
		emqx.Spec.CoreTemplate.Spec.ExtraVolumeMounts = []corev1.VolumeMount{{Name: "fake-volume-mount"}}

		got = generateStatefulSet(emqx)
		assert.Equal(t, emqx.Spec.Image, got.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, emqx.Spec.ImagePullPolicy, got.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Command, got.Spec.Template.Spec.Containers[0].Command)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Args, got.Spec.Template.Spec.Containers[0].Args)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Ports, got.Spec.Template.Spec.Containers[0].Ports)
		assert.Equal(t, []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "STS_HEADLESS_SERVICE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['apps.emqx.io/headless-service-name']",
					},
				},
			},
			{
				Name:  "EMQX_HOST",
				Value: "$(POD_NAME).$(STS_HEADLESS_SERVICE_NAME).$(POD_NAMESPACE).svc.cluster.local",
			},
			{
				Name:  "EMQX_NODE__DB_ROLE",
				Value: "core",
			},
			{
				Name: "EMQX_NODE__COOKIE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: emqx.NodeCookieNamespacedName().Name,
						},
						Key: "node_cookie",
					},
				},
			},
			{
				Name:  "EMQX_DASHBOARD__BOOTSTRAP_USERS_FILE",
				Value: `"/opt/emqx/data/bootstrap_user"`,
			},
			{
				Name:  "foo",
				Value: "bar",
			},
		}, got.Spec.Template.Spec.Containers[0].Env)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.EnvFrom, got.Spec.Template.Spec.Containers[0].EnvFrom)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Resources, got.Spec.Template.Spec.Containers[0].Resources)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.ContainerSecurityContext, got.Spec.Template.Spec.Containers[0].SecurityContext)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.ReadinessProbe, got.Spec.Template.Spec.Containers[0].ReadinessProbe)
		assert.Equal(t, emqx.Spec.CoreTemplate.Spec.Lifecycle, got.Spec.Template.Spec.Containers[0].Lifecycle)
		assert.Equal(t, []corev1.VolumeMount{
			{
				Name:      "bootstrap-user",
				MountPath: "/opt/emqx/data/bootstrap_user",
				SubPath:   "bootstrap_user",
				ReadOnly:  true,
			},
			{
				Name:      "bootstrap-config",
				MountPath: "/opt/emqx/etc/emqx.conf",
				SubPath:   "emqx.conf",
				ReadOnly:  true,
			},
			{
				Name:      "emqx-core-data",
				MountPath: "/opt/emqx/data",
			},
			{
				Name: "fake-volume-mount",
			},
		}, got.Spec.Template.Spec.Containers[0].VolumeMounts)
	})

	t.Run("check sts spec volume", func(t *testing.T) {
		emqx := instance.DeepCopy()
		emqx.Spec.CoreTemplate.Spec.ExtraVolumes = []corev1.Volume{{Name: "fake-volume"}}

		got := generateStatefulSet(emqx)
		assert.Equal(t, []corev1.Volume{
			{
				Name: "emqx-core-data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "bootstrap-user",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: emqx.BootstrapUserNamespacedName().Name,
					},
				},
			},
			{
				Name: "bootstrap-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: emqx.BootstrapConfigNamespacedName().Name,
						},
					},
				},
			},
			{
				Name: "fake-volume",
			},
		}, got.Spec.Template.Spec.Volumes)
	})

	t.Run("check sts volume claim templates", func(t *testing.T) {
		emqx := instance.DeepCopy()
		emqx.Spec.CoreTemplate.Spec.VolumeClaimTemplates = corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Mi"),
				},
			},
		}

		got := generateStatefulSet(emqx)
		assert.Equal(t, []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "emqx-core-data",
					Namespace: "emqx",
					Labels:    coreLabels,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("20Mi"),
						},
					},
				},
			},
		}, got.Spec.VolumeClaimTemplates)
		assert.NotContains(t, got.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "emqx-core-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	})
}
