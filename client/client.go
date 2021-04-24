package main

import (
	"fmt"
	"math"
	"syscall/js"
)

type key int

const (
	keyZ     = key(90)
	keyX     = key(88)
	keyC     = key(67)
	keyUp    = key(38)
	keyDown  = key(40)
	keyLeft  = key(37)
	keyRight = key(39)
)

func main() {
	fmt.Println("Starting game")
	s = &state{
		keyDown: make(map[key]bool),
		key:     make(map[key]bool),
		keyUp:   make(map[key]bool),
	}
	r := NewRender()
	r.render()

	c := client{
		r: r,
		s: s,
	}
	c.animationFrameJs = js.FuncOf(c.animationFrame)
	js.Global().Get("window").Call("requestAnimationFrame", c.animationFrameJs)

	js.Global().Get("document").Call("addEventListener", "keydown", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		k := key(e.Get("keyCode").Int())
		s.keyDown[k] = true
		s.key[k] = true

		return nil
	}))

	js.Global().Get("document").Call("addEventListener", "keyup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		k := key(e.Get("keyCode").Int())
		s.keyUp[k] = true
		s.key[k] = false

		return nil
	}))

	waitforever := make(chan struct{})
	<-waitforever
}

var s *state

type client struct {
	r                *render
	s                *state
	animationFrameJs js.Func
	lasttimestamp    float64
}

func (c *client) animationFrame(this js.Value, args []js.Value) interface{} {
	timestamp := args[0].Float()
	if c.lasttimestamp != 0 {
		dt := (timestamp - c.lasttimestamp) / 1000
		s.step(dt)

		for k := range s.keyDown {
			s.keyDown[k] = false
		}
		for k := range s.keyUp {
			s.keyDown[k] = false
		}
	}
	c.lasttimestamp = timestamp
	c.r.render()

	js.Global().Get("window").Call("requestAnimationFrame", c.animationFrameJs)
	return nil
}

type render struct {
	container  js.Value
	canvas     js.Value
	ctx        js.Value
	width      int
	height     int
	spriteSize float64
	halfWidth  float64
	halfHeight float64
}

func NewRender() *render {
	r := &render{
		// Invalidate to be set by fixed render call
		width:  -1,
		height: -1,
	}
	r.container = js.Global().Get("document").Call("getElementsByTagName", "body").Index(0)
	r.canvas = js.Global().Get("document").Call("getElementById", "game")
	r.ctx = r.canvas.Call("getContext", "2d")

	return r
}

func (r *render) render() {
	{
		height := r.container.Get("clientHeight").Int()
		width := r.container.Get("clientWidth").Int()

		if height != r.height || width != r.width {
			r.height = height
			r.width = width
			r.canvas.Set("height", height)
			r.canvas.Set("width", width)
			mindim := width
			// if height < mindim {
			// 	mindim = height
			// }
			r.spriteSize = float64(mindim) / spritesPerWidth
			r.halfWidth = float64(width) / 2
			r.halfHeight = float64(height) / 2
		}
	}

	r.ctx.Set("fillStyle", "#000000")
	r.ctx.Call("fillRect", 0, 0, r.width, r.height)

	r.draw("ship", s.ship.p, 1, 1)

	for i := -100; i < 100; i++ {
		r.draw("rock", [2]float64{0, float64(i) * 4}, 0.2, 0.2)
	}

}

const spritesPerWidth = 16

var cachedImages = map[string]js.Value{}

func (r *render) draw(id string, p [2]float64, sx, sy float64) {
	v, ok := cachedImages[id]
	if !ok {
		v = js.Global().Get("Image").New()
		v.Set("src", "sprites/"+id+".svg")
	}

	w := sx * r.spriteSize
	h := sy * r.spriteSize

	xx := (p[0]-s.viewx)*r.spriteSize - w/2 + r.halfWidth
	yy := (p[1]-s.viewy)*r.spriteSize - h/2 + r.halfHeight

	r.ctx.Call("drawImage", v, xx, yy, w, h)
}

type state struct {
	ship         transform
	viewx, viewy float64
	keyDown      map[key]bool
	key          map[key]bool
	keyUp        map[key]bool
}

func (s *state) step(dt float64) {
	{ /// Update ship position

		coasting := true
		const accel = 50
		if s.key[keyLeft] {
			s.ship.v[0] -= accel * dt
			coasting = false
		}
		if s.key[keyRight] {
			s.ship.v[0] += accel * dt
			coasting = false
		}
		if s.key[keyUp] {
			s.ship.v[1] -= accel * dt
			coasting = false
		}
		if s.key[keyDown] {
			s.ship.v[1] += accel * dt
			coasting = false
		}

		clamp(&s.ship.v[1], -10, 10)
		if coasting && math.Abs(s.ship.v[0]) < 2.5 && math.Abs(s.ship.v[1]) < 2.5 {
			fmt.Println(math.Pow(0.5, dt))
			s.ship.v[0] *= math.Pow(0.1, dt)
			s.ship.v[1] *= math.Pow(0.1, dt)
		}
		if coasting && math.Abs(s.ship.v[0]) < 0.1 && math.Abs(s.ship.v[1]) < 0.1 {
			s.ship.v[0] = 0
			s.ship.v[1] = 0
		}

		s.ship.applyVelocity(dt)

		clampAndReset(&s.ship.p[0], -1*spritesPerWidth/2+0.5, spritesPerWidth/2-0.5, &s.ship.v[0])
		clampAndReset(&s.ship.p[1], -100, 5, &s.ship.v[1])
	}

	s.viewy += (s.ship.p[1] - s.viewy) * dt
	clamp(&s.viewy, s.ship.p[1]-3, s.ship.p[1]+3)
}

type transform struct {
	p [2]float64
	v [2]float64
}

func (t *transform) applyVelocity(dt float64) {
	t.p[0] += t.v[0] * dt
	t.p[1] += t.v[1] * dt
}

func clamp(value *float64, min, max float64) {
	if *value < min {
		*value = min
	} else if *value > max {
		*value = max
	}
}

func clampAndReset(value *float64, min, max float64, reset *float64) {
	if *value < min {
		*value = min
		*reset = 0
	} else if *value > max {
		*value = max
		*reset = 0
	}
}
