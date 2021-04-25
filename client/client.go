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
	stepStart := js.Global().Get("window").Get("performance").Call("now").Int()
	stepEnd := stepStart
	timestamp := args[0].Float()
	if c.lasttimestamp != 0 {
		dt := (timestamp - c.lasttimestamp) / 1000
		if dt > 1.0/20 {
			dt = 1.0 / 20
		}
		s.step(dt)
		stepEnd = js.Global().Get("window").Get("performance").Call("now").Int()

		for k := range s.keyDown {
			s.keyDown[k] = false
		}
		for k := range s.keyUp {
			s.keyDown[k] = false
		}
	}
	c.lasttimestamp = timestamp
	c.r.render()
	renderEnd := js.Global().Get("window").Get("performance").Call("now").Int()

	c.r.ctx.Call("fillText", fmt.Sprintf("Step time:   %d", stepEnd-stepStart), 300, 60)
	c.r.ctx.Call("fillText", fmt.Sprintf("Render time: %d", renderEnd-stepEnd), 300, 90)

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

	stars [][2]float64
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

	r.ctx.Set("fillStyle", "#111111")
	r.ctx.Call("fillRect", 0, 0, r.width, r.height)

	r.draw("background", vec{spritesPerWidth / 2, 150 / 2}, spritesPerWidth, 150)

	r.draw("station", vec{spritesPerWidth / 2, -3}, 5, 5)

	tileTop := int(math.Floor(r.viewTop))
	if tileTop < 0 {
		tileTop = 0
	}
	tileBottom := int(math.Ceil(r.viewBottom))
	if tileBottom >= len(s.tiles[0]) {
		tileBottom = len(s.tiles[0]) - 1
	}

	r.draw("scaffolding", vec{spritesPerWidth / 2, 0}, 1, 1)
	for y := tileTop; y < tileBottom; y++ {
		if y < s.scaffoldings {
			r.draw("scaffolding", vec{spritesPerWidth / 2, 1 + float64(y)}, 1, 1)
		}
	}

	////////////////////////////move items to below tiles

	{
		minBand := int(math.Floor(r.viewTop / bandHeight))
		if minBand < 0 {
			minBand = 0
		}
		maxBand := int(math.Floor(r.viewBottom/bandHeight)) + 1
		if maxBand >= len(s.bands) {
			maxBand = len(s.bands) - 1
		}
		for _, b := range s.bands[minBand:maxBand] {
			for itemName, i := range b.i {
				for _, pos := range i.p {
					r.draw(itemSprite[item(itemName)], pos, 0.1, 0.1)
				}
			}
		}
	}
	for i := range s.collecting {
		for j := range s.collecting[i] {
			r.draw(itemSprite[item(i)], s.collecting[i][j].p, 0.1, 0.1)
		}
	}
	////////////////////////////move items to below tiles
	r.draw("foot", vec{spritesPerWidth / 2, s.foot + 1.25}, 2, 2)

	for x := 0; x < len(s.tiles); x++ {
		for y := tileTop; y < tileBottom; y++ {
			if s.tiles[x][y] != empty {
				r.draw(tileSprite[s.tiles[x][y]], vec{(float64(x) + 0.5), float64(y) + 0.5}, 1, 1)
			}
		}
	}

	for _, f := range s.faders {
		r.draw(itemSprite[f.i], f.t.p, f.s, f.s)
	}

	for i := range s.sending {
		r.draw(itemSprite[s.sending[i].i], s.sending[i].p, 0.1, 0.1)
	}

	r.draw("ship", s.ship.p, shipSize, shipSize) // ALWAYS DRAW LAST, except for UI

	{
		{
			guiHeight := r.height / 4
			guiWidth := guiHeight * 10
			r.ctx.Call("drawImage", image("gui"), r.width/2-guiWidth/2, r.height-guiHeight, guiWidth, r.height/4)
		}

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
			if !i.canHold() {
				continue
			}
			r.ctx.Call("fillText", fmt.Sprintf("%d - %d (%d / %d)", s.inventory[i], tileCost[s.buildSelector][i], s.footInv[i], s.scaffoldingCost[i]), 30, y)
			r.ctx.Call("drawImage", image(itemSprite[i]), 0, y-30, 30, 30)
			y += 30
		}
		selected := "remove"
		if s.buildSelector != empty {
			selected = tileSprite[s.buildSelector]
		}
		r.ctx.Call("fillText", fmt.Sprintf("Power %0.1f", s.powerLevel), 30, y)
		y += 30

		r.ctx.Call("drawImage", image(selected), 0, y, 100, 100)
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
	ship               transform
	destination        vec
	hasDestination     bool
	inventory          [numItems]int
	collecting         [numItems][]transform
	sending            []sending
	viewx, viewy       float64
	dropCooldown       float64
	keyDown            map[key]bool
	key                map[key]bool
	keyUp              map[key]bool
	rocks              []*rockband
	tiles              [spritesPerWidth - 1][worldHeight]tile
	tileItems          [spritesPerWidth - 1][worldHeight][numItems]int16
	tileCooldown       [spritesPerWidth - 1][worldHeight]float64
	bands              [numBands]*band
	scaffoldings       int
	foot               float64
	footSpeed          float64
	footInv            [numItems]int
	shipInFootZone     bool
	faders             []fader
	heliumRainCooldown float64
	powerLevel         float64

	buildSelector   tile
	scaffoldingCost [numItems]int
}

type tile int

const (
	empty = tile(iota)
	extractor
	redirectorUp
	redirectorLeft
	redirectorDown
	redirectorRight
	weaver
	fan
	splitter
	laser
	core
	filter
	boiler
	turbine
	fabricator
	numTiles
	// forge
)

var maxItems = map[tile][numItems]int16{
	extractor: {
		ice: 15,
	},
	weaver: {
		carbon: 10,
	},
	fabricator: {
		helium:  100,
		silicon: 10,
	},
	laser: {
		hydrogen: 10,
	},
	core: {
		plasma: 1000,
	},
	boiler: {
		water: 100,
		//plasma: 1, // boiler needs plasma temp logic.
	},
}

// var inputSide

var tileSprite = map[tile]string{
	extractor:  "extractor",
	filter:     "waterextractor",
	weaver:     "weaver",
	fabricator: "fabricator",
	fan:        "fan",
	laser:      "laser",
	core:       "core",
	boiler:     "boiler",
	turbine:    "turbine",

	// forge:           "forge",
	redirectorUp:    "redirectorUp",
	redirectorLeft:  "redirectorLeft",
	redirectorDown:  "redirectordown",
	redirectorRight: "redirectorRight",
	splitter:        "splitter",
}

var tileCost = map[tile]map[item]int{
	// forge: {
	// 	rock: 40,
	// },
	extractor: {
		ice: 20,
	},
	redirectorUp: {
		silicon: 1,
		carbon:  1,
		// metal: 25,
	},
	redirectorLeft: {
		silicon: 1,
		carbon:  1,
		// metal: 25,
	},
	redirectorDown: {
		silicon: 1,
		carbon:  1,
		// metal: 25,
	},
	redirectorRight: {
		silicon: 1,
		carbon:  1,
		// metal: 25,
	},
	splitter: {
		hydrogen: 1,
		silicon:  1,
		carbon:   1,
	},
	filter: {
		hydrogen: 3,
		silicon:  5,
		carbon:   5,
	},
	weaver: {
		silicon: 7,
		carbon:  6,
	},
	fabricator: {
		nanotubes: 10,
		helium:    5,
	},
	fan: {
		silicon: 4,
		carbon:  4,
	},
	laser: {
		hydrogen:  20,
		carbon:    10,
		nanotubes: 1,
	},
	core: {
		silicon:   100,
		hydrogen:  10,
		nanotubes: 1,
	},
	boiler: {
		hydrogen:  2,
		carbon:    10,
		nanotubes: 1,
	},
	turbine: {
		hydrogen:  2,
		carbon:    10,
		nanotubes: 1,
	},
}

type sending struct {
	dst       transform
	p         vec
	i         item
	dontspawn bool
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
		footSpeed: 0.5,
		// scaffoldings: 1,
		// foot:         1,
	}

	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////// cheat, remove
	for i := item(0); i < numItems; i++ {
		s.inventory[i] = 9999999
	}
	// s.scaffoldings = worldHeight
	// s.footSpeed = 6
	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////// cheat, remove

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
	s.powerLevel += dt
	if s.powerLevel > 100 {
		s.powerLevel = 100
	}
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
		if !coasting {
			s.hasDestination = false
		}
		if coasting && math.Abs(s.ship.v[0]) < 4 && math.Abs(s.ship.v[1]) < 4 {
			if !s.hasDestination {
				if math.Abs(s.ship.v[0]) > math.Abs(s.ship.v[1]) {
					if s.ship.v[0] < 0 {
						s.destination = s.ship.p.floor().add(vec{-0.5, 0.5})
					} else {
						s.destination = s.ship.p.floor().add(vec{1.5, 0.5})
					}
				} else {
					if s.ship.v[1] < 0 {
						s.destination = s.ship.p.floor().add(vec{0.5, -0.5})
					} else {
						s.destination = s.ship.p.floor().add(vec{0.5, 1.5})
					}
				}

				s.ship.v[0] = 0
				s.ship.v[1] = 0
				s.hasDestination = true

			}
			s.ship.p = s.ship.p.tween(s.destination, dt*3)
		}

		s.ship.applyVelocity(dt)

		clampAndReset(&s.ship.p[0], -0.5+shipSize/2, spritesPerWidth-0.5-shipSize/2, &s.ship.v[0])
		clampAndReset(&s.ship.p[1], -5, 150-shipSize/2, &s.ship.v[1])
		clampAndReset(&s.ship.p[1], -5, s.foot+3, &s.ship.v[1])
	}

	if s.keyDown[keyZ] {
		xTile, yTile := s.ship.p.tilePos()
		if !(xTile < 0 || yTile < 0 || xTile >= len(s.tiles) || yTile >= len(s.tiles[0]) || yTile >= s.scaffoldings) {
			if s.tileAt(s.ship.p) == empty {
				canAfford := true
				for i, cost := range tileCost[s.buildSelector] {
					fmt.Println(i, s.inventory[i], cost)
					if s.inventory[i] < cost {
						canAfford = false
					}
				}
				if canAfford {
					s.tiles[xTile][yTile] = s.buildSelector
					for i, cost := range tileCost[s.buildSelector] {
						fmt.Println(i, s.inventory[i], cost)
						s.inventory[i] -= cost
						for j := 0; j < cost; j++ {

							s.sending = append(s.sending, sending{
								dst: transform{
									p: s.ship.p.floor().add(vec{rand.Float64(), rand.Float64()}),
								},
								p:         s.ship.p,
								i:         i,
								dontspawn: true,
							})
						}
					}
				}
			} else if s.buildSelector == empty {
				for i, cost := range tileCost[s.tiles[xTile][yTile]] {
					for j := 0; j < cost; j++ {
						s.pushItem(transform{
							p: s.ship.p.floor().add(vec{0.1 + rand.Float64()*0.8, 0.1 + rand.Float64()*0.8}),
							v: vec{rand.Float64() - 0.5, rand.Float64() - 0.5}.norm().scale(0.1),
						}, i)
					}
				}
				s.tiles[xTile][yTile] = empty
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
	s.shipInFootZone = s.ship.p[0] > 3.25 && s.ship.p[0] < 4.75 && s.ship.p[1] < s.foot+1 && s.ship.p[1] > s.foot-2
	{
		if s.dropCooldown > 0 {
			s.dropCooldown -= dt
		}
		if s.dropCooldown <= 0 && s.tileAt(s.ship.p.add(vec{1, 0})) == extractor && s.inventory[ice] > 0 {
			s.sending = append(s.sending, sending{
				dst: transform{
					p: s.ship.p.floor().add(vec{1.1, 0.45 + rand.Float64()*0.1}),
					v: vec{0.5, 0},
				},
				p: s.ship.p,
				i: ice,
			})
			s.inventory[ice]--
			s.dropCooldown += 0.1
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
					s.dropCooldown += 0.1
					break
				}
			}
		}

	}

	for i := 0; i < len(s.sending); i++ {
		s.sending[i].p = s.sending[i].p.tween(s.sending[i].dst.p, dt)
		if s.sending[i].p[0] == s.sending[i].dst.p[0] && s.sending[i].p[1] == s.sending[i].dst.p[1] {
			if !s.sending[i].dontspawn {
				s.pushItem(s.sending[i].dst, s.sending[i].i)
			}
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

	{
		s.heliumRainCooldown += dt
		if s.heliumRainCooldown > 0 {
			s.heliumRainCooldown -= 1

			s.pushItem(transform{
				vec{
					rand.Float64() * spritesPerWidth,
					100 + rand.Float64()*25,
				}, vec{0, 1},
			}, helium)
		}
	}

	for i := 0; i < len(s.faders); i++ {
		s.faders[i].s -= dt / 10
		if s.faders[i].s <= 0 {
			last := len(s.faders) - 1
			s.faders[i] = s.faders[last]
			s.faders = s.faders[:last]
			i--
			continue
		}
		s.faders[i].t.applyVelocity(dt)
	}

	{
		// moveUps := 0
		// moveDowns := 0
		minY := float64(-1000) // first band holds things going vertically
		maxY := float64(bandHeight)
		for j, b := range s.bands {
			// fmt.Println("BAND", j, minY, maxY)
			if j == len(s.bands)-1 {
				maxY += worldHeight // should be enough, lol
			}
			for i := range b.i {
				for k := 0; k < len(b.i[i].p); k++ {
					// fmt.Println(b.i[i].p[k][0])
					if b.i[i].p[k][1] < minY {
						s.bands[j-1].i[i].push(b.i[i].pop(k))
						k--
						// moveUps++
						continue
					}
					if b.i[i].p[k][1] >= maxY {
						s.bands[j+1].i[i].push(b.i[i].pop(k))
						k--
						// moveDowns++
						continue
					}
				}
			}
			minY = maxY
			maxY += bandHeight
		}
		// fmt.Println("UP", moveUps, "DOWN", moveDowns)

		for _, b := range s.bands {
			for i := range b.i {
				for len(b.i[i].p) > 200 {
					s.fader(b.i[i].pop(rand.Intn(len(b.i[i].p))), item(i))
				}
			}
		}
	}

	// {
	// 	bandPopulation := []int{}
	// 	for _, b := range s.bands {
	// 		sum := 0
	// 		for i := range b.i {
	// 			sum += len(b.i[i].p)
	// 		}
	// 		bandPopulation = append(bandPopulation, sum)
	// 	}
	// 	fmt.Println(bandPopulation)
	// 	panic("AT THE DISCO")
	// }

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
			s.foot += dt * s.footSpeed
			if s.foot > float64(s.scaffoldings) {
				s.foot = float64(s.scaffoldings)
			}
		} else {
			s.scaffoldingCost = [numItems]int{}
			if s.scaffoldings < 3 {

			} else if s.scaffoldings < 10 {
				s.scaffoldingCost[ice] += 5
			} else if s.scaffoldings < 50 {
				s.scaffoldingCost[silicon] += s.scaffoldings/20 + 1
				s.scaffoldingCost[carbon] += s.scaffoldings/20 + 1
			} else if s.scaffoldings < 60 {
				s.scaffoldingCost[nanotubes] += 5
				s.scaffoldingCost[silicon] += 5
			} else if s.scaffoldings < 120 {
				s.scaffoldingCost[nanotubes] += 20
				s.scaffoldingCost[silicon] += 20
			} else {
				s.scaffoldingCost[nanotubes] += 20
				s.scaffoldingCost[computer] += 20
			}
			canbuildnext := true
			for i := range s.scaffoldingCost {
				if s.scaffoldingCost[i] > s.footInv[i] {
					canbuildnext = false
				}
			}
			if canbuildnext && s.scaffoldings < 149 {
				for i := range s.scaffoldingCost {
					s.footInv[i] -= s.scaffoldingCost[i]
				}
				s.scaffoldings++
			}
		}
	}

	{ ////////////////////////////////////
		for xTile := range s.tiles {
			for yTile := range s.tiles[xTile] {
				s.tileCooldown[xTile][yTile] -= dt
				if s.tileCooldown[xTile][yTile] <= 0 {
					powerUsage := float64(0)
					switch s.tiles[xTile][yTile] {
					case extractor, fan, laser, fabricator:
						powerUsage = 0.5
					case weaver:
						powerUsage = 20
					}
					if s.powerLevel < powerUsage {
						continue
					}

					// s.tileCooldown[xTile][yTile] = 0

					switch s.tiles[xTile][yTile] {
					case extractor:
						if s.tileItems[xTile][yTile][ice] >= 10 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{1, 0}}, carbon)
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{0, -1}}, silicon)
							s.tileItems[xTile][yTile][ice] -= 10
							s.tileCooldown[xTile][yTile] = 3
							if yTile+1 < worldHeight && s.tiles[xTile][yTile+1] == filter {
								s.tileItems[xTile][yTile+1][water] = 5
							}
							s.powerLevel -= powerUsage
						}
					case filter:
						if s.tileItems[xTile][yTile][water] > 0 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{1, 0}}, water)
							s.tileItems[xTile][yTile][water]--
							s.tileCooldown[xTile][yTile] = 0.15
							s.powerLevel -= powerUsage
						}
					case weaver:
						if s.tileItems[xTile][yTile][carbon] >= 1 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{1, 0}}, nanotubes)
							s.tileItems[xTile][yTile][carbon] -= 1
							s.tileCooldown[xTile][yTile] = 0.5
							s.powerLevel -= powerUsage
						}
					case fabricator:
						if s.tileItems[xTile][yTile][silicon] >= 1 && s.tileItems[xTile][yTile][helium] >= 1 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{1, 0}}, computer)
							s.tileItems[xTile][yTile][carbon] -= 1
							s.tileItems[xTile][yTile][helium] -= 1
							s.tileCooldown[xTile][yTile] = 0.5
							s.powerLevel -= powerUsage
						}
					case fan:
						if yTile >= 50 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{0, 1}}, hydrogen)
							s.tileCooldown[xTile][yTile] = 10
							s.powerLevel -= powerUsage
						}
					case laser:
						if s.tileItems[xTile][yTile][hydrogen] >= 1 {
							s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{0, -1}}, plasma)
							s.tileCooldown[xTile][yTile] = 1
							s.tileItems[xTile][yTile][hydrogen] -= 1
							s.powerLevel -= powerUsage
						}
					case core:
						for s.tileCooldown[xTile][yTile] <= 0 {
							s.tileCooldown[xTile][yTile] += 0.05
							if rand.Intn(1000) < int(s.tileItems[xTile][yTile][plasma]) {
								speed := 1 + float64(s.tileItems[xTile][yTile][plasma])/1000
								angle := rand.Float64() * math.Pi * 2
								s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{speed * math.Cos(angle), speed * math.Sin(angle)}}, plasma)
								s.tileItems[xTile][yTile][plasma] -= 1
							}
						}
					}
				}
			}
		}
	}

	s.viewy += (s.ship.p[1] - s.viewy) * dt
	clamp(&s.viewy, s.ship.p[1]-1, s.ship.p[1]+1)
}

func randPosCenterTile(xTile, yTile int) vec {
	return vec{float64(xTile) + rand.Float64()*0.2 + 0.4, float64(yTile) + rand.Float64()*0.2 + 0.4}
}

const shipSize = 0.5

func (s *state) pushItem(i transform, t item) {
	bi := int(math.Floor(i.p[1] / bandHeight))
	if bi >= len(s.bands) {
		bi = len(s.bands) - 1
	}
	s.bands[bi].i[t].push(i)
}

func (s *state) fader(t transform, i item) {
	if math.Abs(t.p[1]-s.ship.p[1]) < 10 {
		t.v[0] = rand.Float64()*2 - 1
		t.v[1] = rand.Float64()*2 - 1
		s.faders = append(s.faders, fader{
			t, i, 0.1,
		})
	}
}

type fader struct {
	t transform
	i item
	s float64
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
	// rock
	// metal
	ice = item(iota)
	carbon
	silicon
	nanotubes
	computer
	water
	hydrogen
	plasma
	steam
	helium

	numItems
)

func (i item) canHold() bool {
	if i == water || i == steam || i == plasma {
		return false
	}
	return true
}

var itemSprite = map[item]string{
	// rock:      "rock",
	// metal:     "metal",
	ice:       "ice",
	carbon:    "carbon",
	silicon:   "silicon",
	nanotubes: "nanotubes",
	computer:  "computer",
	water:     "water",
	hydrogen:  "hydrogen",
	plasma:    "plasma",
	steam:     "steam",
	helium:    "helium",
}

type band struct {
	i [numItems]items
}

type items struct {
	p []vec
	v []vec
}

func tileLeftSide() (vec, vec) {
	return vec{0, 0.25}, vec{0.25, 0.75}
}

func tileRightSide() (vec, vec) {
	return vec{0.75, 0.25}, vec{1, 0.75}
}

func tileTopSide() (vec, vec) {
	return vec{0.25, 0}, vec{0.75, 0.25}
}

func tileBottomSide() (vec, vec) {
	return vec{0.25, 0.75}, vec{0.75, 1}
}

func tileCenter() (vec, vec) {
	return vec{0.24, 0.24}, vec{0.76, 0.76}
}

func (b *band) step(dt float64, bandIndex int) {
	for i := range b.i {
		ii := item(i)
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

			b.i[i].p[j][0] += b.i[i].v[j][0] * dt
			b.i[i].p[j][1] += b.i[i].v[j][1] * dt

			tilePos := b.i[i].p[j].floor()
			relTile := b.i[i].p[j].sub(tilePos)
			xTile, yTile := b.i[i].p[j].tilePos()

			storeInTile := false
			fade := false
			tt := s.tileAt(b.i[i].p[j])
			switch tt {
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
					if relTile.within(vec{0.4, 0.4}, vec{0.6, 0.6}) {
						b.i[i].v[j] = dir
						b.i[i].p[j] = tilePos.add(vec{rand.Float64()*0.2 + 0.4, rand.Float64()*0.2 + 0.4})
					} else {
						speed := 0.5
						if ii == plasma {
							speed = 3
						}
						b.i[i].v[j] = b.i[i].v[j].tween(tilePos.add(vec{0.5, 0.5}).sub(b.i[i].p[j]).norm(), speed*dt)
					}
				}
			// case forge:
			// 	if !relTile.within(vec{-1, 0.35}, vec{2, 0.65}) {
			// 		s.fader(b.i[i].pop(j), item(i))
			// 		j--
			// 		continue jLoop
			// 	}
			// 	if item(i) == rock && relTile.within(vec{0.35, 0.35}, vec{0.65, 0.65}) {
			// 		b.i[metal].push(b.i[i].pop(j))
			// 		j--
			// 		continue jLoop
			// 	}
			// case extractor, weaver, fabricator, laser:
			case extractor:
				if relTile.within(tileLeftSide()) && ii == ice {

				} else if (relTile.within(tileRightSide()) || relTile.within(tileCenter()) || relTile.within(tileTopSide())) && (ii == carbon || ii == silicon) {

				} else if relTile.within(tileCenter()) && ii == ice {
					storeInTile = true
				} else {
					fade = true
				}
			case weaver:
				if relTile.within(tileLeftSide()) && ii == carbon {
				} else if (relTile.within(tileRightSide()) || relTile.within(tileCenter())) && (ii == nanotubes) {
				} else if relTile.within(tileCenter()) && ii == carbon {
					storeInTile = true
				} else {
					fade = true
				}

			case fabricator:
				if relTile.within(tileLeftSide()) && (ii == silicon || ii == helium) {
				} else if (relTile.within(tileRightSide()) || relTile.within(tileCenter())) && (ii == computer) {
				} else if relTile.within(tileCenter()) && (ii == silicon || ii == helium) {
					storeInTile = true
				} else {
					fade = true
				}

			case fan:
				if (relTile.within(tileBottomSide()) || relTile.within(tileCenter())) && (ii == hydrogen) {
				} else {
					fade = true
				}
			case laser:
				if relTile.within(tileBottomSide()) && ii == hydrogen {
				} else if (relTile.within(tileTopSide()) || relTile.within(tileCenter())) && (ii == plasma) {
				} else if relTile.within(tileCenter()) && ii == hydrogen {
					storeInTile = true
				} else {
					fade = true
				}

			case core:
				if relTile.within(tileCenter()) && ii == plasma {
					if b.i[i].v[j].abs() <= 1 {
						storeInTile = true
					}
				} else if relTile.within(tileCenter()) {
					fade = true
				}

			case boiler:
				if relTile.within(tileLeftSide()) && ii == water {
				} else if relTile.within(tileCenter()) && ii == water {
					storeInTile = true
				} else if ii == plasma {
					if s.tileItems[xTile][yTile][water] > 0 {
						s.tileItems[xTile][yTile][water]--
						speed := b.i[i].v[j].abs()
						s.pushItem(transform{p: randPosCenterTile(xTile, yTile), v: vec{speed, 0}}, steam)
						b.i[i].pop(j)
						j--
						continue jLoop
					}
				} else if (relTile.within(tileRightSide()) || relTile.within(tileCenter())) && (ii == steam) {
				} else {
					fade = true
				}

			case splitter:
				if relTile.within(tileTopSide()) {
					switch rand.Intn(3) {
					case 0:
						b.i[i].v[j] = vec{-1, 0}
					case 1:
						b.i[i].v[j] = vec{1, 0}
					case 2:
						b.i[i].v[j] = vec{0, 1}
					}
					b.i[i].p[j] = tilePos.add(vec{rand.Float64()*0.2 + 0.4, rand.Float64()*0.2 + 0.4})
				}

			case turbine:
				if ii == steam && (relTile.within(tileCenter()) || relTile.within(tileLeftSide()) || relTile.within(tileRightSide())) {
					speed := b.i[i].v[j].abs()
					if relTile.within(tileCenter()) && speed > 1 {
						b.i[i].v[j] = b.i[i].v[j].norm()
						s.powerLevel = 50 * (speed - 1)
					}
				} else {
					fade = true
				}

			case empty:
				if ii.canHold() && b.i[i].p[j].within(shipCollectionMin, shipCollectionMax) && (s.inventory[i]+len(s.collecting[i])) < 100 && !(s.shipInFootZone && b.i[i].v[j][1] > 0) {
					s.collecting[i] = append(s.collecting[i], b.i[i].pop(j))
					j--
					continue jLoop
				}
			}

			if storeInTile {
				if s.tileItems[xTile][yTile][i] < maxItems[tt][i] {
					s.tileItems[xTile][yTile][i]++
					b.i[i].pop(j)
					j--
					continue jLoop
				} else {
					fade = true
				}
			}
			if fade {
				s.fader(b.i[i].pop(j), item(i))
				j--
				continue jLoop
			}

			if b.i[i].p[j][1] > s.foot+0.5 && b.i[i].p[j][1] < s.foot+2.5 {
				if b.i[i].p[j][0] > 3 && b.i[i].p[j][0] < 5 {
					if b.i[i].p[j][0] > 3.25 && b.i[i].p[j][0] < 4.75 && b.i[i].p[j][1] < s.foot+1 && b.i[i].v[j][1] > 0 {
						s.footInv[i]++
						b.i[i].pop(j)
					} else {
						s.fader(b.i[i].pop(j), item(i))
					}
					j--
					continue jLoop
				}
			}

			if b.i[i].p[j][0] < -1.5 || b.i[i].p[j][0] > spritesPerWidth+0.5 || b.i[i].p[j][1] < -30 || b.i[i].p[j][1] > 150 {
				b.i[i].pop(j)
				j--
			}
		}
	}
}

func (s *state) tileAt(v vec) tile {
	// vTile := v.TilePos()
	xTile, yTile := v.tilePos()
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

const bandHeight = 10
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
		// s.pushItem(pi, rock)
		s.pushItem(pi, ice)

		// i := len(rb.t)
		// rb.t = append(rb.t, transform{})
		// rb.t[i].p[0] = -1*spritesPerWidth/2 - 0.5
		// rb.t[i].p[1] = rb.height*rand.Float64() + rb.topY
		// rb.t[i].v[0] = math.Sqrt(1/(worldHeight-rb.t[i].p[1])) * 10
		// rb.t[i].applyVelocity(rb.nextSpawn)
	}
}

const worldHeight = 200

// const worldHeight = 100

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

func (v vec) tilePos() (x, y int) {
	v2 := v.floor()
	return int(v2[0]), int(v2[1])
}
