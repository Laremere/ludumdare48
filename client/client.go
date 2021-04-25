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
		if !s.key[k] {
			s.keyDown[k] = true
		}
		s.key[k] = true

		return nil
	}))

	js.Global().Get("document").Call("addEventListener", "keyup", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		k := key(e.Get("keyCode").Int())
		if s.key[k] {
			s.keyUp[k] = true
		}
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
		if dt > 1.0/20 {
			dt = 1.0 / 20
		}
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

	r.draw("station", vec{spritesPerWidth / 2, -3}, 5, 5)

	tileTop := int(r.viewTop)
	if tileTop < 0 {
		tileTop = 0
	}
	tileBottom := int(r.viewBottom)
	if tileBottom >= len(s.tiles[0]) {
		tileBottom = len(s.tiles[0]) - 1
	}

	r.draw("scaffolding", vec{spritesPerWidth / 2, 0}, 1, 1)
	for y := tileTop; y < tileBottom; y++ {
		if y < s.scaffoldings {
			r.draw("scaffolding", vec{spritesPerWidth / 2, 1 + float64(y)}, 1, 1)
		}
	}

	r.draw("foot", vec{spritesPerWidth / 2, s.foot + 1.5}, 2, 2)

	for x := 0; x < len(s.tiles); x++ {
		for y := tileTop; y < tileBottom; y++ {
			if s.tiles[x][y] != empty {
				r.draw(tileSprite[s.tiles[x][y]], vec{(float64(x) + 0.5), float64(y) + 0.5}, 1, 1)
			}
		}
	}

	////////////////////////////move items to below tiles
	for _, b := range s.bands {
		for itemName, i := range b.i {
			for _, pos := range i.p {
				r.draw(itemSprite[item(itemName)], pos, 0.1, 0.1)
			}
		}
	}
	for i := range s.collecting {
		for j := range s.collecting[i] {
			r.draw(itemSprite[item(i)], s.collecting[i][j].p, 0.1, 0.1)
		}
	}
	for i := range s.sending {
		r.draw(itemSprite[s.sending[i].i], s.sending[i].p, 0.1, 0.1)
	}
	////////////////////////////move items to below tiles

	r.draw("ship", s.ship.p, shipSize, shipSize) // ALWAYS DRAW LAST, except for UI

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
		r.ctx.Call("fillText", fmt.Sprintf("X: %0.2f, Y: %0.2f", s.ship.p[0], s.ship.p[1]), 0, 60)
		y := 95
		for i := item(0); i < numItems; i++ {
			r.ctx.Call("fillText", fmt.Sprintf("%d", s.inventory[i]), 30, y)
			r.ctx.Call("drawImage", image(itemSprite[i]), 0, y-30, 30, 30)
			y += 30
		}
		selected := "remove"
		if s.buildSelector != empty {
			selected = tileSprite[s.buildSelector]
		}

		r.ctx.Call("drawImage", image(selected), 0, y, 50, 50)
	}
}

// const spritesPerWidth = 16
const spritesPerWidth = 9

var cachedImages = map[string]js.Value{}

func image(id string) js.Value {
	v, ok := cachedImages[id]
	if !ok {
		v = js.Global().Get("Image").New()
		v.Set("src", "sprites/"+id+".svg")
	}
	return v
}

func (r *render) draw(id string, p vec, sx, sy float64) {
	v := image(id)

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
	ship           transform
	inventory      [numItems]int
	collecting     [numItems][]transform
	sending        []sending
	viewx, viewy   float64
	dropCooldown   float64
	keyDown        map[key]bool
	key            map[key]bool
	keyUp          map[key]bool
	rocks          []*rockband
	tiles          [spritesPerWidth - 1][worldHeight / 2]tile
	bands          [numBands]*band
	scaffoldings   int
	foot           float64
	footInv        [numItems]int
	shipInFootZone bool

	buildSelector   tile
	scaffoldingCost [numItems]int
}

type tile int

const (
	empty = tile(iota)
	forge
	redirectorUp
	redirectorLeft
	redirectorRight
	redirectorDown
	numTiles
)

var tileSprite = map[tile]string{
	forge:           "forge",
	redirectorUp:    "redirectorUp",
	redirectorLeft:  "redirectorLeft",
	redirectorRight: "redirectorRight",
	redirectorDown:  "redirectordown",
}

type sending struct {
	dst transform
	p   vec
	i   item
}

func init() {
	s = &state{
		ship: transform{
			p: vec{spritesPerWidth / 2.0, 0.5},
		},
		viewx:   spritesPerWidth/2.0 - 0.5,
		keyDown: make(map[key]bool),
		key:     make(map[key]bool),
		keyUp:   make(map[key]bool),
		rocks: []*rockband{
			&rockband{
				topY:   0,
				height: 10,
			},
			&rockband{
				topY:   11,
				height: 10,
			},
			&rockband{
				topY:   21,
				height: 10,
			},
		},
		// scaffoldings: 1,
		// foot:         1,
	}

	for i := 0; i < numBands; i++ {
		s.bands[i] = &band{}
	}

	for _, rb := range s.rocks {
		speed := math.Sqrt(1/float64(worldHeight)) * 5
		rb.step(spritesPerWidth / speed)
	}
	// for x := 0; x < spritesPerWidth-1; x++ {
	// 	s.tiles[x][0] = forge
	// }

	// s.tiles[spritesPerWidth/2][1] = forge
	// s.tiles[spritesPerWidth/2][3] = forge
	// s.tiles[spritesPerWidth/2][5] = forge
	// s.tiles[spritesPerWidth/2-2][1] = redirectorRight
	// s.tiles[spritesPerWidth/2-2][2] = redirectorUp
	// s.tiles[spritesPerWidth/2-2][4] = redirectorUp
}

func (s *state) step(dt float64) {
	{ /// Update ship position

		coasting := true
		const accel = 25
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

		clamp(&s.ship.v[1], -5, 5)
		if coasting && math.Abs(s.ship.v[0]) < 2.5 && math.Abs(s.ship.v[1]) < 2.5 {
			s.ship.v[0] *= math.Pow(0.1, dt)
			s.ship.v[1] *= math.Pow(0.1, dt)
		}
		if coasting && math.Abs(s.ship.v[0]) < 0.1 && math.Abs(s.ship.v[1]) < 0.1 {
			s.ship.v[0] = 0
			s.ship.v[1] = 0
		}

		s.ship.applyVelocity(dt)

		clampAndReset(&s.ship.p[0], -0.5+shipSize/2, spritesPerWidth-0.5-shipSize/2, &s.ship.v[0])
		clampAndReset(&s.ship.p[1], -5, worldHeight, &s.ship.v[1])
		clampAndReset(&s.ship.p[1], -5, s.foot+3, &s.ship.v[1])
	}

	if s.keyDown[keyZ] {
		if s.tileAt(s.ship.p) == empty {
			xTile := int(s.ship.p[0])
			yTile := int(s.ship.p[1])
			if !(xTile < 0 || yTile < 0 || xTile >= len(s.tiles) || yTile >= len(s.tiles[0]) || yTile >= s.scaffoldings) {
				s.tiles[xTile][yTile] = s.buildSelector
			}
		}
	}
	if s.keyDown[keyX] {
		s.buildSelector--
		for s.buildSelector < 0 {
			s.buildSelector += numTiles
		}
	}
	if s.keyDown[keyC] {
		s.buildSelector++
		for s.buildSelector >= numTiles {
			s.buildSelector -= numTiles
		}
	}

	// Used in bands to prevent re-pickup
	s.shipInFootZone = s.ship.p[0] > 3.25 && s.ship.p[0] < 4.75 && s.ship.p[1] < s.foot && s.ship.p[1] > s.foot-2
	fmt.Println(s.shipInFootZone)
	{
		if s.dropCooldown > 0 {
			s.dropCooldown -= dt
		}
		if s.dropCooldown <= 0 && s.tileAt(s.ship.p.add(vec{1, 0})) == forge && s.inventory[rock] > 0 {
			s.sending = append(s.sending, sending{
				dst: transform{
					p: s.ship.p.floor().add(vec{1.1, 0.45 + rand.Float64()*0.1}),
					v: vec{0.5, 0},
				},
				p: s.ship.p,
				i: rock,
			})
			s.inventory[rock]--
			s.dropCooldown += 0.5
		}

		if s.dropCooldown <= 0 && s.shipInFootZone {
			for i := item(0); i < numItems; i++ {
				if s.inventory[i] > 0 && s.scaffoldingCost[i] > s.footInv[i] {
					s.sending = append(s.sending, sending{
						dst: transform{
							p: vec{3.5 + rand.Float64(), s.foot + 0.5},
							v: vec{0, 1},
						},
						p: s.ship.p,
						i: i,
					})
					s.inventory[i]--
					s.dropCooldown += 0.5
					break
				}
			}
		}

	}

	for i := 0; i < len(s.sending); i++ {
		s.sending[i].p = s.sending[i].p.tween(s.sending[i].dst.p, dt)
		if s.sending[i].p[0] == s.sending[i].dst.p[0] && s.sending[i].p[1] == s.sending[i].dst.p[1] {
			s.pushItem(s.sending[i].dst, s.sending[i].i)
			last := len(s.sending) - 1
			s.sending[i] = s.sending[last]
			s.sending = s.sending[:last]
			i--
		}
	}

	for _, rb := range s.rocks {
		rb.step(dt)
	}

	for i, b := range s.bands {
		b.step(dt, i)
	}
	// TODO: move items to new bands

	for i := range s.collecting {
		for j := 0; j < len(s.collecting[i]); j++ {
			if s.collecting[i][j].p.sub(s.ship.p).abs() < 0.25 {
				s.inventory[i]++
				last := len(s.collecting[i]) - 1
				s.collecting[i][j] = s.collecting[i][last]
				s.collecting[i] = s.collecting[i][:last]
				j--
				continue
			}

			s.collecting[i][j].v = s.collecting[i][j].v.tween(s.ship.p.sub(s.collecting[i][j].p).norm().scale(10), 10*dt) // .scale(math.Pow(0.9, dt))
			s.collecting[i][j].applyVelocity(dt)
		}
	}

	{
		if s.foot < float64(s.scaffoldings) {
			s.foot += dt / 2
			if s.foot > float64(s.scaffoldings) {
				s.foot = float64(s.scaffoldings)
			}
		} else {
			s.scaffoldingCost = [numItems]int{}
			if s.scaffoldings > 3 && s.scaffoldings <= 10 {
				s.scaffoldingCost[rock] += 5
			}
			if s.scaffoldings > 10 {
				s.scaffoldingCost[metal] += s.scaffoldings
			}
			canbuildnext := true
			for i := range s.scaffoldingCost {
				if s.scaffoldingCost[i] > s.footInv[i] {
					canbuildnext = false
				}
			}
			if canbuildnext {
				for i := range s.scaffoldingCost {
					s.footInv[i] -= s.scaffoldingCost[i]
				}
				s.scaffoldings++
			}
		}
	}

	s.viewy += (s.ship.p[1] - s.viewy) * dt
	clamp(&s.viewy, s.ship.p[1]-1, s.ship.p[1]+1)
}

const shipSize = 0.5

func (s *state) pushItem(i transform, t item) {
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
	p []vec
	v []vec
}

func (b *band) step(dt float64, bandIndex int) {
	for i := range b.i {
		// if item(i) == rock {
		// 	for j := 0; j < len(b.i[i].p); j++ {
		// 		if s.tileAt(b.i[i].p[j]) == forge {
		// 			tilepos := b.i[i].p[j].floor()

		// 		}
		// 		// } else if s.tileAt(b.i[i].p[j].add(vec{2, 0})) == forge {
		// 		// 	b.i[i].v[j] = vec{2, 0}
		// 		// }
		// 	}
		// }

		shipCollectionMin := s.ship.p.sub(vec{1, 1})
		shipCollectionMax := s.ship.p.add(vec{1, 1})

	jLoop:
		for j := 0; j < len(b.i[i].p); j++ {
			tilePos := b.i[i].p[j].floor()
			refTile := b.i[i].p[j].sub(tilePos)

			switch s.tileAt(b.i[i].p[j]) {
			case redirectorDown, redirectorUp, redirectorRight, redirectorLeft:
				var dir vec
				switch s.tileAt(b.i[i].p[j]) {
				case redirectorUp:
					dir = vec{0, -1}
				case redirectorDown:
					dir = vec{0, 1}
				case redirectorRight:
					dir = vec{1, 0}
				case redirectorLeft:
					dir = vec{-1, 0}
				}
				if b.i[i].v[j][0] != dir[0] || b.i[i].v[j][1] != dir[1] {
					if refTile.within(vec{0.4, 0.4}, vec{0.6, 0.6}) {
						b.i[i].v[j] = dir
						b.i[i].p[j] = tilePos.add(vec{rand.Float64()*0.2 + 0.4, rand.Float64()*0.2 + 0.4})
					} else {
						b.i[i].v[j] = b.i[i].v[j].tween(tilePos.add(vec{0.5, 0.5}).sub(b.i[i].p[j]).norm(), 0.5*dt)
					}
				}
			case forge:
				if !refTile.within(vec{-1, 0.35}, vec{2, 0.65}) {
					b.i[i].pop(j)
					j--
					continue jLoop
				}
				if item(i) == rock && refTile.within(vec{0.35, 0.35}, vec{0.65, 0.65}) {
					b.i[metal].push(b.i[i].pop(j))
					j--
					continue jLoop
				}
			case empty:
				if b.i[i].p[j].within(shipCollectionMin, shipCollectionMax) && (s.inventory[i]+len(s.collecting[i])) < 100 && !s.shipInFootZone {
					s.collecting[i] = append(s.collecting[i], b.i[i].pop(j))
					j--
					continue jLoop
				}
			}

			if b.i[i].p[j][1] > s.foot+0.5 && b.i[i].p[j][1] < s.foot+2.5 {
				if b.i[i].p[j][0] > 3 && b.i[i].p[j][0] < 5 {
					if b.i[i].p[j][0] > 3.25 && b.i[i].p[j][0] < 4.75 && b.i[i].p[j][1] < s.foot+1 && b.i[i].v[j][1] > 0 {
						s.footInv[i]++
					}
					b.i[i].pop(j)
					j--
					continue jLoop
				}
			}

			b.i[i].p[j][0] += b.i[i].v[j][0] * dt
			b.i[i].p[j][1] += b.i[i].v[j][1] * dt

			if b.i[i].p[j][0] < -1.5 || b.i[i].p[j][0] > spritesPerWidth+0.5 || b.i[i].p[j][1] < -30 || b.i[i].p[j][1] > worldHeight+30 {
				b.i[i].pop(j)
				j--
			}
		}
	}
}

func (s *state) tileAt(v vec) tile {
	// vTile := v.TilePos()
	xTile := int(v[0])
	yTile := int(v[1])
	if xTile < 0 || yTile < 0 || xTile >= len(s.tiles) || yTile >= len(s.tiles[0]) {
		return empty
	}
	return s.tiles[xTile][yTile]
}

func (is *items) pop(i int) transform {
	pi := transform{
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

func (is *items) push(pi transform) {
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
	for ; rb.nextSpawn > 0; rb.nextSpawn -= 0.25 {
		pi := transform{
			p: vec{
				-1,
				rb.height*rand.Float64() + rb.topY,
			},
			// v: vec{
			// 	math.Sqrt(1/(worldHeight-rb.t[i].p[1])) * 10,
			// 	0,
			// },
		}
		pi.v[0] = math.Sqrt(1/(worldHeight-pi.p[1])) * 5
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
	p vec
	v vec
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

type vec [2]float64

func (v vec) add(o vec) vec {
	return vec{
		v[0] + o[0],
		v[1] + o[1],
	}
}

func (v vec) sub(o vec) vec {
	return vec{
		v[0] - o[0],
		v[1] - o[1],
	}
}

func (v vec) scale(m float64) vec {
	return vec{v[0] * m, v[1] * m}
}

func (v vec) norm() vec {
	d := v.abs()
	return vec{v[0] / d, v[1] / d}
}

func (v vec) abs() float64 {
	return math.Sqrt(v[0]*v[0] + v[1]*v[1])
}

func (v vec) tween(dst vec, dt float64) vec {
	dir := dst.sub(v)
	dirlen := dir.abs()
	if dirlen < dt {
		return dst
	}
	return v.add(dir.scale(dt / dirlen))
}

func (v vec) floor() vec {
	return vec{math.Floor(v[0]), math.Floor(v[1])}
}

func (v vec) within(min, max vec) bool {
	return v[0] >= min[0] && v[0] <= max[0] && v[1] >= min[1] && v[1] <= max[1]
}
