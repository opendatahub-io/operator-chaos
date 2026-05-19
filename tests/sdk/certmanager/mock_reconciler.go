package certmanager

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var certManagerGVK = schema.GroupVersionKind{
	Group:   "operator.openshift.io",
	Version: "v1alpha1",
	Kind:    "CertManager",
}

// CertManagerReconciler mimics the cert-manager-operator's deployment management pattern.
// It uses Get + Update (simulating SSA behavior) to enforce desired state on each reconcile.
type CertManagerReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *CertManagerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	cm := &unstructured.Unstructured{}
	cm.SetGroupVersionKind(certManagerGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, cm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.ensureDeployment(ctx, "cert-manager", "cert-manager", "cert-manager-controller"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure controller deployment: %w", err)
	}

	if err := r.ensureDeployment(ctx, "cert-manager-webhook", "cert-manager", "cert-manager-webhook"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure webhook deployment: %w", err)
	}

	if err := r.ensureDeployment(ctx, "cert-manager-cainjector", "cert-manager", "cert-manager-cainjector"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure cainjector deployment: %w", err)
	}

	if err := r.ensureService(ctx, "cert-manager-webhook", "cert-manager"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure webhook service: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *CertManagerReconciler) ensureDeployment(ctx context.Context, name, namespace, container string) error {
	desired := buildDeployment(name, namespace, container)

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if k8serrors.IsNotFound(err) {
		return r.Client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Enforce spec: replicas, image, labels (simulates SSA behavior)
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	existing.Labels = desired.Labels
	return r.Client.Update(ctx, existing)
}

func (r *CertManagerReconciler) ensureService(ctx context.Context, name, namespace string) error {
	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{{
				Port:       443,
				TargetPort: intstr.FromInt32(10250),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}

	existing := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if k8serrors.IsNotFound(err) {
		return r.Client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Ports = desired.Spec.Ports
	return r.Client.Update(ctx, existing)
}

func buildDeployment(name, namespace, container string) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  container,
						Image: fmt.Sprintf("quay.io/jetstack/%s:v1.14.0", container),
					}},
				},
			},
		},
	}
}
