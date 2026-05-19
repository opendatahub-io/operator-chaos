package opendatahub_test

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk/fuzz"
)

func FuzzODHReconciler(f *testing.F) {
	f.Add(uint16(0x01FF), uint8(0), uint16(32768))
	f.Add(uint16(0), uint8(3), uint16(65535))
	f.Add(uint16(1), uint8(1), uint16(0))
	f.Add(uint16(0xFFFF), uint8(255), uint16(1))
	f.Add(uint16(0x0003), uint8(2), uint16(49152))

	scheme := testScheme()

	f.Fuzz(func(t *testing.T, opMask uint16, faultType uint8, intensity uint16) {
		dsc := newDSC()

		h := fuzz.NewHarness(odhReconcilerFactory, scheme, dscRequest(), dsc)
		h.AddInvariant(fuzz.ObjectExists(dscKey(), &unstructured.Unstructured{}))
		h.AddInvariant(finalizerValid(dscKey(), dscGVK, []string{dscFinalizer}))
		h.SetTimeout(5 * time.Second)

		fc := fuzz.DecodeFaultConfig(opMask, faultType, intensity)
		_ = h.Run(t, fc)
	})
}

func FuzzDSCIReconciler(f *testing.F) {
	f.Add(uint16(0x01FF), uint8(0), uint16(32768))
	f.Add(uint16(0), uint8(3), uint16(65535))
	f.Add(uint16(1), uint8(1), uint16(0))
	f.Add(uint16(0x0007), uint8(4), uint16(16384))

	scheme := testScheme()

	f.Fuzz(func(t *testing.T, opMask uint16, faultType uint8, intensity uint16) {
		dsci := newDSCI()

		h := fuzz.NewHarness(dsciReconcilerFactory, scheme, dsciRequest(), dsci)
		h.AddInvariant(fuzz.ObjectExists(dsciKey(), &unstructured.Unstructured{}))
		h.AddInvariant(finalizerValid(dsciKey(), dsciGVK, []string{dsciFinalizer}))
		h.SetTimeout(5 * time.Second)

		fc := fuzz.DecodeFaultConfig(opMask, faultType, intensity)
		_ = h.Run(t, fc)
	})
}
