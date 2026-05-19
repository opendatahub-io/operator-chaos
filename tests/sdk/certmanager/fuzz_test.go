package certmanager_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk/fuzz"
)

func FuzzCertManagerReconciler(f *testing.F) {
	f.Add(uint16(0x01FF), uint8(0), uint16(32768))
	f.Add(uint16(0), uint8(3), uint16(65535))
	f.Add(uint16(1), uint8(1), uint16(0))
	f.Add(uint16(0xFFFF), uint8(255), uint16(1))
	f.Add(uint16(0x000C), uint8(2), uint16(49152))

	scheme := testScheme()

	f.Fuzz(func(t *testing.T, opMask uint16, faultType uint8, intensity uint16) {
		cr := certManagerCR()

		h := fuzz.NewHarness(certManagerReconcilerFactory, scheme, certManagerRequest(), cr)
		h.AddInvariant(fuzz.ObjectExists(certManagerKey(), &unstructured.Unstructured{}))
		h.AddInvariant(noDeploymentDeleted())
		h.SetTimeout(5 * time.Second)

		fc := fuzz.DecodeFaultConfig(opMask, faultType, intensity)
		_ = h.Run(t, fc)
	})
}

func noDeploymentDeleted() fuzz.Invariant {
	return func(ctx context.Context, c client.Client) error {
		deployments := &appsv1.DeploymentList{}
		if err := c.List(ctx, deployments); err != nil {
			return nil
		}
		for _, d := range deployments.Items {
			if d.DeletionTimestamp != nil {
				return fmt.Errorf("deployment %s is being deleted, reconciler should not delete deployments", d.Name)
			}
		}
		return nil
	}
}
