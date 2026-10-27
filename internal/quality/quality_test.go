package quality

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"testing"
)

func TestMetricsAndThresholds(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	b := image.NewNRGBA(image.Rect(3, 4, 4, 5))
	a.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 255})
	b.SetNRGBA(3, 4, color.NRGBA{10, 10, 10, 255})
	m := Measure(a, b)
	want := 10 * math.Log10(255*255/100.0)
	if m.DB == nil || math.Abs(*m.DB-want) > 1e-10 {
		t.Fatal(m, want)
	}
	if Measure(a, a).Status != "infinite" || Measure(a, image.NewNRGBA(image.Rect(0, 0, 2, 1))).Status != "unavailable" {
		t.Fatal("metric states")
	}
	for _, delta := range []float64{-0.01, 0, 0.01} {
		v := want + delta
		p := Policy{&v}
		if p.Accept(m) != (delta <= 0) {
			t.Fatal(delta)
		}
	}
	n := 35.0
	p := Policy{&n}
	if !p.Accept(Measure(a, a)) || p.Accept(Metric{Status: "unavailable"}) || !(Policy{}).Accept(Metric{Status: "unavailable"}) {
		t.Fatal("gating")
	}
	for _, v := range []float64{19, 61, math.NaN(), math.Inf(1)} {
		if (Policy{&v}).Valid() {
			t.Fatal(v)
		}
	}
	data, err := json.Marshal(Measure(a, a))
	if err != nil || string(data) != `{"status":"infinite"}` {
		t.Fatal(string(data), err)
	}
}
