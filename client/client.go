package main

import (
	"syscall/js"
)

func main() {
	s := &state{}
	r := NewRender()
	r.render(s)

	c := client{
		r: r,
		s: s,
	}
	c.animationFrameJs = js.FuncOf(c.animationFrame)
	c.animationFrame(js.Null(), nil)

	waitforever := make(chan struct{})
	<-waitforever
}

type client struct {
	r                *render
	s                *state
	animationFrameJs js.Func
}

func (c *client) animationFrame(this js.Value, args []js.Value) interface{} {
	c.r.render(c.s)

	js.Global().Get("window").Call("requestAnimationFrame", c.animationFrameJs)
	return nil
}

type render struct {
	container js.Value
	canvas    js.Value
	ctx       js.Value
	width     int
	height    int
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

func (r *render) render(s *state) {
	{
		height := r.container.Get("clientHeight").Int()
		width := r.container.Get("clientWidth").Int()

		if height != r.height || width != r.width {
			r.height = height
			r.width = width
			r.canvas.Set("height", height)
			r.canvas.Set("width", width)
		}
	}

	r.ctx.Set("fillStyle", "#000000")
	r.ctx.Call("fillRect", 0, 0, r.width, r.height)

	r.ctx.Call("drawImage", getImage("circle"), 0, 0)
}

type state struct {
}

func valuesToInterfaces(arr []js.Value) []interface{} {
	r := make([]interface{}, len(arr))
	for i, v := range arr {
		r[i] = v
	}
	return r
}

var cachedImages = map[string]js.Value{}

func getImage(id string) js.Value {
	if v, ok := cachedImages[id]; ok {
		return v
	}

	v := js.Global().Get("Image").New()
	v.Set("src", "sprites/"+id+".svg")
	return v
}
