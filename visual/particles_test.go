package visual

import "testing"

func TestNewParticleSystem(t *testing.T) {
	ps := NewParticleSystem(30, 80, 24)
	if len(ps.particles) != 30 {
		t.Errorf("particle count = %d, want 30", len(ps.particles))
	}
}

func TestParticleSystemUpdate(t *testing.T) {
	ps := NewParticleSystem(10, 80, 24)
	initialX := ps.particles[0].x
	initialY := ps.particles[0].y

	ps.Update(0.5, "#ff0000", "#00ff00")

	moved := false
	for _, p := range ps.particles {
		if p.x != initialX || p.y != initialY {
			moved = true
			break
		}
	}
	if !moved {
		t.Error("expected at least one particle to move after update")
	}
}

func TestParticleSystemRender(t *testing.T) {
	ps := NewParticleSystem(5, 40, 10)
	ps.Update(0.5, "#ff0000", "#00ff00")
	grid := ps.Render()
	if len(grid) != 10 {
		t.Errorf("grid height = %d, want 10", len(grid))
	}
	if len(grid[0]) != 40 {
		t.Errorf("grid width = %d, want 40", len(grid[0]))
	}
}

func TestParticleSystemResize(t *testing.T) {
	ps := NewParticleSystem(10, 80, 24)
	ps.Resize(120, 40)
	if ps.width != 120 || ps.height != 40 {
		t.Errorf("resize failed: got %dx%d", ps.width, ps.height)
	}
	grid := ps.Render()
	if len(grid) != 40 {
		t.Errorf("grid height after resize = %d, want 40", len(grid))
	}
}
