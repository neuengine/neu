package ui

import (
	"testing"

	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

func TestValResolve(t *testing.T) {
	t.Parallel()
	const parent, vw, vh = 200, 1920, 1080
	cases := []struct {
		v    Val
		want float32
		ok   bool
	}{
		{Px(50), 50, true},
		{Percent(25), 50, true}, // 25% of 200
		{Vw(10), 192, true},     // 10% of 1920
		{Vh(50), 540, true},     // 50% of 1080
		{Auto, 0, false},
	}
	for _, c := range cases {
		got, ok := c.v.Resolve(parent, vw, vh)
		if got != c.want || ok != c.ok {
			t.Errorf("%+v.Resolve = %v,%v want %v,%v", c.v, got, ok, c.want, c.ok)
		}
	}
}

func TestLayoutRectContains(t *testing.T) {
	t.Parallel()
	r := LayoutRect{Position: pkgmath.Vec2{X: 10, Y: 10}, Size: pkgmath.Vec2{X: 100, Y: 50}}
	if !r.Contains(pkgmath.Vec2{X: 50, Y: 30}) {
		t.Error("interior point should be contained")
	}
	if r.Contains(pkgmath.Vec2{X: 200, Y: 30}) {
		t.Error("point right of rect should not be contained")
	}
	if !r.Contains(pkgmath.Vec2{X: 10, Y: 10}) {
		t.Error("top-left corner is inclusive")
	}
}

func TestZIndexEffective(t *testing.T) {
	t.Parallel()
	if (ZIndex{Local: 3}).Effective() != 3 {
		t.Error("local z")
	}
	if (ZIndex{Global: 9, UseGlobal: true}).Effective() != 9 {
		t.Error("global z")
	}
}

func TestFlexDirection(t *testing.T) {
	t.Parallel()
	if !Row.IsRow() || RowReverse.IsRow() != true || Column.IsRow() {
		t.Error("IsRow classification")
	}
	if !RowReverse.IsReverse() || !ColumnReverse.IsReverse() || Row.IsReverse() {
		t.Error("IsReverse classification")
	}
}

func TestInteractionString(t *testing.T) {
	t.Parallel()
	if InteractionNone.String() != "None" || InteractionHovered.String() != "Hovered" || InteractionPressed.String() != "Pressed" {
		t.Error("Interaction.String mismatch")
	}
}

func TestPlainText(t *testing.T) {
	t.Parallel()
	txt := PlainText("hi", asset.Handle[Font]{}, 16, pkgmath.LinearRgba{A: 1})
	if len(txt.Sections) != 1 || txt.Sections[0].Value != "hi" || txt.Sections[0].FontSize != 16 {
		t.Errorf("PlainText section = %+v", txt.Sections)
	}
}
