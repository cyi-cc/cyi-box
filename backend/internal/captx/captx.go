// Package captx 滑块拼图验证码出图：程序化 Memphis 几何背景 + 拼图块抠图。
// 无字体/素材依赖；每次随机形状与噪点，防图像匹配打码。
package captx

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/png"
	"math/big"
)

const (
	Width  = 300              // 底图宽
	Height = 170              // 底图高
	Size   = 48               // 拼图主体边长
	Knob   = 10               // 凹凸圆头半径
	Box    = Size + 2*Knob    // 拼图包围盒边长
	TolPX  = 4                // 校验容差（px）
)

// Puzzle 一次出题产物：BG 底图 PNG（带凹槽），Piece 拼图块 PNG（带 alpha），
// Y 为拼图块在底图的纵坐标（前端定位用），答案横坐标由调用方持有。
type Puzzle struct {
	BG    []byte
	Piece []byte
	Y     int
}

// 孟菲斯调色板（与前端 --cb 变量同色系）
var palette = []color.RGBA{
	{0x1c, 0x1b, 0x1a, 0xff}, // ink
	{0x16, 0x77, 0xff, 0xff}, // blue
	{0x07, 0xc1, 0x60, 0xff}, // green
	{0xf4, 0xa3, 0x26, 0xff}, // yellow
	{0xe8, 0x5d, 0x4a, 0xff}, // coral
	{0x8b, 0x5c, 0xf6, 0xff}, // violet
}

var cream = color.RGBA{0xf7, 0xf4, 0xec, 0xff}

func rnd(n int) int {
	if n <= 0 {
		return 0
	}
	b, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(b.Int64())
}

func pcol() color.RGBA { return palette[rnd(len(palette))] }

// New 生成一题，返回产物与答案横坐标（拼图包围盒左沿 px）。
func New() (*Puzzle, int, error) {
	bg := makeBG()

	// 缺口位置：主体矩形左上 (x0,y0)，包围盒左上 (x0-Knob, y0-Knob)
	x0 := 55 + rnd(Width-Size-Knob-55-12)
	y0 := 12 + rnd(Height-Size-Knob-12-14)
	bx, by := x0-Knob, y0-Knob

	piece := image.NewRGBA(image.Rect(0, 0, Box, Box))
	for v := 0; v < Box; v++ {
		for u := 0; u < Box; u++ {
			if !inPiece(u, v) {
				continue
			}
			src := bg.RGBAAt(bx+u, by+v)
			if pieceEdge(u, v) {
				src = shade(src, 0.55)
			}
			piece.SetRGBA(u, v, src)
		}
	}
	carveSocket(bg, bx, by)

	var b1, b2 bytes.Buffer
	if err := png.Encode(&b1, bg); err != nil {
		return nil, 0, err
	}
	if err := png.Encode(&b2, piece); err != nil {
		return nil, 0, err
	}
	return &Puzzle{BG: b1.Bytes(), Piece: b2.Bytes(), Y: by}, bx, nil
}

func dist2(x0, y0, x1, y1 int) int {
	dx, dy := x0-x1, y0-y1
	return dx*dx + dy*dy
}

// inPiece 拼图形状（包围盒内坐标）：方体 + 右凸 + 上凸 - 左凹 - 下凹
func inPiece(u, v int) bool {
	mid := Knob + Size/2
	in := u >= Knob && u < Knob+Size && v >= Knob && v < Knob+Size
	in = in || dist2(u, v, Knob+Size, mid) < Knob*Knob   // 右凸
	in = in || dist2(u, v, mid, Knob) < Knob*Knob       // 上凸
	in = in && dist2(u, v, Knob, mid) >= Knob*Knob      // 左凹
	in = in && dist2(u, v, mid, Knob+Size) >= Knob*Knob // 下凹
	return in
}

func pieceEdge(u, v int) bool {
	return inPiece(u, v) &&
		(!inPiece(u-1, v) || !inPiece(u+1, v) || !inPiece(u, v-1) || !inPiece(u, v+1))
}

// carveSocket 在底图刻凹槽：形状区压暗 + 边缘描边
func carveSocket(bg *image.RGBA, bx, by int) {
	for v := 0; v < Box; v++ {
		for u := 0; u < Box; u++ {
			if !inPiece(u, v) {
				continue
			}
			if pieceEdge(u, v) {
				bg.SetRGBA(bx+u, by+v, color.RGBA{0x1c, 0x1b, 0x1a, 0xff})
			} else {
				bg.SetRGBA(bx+u, by+v, shade(bg.RGBAAt(bx+u, by+v), 0.72))
			}
		}
	}
}

func shade(c color.RGBA, k float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * k),
		G: uint8(float64(c.G) * k),
		B: uint8(float64(c.B) * k),
		A: c.A,
	}
}

// ---- 背景：奶油底 + 随机几何形状 + 干扰线与噪点 ----

func makeBG() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			img.SetRGBA(x, y, cream)
		}
	}
	for i := 0; i < 3; i++ {
		line(img, rnd(Width), rnd(Height), rnd(Width), rnd(Height), 1+rnd(3), pcol())
	}
	for i := 0; i < 12; i++ {
		cx, cy := 8+rnd(Width-16), 8+rnd(Height-16)
		r := 5 + rnd(22)
		switch rnd(5) {
		case 0:
			fillCircle(img, cx, cy, r, pcol())
		case 1:
			strokeCircle(img, cx, cy, r, 2+rnd(2), pcol())
		case 2:
			fillRect(img, cx, cy, r+4, r+4, pcol())
		case 3:
			fillTriangle(img, cx, cy-r, cx-r, cy+r, cx+r, cy+r, pcol())
		default:
			// 十字
			c := pcol()
			line(img, cx-r/2, cy-r/2, cx+r/2, cy+r/2, 2, c)
			line(img, cx-r/2, cy+r/2, cx+r/2, cy-r/2, 2, c)
		}
	}
	// 噪点
	for i := 0; i < 160; i++ {
		img.SetRGBA(rnd(Width), rnd(Height), shade(pcol(), 0.7))
	}
	return img
}

func put(img *image.RGBA, x, y int, c color.RGBA) {
	if x >= 0 && x < Width && y >= 0 && y < Height {
		img.SetRGBA(x, y, c)
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if dist2(x, y, cx, cy) <= r*r {
				put(img, x, y, c)
			}
		}
	}
}

func strokeCircle(img *image.RGBA, cx, cy, r, w int, c color.RGBA) {
	inner := (r - w) * (r - w)
	outer := (r + w/2) * (r + w/2)
	for y := cy - r - w; y <= cy+r+w; y++ {
		for x := cx - r - w; x <= cx+r+w; x++ {
			d := dist2(x, y, cx, cy)
			if d >= inner && d <= outer {
				put(img, x, y, c)
			}
		}
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for yy := y - h/2; yy < y+h/2; yy++ {
		for xx := x - w/2; xx < x+w/2; xx++ {
			put(img, xx, yy, c)
		}
	}
}

// fillTriangle 重心符号法逐点测试（三角形小，开销可忽略）
func fillTriangle(img *image.RGBA, x0, y0, x1, y1, x2, y2 int, c color.RGBA) {
	minX, maxX := min3(x0, x1, x2), max3(x0, x1, x2)
	minY, maxY := min3(y0, y1, y2), max3(y0, y1, y2)
	d := (y1-y2)*(x0-x2) + (x2-x1)*(y0-y2)
	if d == 0 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			a := (y1-y2)*(x-x2) + (x2-x1)*(y-y2)
			b := (y2-y0)*(x-x2) + (x0-x2)*(y-y2)
			e := (y0-y1)*(x-x1) + (x1-x0)*(y-y1)
			if (d > 0 && a >= 0 && b >= 0 && e >= 0) || (d < 0 && a <= 0 && b <= 0 && e <= 0) {
				put(img, x, y, c)
			}
		}
	}
}

func line(img *image.RGBA, x0, y0, x1, y1, w int, c color.RGBA) {
	dx, dy := x1-x0, y1-y0
	steps := max(abs(dx), abs(dy))
	if steps == 0 {
		return
	}
	for i := 0; i <= steps; i++ {
		x := x0 + dx*i/steps
		y := y0 + dy*i/steps
		fillCircle(img, x, y, w, c)
	}
}

func min3(a, b, c int) int { return min(min(a, b), c) }
func max3(a, b, c int) int { return max(max(a, b), c) }
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
