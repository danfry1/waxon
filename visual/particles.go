package visual

import (
	"math/rand/v2"

	"github.com/charmbracelet/lipgloss"
)

var particleChars = []rune{'·', '•', '∘', '⋅', '◦'}

type particle struct {
	x, y   float64
	vx, vy float64
	char   rune
	alive  bool
}

type ParticleCell struct {
	Char  string
	Style lipgloss.Style
}

type ParticleSystem struct {
	particles []particle
	width     int
	height    int
}

func NewParticleSystem(count, width, height int) *ParticleSystem {
	ps := &ParticleSystem{
		particles: make([]particle, count),
		width:     width,
		height:    height,
	}
	for i := range ps.particles {
		ps.particles[i] = ps.spawnParticle()
	}
	return ps
}

func (ps *ParticleSystem) spawnParticle() particle {
	return particle{
		x:     rand.Float64() * float64(ps.width),
		y:     rand.Float64() * float64(ps.height),
		vx:    (rand.Float64() - 0.5) * 0.3,
		vy:    (rand.Float64() - 0.5) * 0.15,
		char:  particleChars[rand.IntN(len(particleChars))],
		alive: true,
	}
}

func (ps *ParticleSystem) Resize(width, height int) {
	ps.width = width
	ps.height = height
}

func (ps *ParticleSystem) Update(energy float64, primary, secondary string) {
	speedMul := 0.5 + energy*1.5

	for i := range ps.particles {
		p := &ps.particles[i]

		p.x += p.vx * speedMul
		p.y += p.vy * speedMul

		p.vx += (rand.Float64() - 0.5) * 0.02
		p.vy += (rand.Float64() - 0.5) * 0.01

		if p.vx > 0.5 {
			p.vx = 0.5
		} else if p.vx < -0.5 {
			p.vx = -0.5
		}
		if p.vy > 0.3 {
			p.vy = 0.3
		} else if p.vy < -0.3 {
			p.vy = -0.3
		}

		if p.x < 0 {
			p.x += float64(ps.width)
		} else if p.x >= float64(ps.width) {
			p.x -= float64(ps.width)
		}
		if p.y < 0 {
			p.y += float64(ps.height)
		} else if p.y >= float64(ps.height) {
			p.y -= float64(ps.height)
		}
	}
}

func (ps *ParticleSystem) Render() [][]rune {
	grid := make([][]rune, ps.height)
	for i := range grid {
		grid[i] = make([]rune, ps.width)
	}

	for _, p := range ps.particles {
		if !p.alive {
			continue
		}
		col := int(p.x)
		row := int(p.y)
		if row >= 0 && row < ps.height && col >= 0 && col < ps.width {
			grid[row][col] = p.char
		}
	}

	return grid
}
