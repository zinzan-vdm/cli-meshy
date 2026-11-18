package plan

import (
	"errors"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// lifetime is the period that mesh.yml writes as 20y.
const lifetime = 20 * 365 * 24 * time.Hour

// caCert is one authority certificate as a node presents it.
func caCert(name, fingerprint string, start time.Time) node.Cert {
	return node.Cert{
		Present:     true,
		Name:        name,
		Fingerprint: fingerprint,
		IsCA:        true,
		NotBefore:   start.Format(time.RFC3339),
		NotAfter:    start.Add(lifetime).Format(time.RFC3339),
	}
}

// rotationMesh is a mesh whose trust and active name are the ones under test.
func rotationMesh(trust ...string) config.Mesh {
	m := testMesh()
	m.CA.Trust = trust
	m.CA.Active = trust[len(trust)-1]
	return m
}

// withBundle puts one bundle on every readable node, and gives each node
// certificate the issuer in the map.
func withBundle(t *testing.T, m config.Mesh, bundle []node.Cert, issuers map[string]string) []Reading {
	t.Helper()
	readings := healthyReadings(t, m)
	for i := range readings {
		readings[i].State.Authorities = bundle
		readings[i].State.Cert.Issuer = issuers[readings[i].Name]
	}
	return readings
}

func TestComputeReportsTheAgeOfTheActiveAuthority(t *testing.T) {
	start := now.Add(-2 * 24 * time.Hour)
	m := rotationMesh("ca-1")
	bundle := []node.Cert{caCert("ca-1", "1111aaaa", start)}

	report := Compute(Input{Mesh: m, Readings: withBundle(t, m, bundle, nil), Now: now})

	if report.Rotation == nil {
		t.Fatal("the plan reports no rotation for a bundle that holds dates")
	}
	r := report.Rotation
	if r.Authority != "ca-1" {
		t.Errorf("authority = %q", r.Authority)
	}
	if r.NotBefore != start.Format(time.RFC3339) {
		t.Errorf("not_before = %q", r.NotBefore)
	}
	if r.Life() != "20y" {
		t.Errorf("life = %q, want 20y", r.Life())
	}
	if r.Used != 0 {
		t.Errorf("used = %v, want one decimal of 0", r.Used)
	}
	if r.Due {
		t.Error("an authority two days old is due")
	}
	if r.Next != "phase 1" {
		t.Errorf("next = %q", r.Next)
	}
	if got := r.PhaseDate().Format("2006-01-02"); got != start.Add(lifetime*8/10).Format("2006-01-02") {
		t.Errorf("phase date = %s", got)
	}
}

func TestComputeReportsDueAtTheThreshold(t *testing.T) {
	cases := []struct {
		name  string
		share float64
		due   bool
	}{
		{name: "just under", share: 0.799, due: false},
		{name: "at the threshold", share: 0.8, due: true},
		{name: "past the threshold", share: 0.801, due: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start := now.Add(-time.Duration(float64(lifetime) * c.share))
			m := rotationMesh("ca-1")
			bundle := []node.Cert{caCert("ca-1", "1111aaaa", start)}

			report := Compute(Input{Mesh: m, Readings: withBundle(t, m, bundle, nil), Now: now})
			if report.Rotation == nil {
				t.Fatal("the plan reports no rotation")
			}
			if report.Rotation.Due != c.due {
				t.Errorf("due = %v at %v%%, want %v", report.Rotation.Due, report.Rotation.Used, c.due)
			}
			if report.Rotation.RotateAt != 80 {
				t.Errorf("rotate_at = %v, want 80", report.Rotation.RotateAt)
			}
		})
	}
}

func TestComputeReportsPhase2WhileANodeHoldsTheOldCertificate(t *testing.T) {
	start := now.Add(-time.Hour)
	m := rotationMesh("ca-1", "ca-2")
	bundle := []node.Cert{
		caCert("ca-1", "1111aaaa", start),
		caCert("ca-2", "9459fea2", start),
	}
	issuers := map[string]string{"core-1": "9459fea2", "web-1": "1111aaaa"}

	report := Compute(Input{Mesh: m, Readings: withBundle(t, m, bundle, issuers), Now: now})

	if report.Rotation == nil {
		t.Fatal("the plan reports no rotation")
	}
	if report.Rotation.Next != "phase 2" {
		t.Errorf("next = %q, want phase 2", report.Rotation.Next)
	}
	if report.Rotation.Authority != "ca-2" {
		t.Errorf("authority = %q, want the active authority", report.Rotation.Authority)
	}
}

func TestComputeReportsPhase3WhenEveryNodeSignedAgain(t *testing.T) {
	start := now.Add(-time.Hour)
	m := rotationMesh("ca-1", "ca-2")
	bundle := []node.Cert{
		caCert("ca-1", "1111aaaa", start),
		caCert("ca-2", "9459fea2", start),
	}
	issuers := map[string]string{"core-1": "9459fea2", "web-1": "9459fea2"}

	report := Compute(Input{Mesh: m, Readings: withBundle(t, m, bundle, issuers), Now: now})

	if report.Rotation == nil {
		t.Fatal("the plan reports no rotation")
	}
	if report.Rotation.Next != "phase 3" {
		t.Errorf("next = %q, want phase 3", report.Rotation.Next)
	}
}

// A short issuer is still the same authority, because nebula-cert writes the
// fingerprint that the certificate carries.
func TestSignedByAcceptsAShortIssuer(t *testing.T) {
	if !signedBy("9459", "9459fea2") {
		t.Error("a four character issuer does not name its authority")
	}
	if signedBy("9459", "1111aaaa") {
		t.Error("an issuer named another authority")
	}
	if signedBy("94", "9459fea2") {
		t.Error("a two character issuer is too short to name an authority")
	}
}

func TestComputeReportsNoRotationWithoutABundle(t *testing.T) {
	m := rotationMesh("ca-1")

	// The fixture bundle holds no dates, which is the shape a node returns when
	// the certificate cannot be read.
	report := Compute(Input{Mesh: m, Readings: healthyReadings(t, m), Now: now})
	if report.Rotation != nil {
		t.Errorf("the plan invented a rotation: %+v", report.Rotation)
	}

	failed := []Reading{{Name: "core-1", Err: errors.New("node core-1: read failed")}}
	report = Compute(Input{Mesh: m, Readings: failed, Now: now})
	if report.Rotation != nil {
		t.Errorf("the plan reported a rotation for a node it could not read: %+v", report.Rotation)
	}
}
