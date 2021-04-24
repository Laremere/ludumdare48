package main

import (
	"fmt"
	"math"
	"math/rand"
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

	viewTop    float64
	viewBottom float64
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
	r.viewTop = s.viewy - r.halfHeight/r.spriteSize - 2
	r.viewBottom = s.viewy + r.halfHeight/r.spriteSize + 4

	r.ctx.Set("fillStyle", "#000000")
	r.ctx.Call("fillRect", 0, 0, r.width, r.height)

	r.draw("station", [2]float64{0, -6}, 10, 10)

	tileTop := int(r.viewTop / 2)
	if tileTop < 0 {
		tileTop = 0
	}
	tileBottom := int(r.viewBottom / 2)
	if tileBottom >= len(s.tiles[0]) {
		tileBottom = len(s.tiles[0]) - 1
	}

	for y := tileTop; y < tileBottom; y++ {
		if y < s.scaffoldings {
			r.draw("scaffolding", [2]float64{0, 2 * float64(y)}, 2, 2)
		}
	}

	for _, b := range s.bands {
		for itemName, i := range b.i {
			for _, pos := range i.p {
				r.draw(itemSprite[item(itemName)], pos, 0.2, 0.2)
			}
		}
	}

	// for _, rb := range s.rocks {
	// 	if r.onscreen(rb.topY, rb.topY+rb.height) {
	// 		for _, t := range rb.t {
	// 			r.draw("rock", t.p, 0.2, 0.2)
	// 		}
	// 	}
	// }

	for x := 0; x < len(s.tiles); x++ {
		for y := tileTop; y < tileBottom; y++ {
			if s.tiles[x][y] != empty {
				r.draw(tileSprite[s.tiles[x][y]], [2]float64{(2*float64(x) - spritesPerWidth/2 + 2), 2 * float64(y)}, 2, 2)
			}
		}
	}

	r.draw("ship", s.ship.p, 1, 1) // ALWAYS DRAW LAST

	{
		totalItems := 0
		for _, b := range s.bands {
			for _, i := range b.i {
				totalItems += len(i.p)
			}
		}
		r.ctx.Set("fillStyle", "#FFFFFF")
		r.ctx.Set("font", "30px Arial")
		r.ctx.Set("textAlign", "30px Arial")
		r.ctx.Call("fillText", fmt.Sprintf("Total items: %v", totalItems), 0, 30)
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

func (r *render) onscreen(min, max float64) bool {
	return !(r.viewBottom < min || r.viewTop > max)
}

type state struct {
	ship         transform
	viewx, viewy float64
	keyDown      map[key]bool
	key          map[key]bool
	keyUp        map[key]bool
	rocks        []*rockband
	tiles        [spritesPerWidth/2 - 1][worldHeight / 2]tile
	scaffoldings int
	bands        [numBands]*band
}

type tile int

const (
	empty = tile(iota)
	forge
	redirectorUp
	redirectorLeft
	redirectorRight
	redirectorDown
)

var tileSprite = map[tile]string{
	forge:           "forge",
	redirectorUp:    "redirectorUp",
	redirectorLeft:  "redirectorLeft",
	redirectorRight: "redirectorRight",
	redirectorDown:  "redirectordown",
}

func init() {
	s = &state{
		keyDown: make(map[key]bool),
		key:     make(map[key]bool),
		keyUp:   make(map[key]bool),
		rocks: []*rockband{
			&rockband{
				topY:   0,
				height: 20,
			},
			&rockband{
				topY:   25,
				height: 20,
			},
			&rockband{
				topY:   45,
				height: 20,
			},
		},
		scaffoldings: 2,
	}

	for i := 0; i < numBands; i++ {
		s.bands[i] = &band{}
	}

	for _, rb := range s.rocks {
		speed := math.Sqrt(1/float64(worldHeight)) * 10
		rb.step(spritesPerWidth / speed)
	}

	s.tiles[spritesPerWidth/4][1] = forge
	s.tiles[spritesPerWidth/4][3] = forge
	s.tiles[spritesPerWidth/4][5] = forge
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
			s.ship.v[0] *= math.Pow(0.1, dt)
			s.ship.v[1] *= math.Pow(0.1, dt)
		}
		if coasting && math.Abs(s.ship.v[0]) < 0.1 && math.Abs(s.ship.v[1]) < 0.1 {
			s.ship.v[0] = 0
			s.ship.v[1] = 0
		}

		s.ship.applyVelocity(dt)

		clampAndReset(&s.ship.p[0], -1*spritesPerWidth/2+0.5, spritesPerWidth/2-0.5, &s.ship.v[0])
		clampAndReset(&s.ship.p[1], -5, worldHeight, &s.ship.v[1])
	}

	for _, rb := range s.rocks {
		rb.step(dt)
	}

	for i, b := range s.bands {
		b.step(dt, i)
	}
	// TODO: move items to new bands

	s.viewy += (s.ship.p[1] - s.viewy) * dt
	clamp(&s.viewy, s.ship.p[1]-3, s.ship.p[1]+3)
}

func (s *state) pushItem(i popItem, t item) {
	bi := int(i.p[1] / bandHeight)
	s.bands[bi].i[t].push(i)
}

type rockband struct {
	nextSpawn float64
	// t         []transform
	// size      []float64
	topY   float64
	height float64
}

type item byte

const (
	rock = item(iota)
	metal
	numItems
)

var itemSprite = map[item]string{
	rock:  "rock",
	metal: "metal",
}

type band struct {
	i [numItems]items
}

type items struct {
	p [][2]float64
	v [][2]float64
}

func (b *band) step(dt float64, bandIndex int) {
	for i := range b.i {
		if item(i) == rock {
			for j := 0; j < len(b.i[i].p); j++ {
				if b.i[i].tileOf(j, bandIndex) == forge {
					b.i[metal].push(b.i[i].pop(j))
				}
			}
		}

		for j := 0; j < len(b.i[i].p); j++ {
			// fmt.Println(items.v[j][0] * dt)
			b.i[i].p[j][0] += b.i[i].v[j][0] * dt
			b.i[i].p[j][1] += b.i[i].v[j][1] * dt

			if b.i[i].p[j][0] < -1*spritesPerWidth/2-2 {
				b.i[i].pop(j)
				j--
				// fmt.Println("leftoff")
			} else if b.i[i].p[j][0] > spritesPerWidth/2+2 {
				b.i[i].pop(j)
				j--
				// fmt.Println("rightoff")
			}
		}
	}
}

func (is *items) tileOf(i int, bandIndex int) tile {
	xTile := int((is.p[i][0] - 1 + spritesPerWidth/2) / 2)
	yTile := int((is.p[i][1] + 1) / 2)
	if xTile < 0 || yTile < 0 || xTile >= len(s.tiles) || yTile >= len(s.tiles[0]) {
		return empty
	}
	return s.tiles[xTile][yTile]
}

type popItem struct {
	p [2]float64
	v [2]float64
}

func (is *items) pop(i int) popItem {
	pi := popItem{
		p: is.p[i],
		v: is.v[i],
	}

	last := len(is.p) - 1
	is.p[i] = is.p[last]
	is.p = is.p[:last]
	is.v[i] = is.v[last]
	is.v = is.v[:last]

	return pi
}

func (is *items) push(pi popItem) {
	is.p = append(is.p, pi.p)
	is.v = append(is.v, pi.v)
}

const bandHeight = 20
const numBands = worldHeight / bandHeight

func init() {
	if worldHeight%bandHeight != 0 {
		panic("NO")
	}
}

func (rb *rockband) step(dt float64) {
	// for i := 0; i < len(rb.t); i++ {
	// 	rb.t[i].applyVelocity(dt)
	// 	if rb.t[i].p[0] > spritesPerWidth/2+0.5 {
	// 		last := len(rb.t) - 1
	// 		rb.t[i] = rb.t[last]
	// 		rb.t = rb.t[:last]
	// 	}
	// }

	rb.nextSpawn += dt
	// fmt.Println(len(rb.t))
	for ; rb.nextSpawn > 0; rb.nextSpawn -= 0.5 {
		pi := popItem{
			p: [2]float64{
				-1*spritesPerWidth/2 - 0.5,
				rb.height*rand.Float64() + rb.topY,
			},
			// v: [2]float64{
			// 	math.Sqrt(1/(worldHeight-rb.t[i].p[1])) * 10,
			// 	0,
			// },
		}
		pi.v[0] = math.Sqrt(1/(worldHeight-pi.p[1])) * 10
		pi.p[0] += pi.v[0] * rb.nextSpawn
		s.pushItem(pi, rock)

		// i := len(rb.t)
		// rb.t = append(rb.t, transform{})
		// rb.t[i].p[0] = -1*spritesPerWidth/2 - 0.5
		// rb.t[i].p[1] = rb.height*rand.Float64() + rb.topY
		// rb.t[i].v[0] = math.Sqrt(1/(worldHeight-rb.t[i].p[1])) * 10
		// rb.t[i].applyVelocity(rb.nextSpawn)
	}
}

const worldHeight = 100

// const worldHeight = 1000

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
