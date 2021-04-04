// Copyright © 2020 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// https://github.com/homeport/termshot

package main

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/esimov/stackblur-go"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

const (
	red    = "#ED655A"
	yellow = "#E1C04C"
	green  = "#71BD47"

	// known ansi sequences
	FG                  = "FG"
	BG                  = "BG"
	STR                 = "STR"
	URL                 = "URL"
	invertedColor       = "inverted"
	invertedColorSingle = "invertedsingle"
	fullColor           = "full"
	foreground          = "foreground"
	reset               = "reset"
	bold                = "bold"
	boldReset           = "boldr"
	italic              = "italic"
	italicReset         = "italicr"
	underline           = "underline"
	underlineReset      = "underliner"
	strikethrough       = "strikethrough"
	strikethroughReset  = "strikethroughr"
	color16             = "color16"
	left                = "left"
	osc99               = "osc99"
	lineChange          = "linechange"
	title               = "title"
	link                = "link"
)

//go:embed font/VictorMono-Bold.ttf
var victorMonoBold []byte

//go:embed font/VictorMono-Regular.ttf
var victorMonoRegular []byte

//go:embed font/VictorMono-Italic.ttf
var victorMonoItalic []byte

type RGB struct {
	r int
	g int
	b int
}

func NewRGBColor(ansiColor string) *RGB {
	colors := strings.Split(ansiColor, ";")
	r, _ := strconv.Atoi(colors[0])
	g, _ := strconv.Atoi(colors[1])
	b, _ := strconv.Atoi(colors[2])
	return &RGB{
		r: r,
		g: g,
		b: b,
	}
}

type ImageRenderer struct {
	ansiString string
	author     string
	formats    *ansiFormats

	factor float64

	columns int
	rows    int

	defaultForegroundColor *RGB
	defaultBackgroundColor *RGB

	shadowBaseColor string
	shadowRadius    uint8
	shadowOffsetX   float64
	shadowOffsetY   float64

	padding float64
	margin  float64

	regular     font.Face
	bold        font.Face
	italic      font.Face
	lineSpacing float64

	// canvas switches
	style                string
	backgroundColor      *RGB
	foregroundColor      *RGB
	ansiSequenceRegexMap map[string]string
}

func NewImageRenderer(content, author string) ImageRenderer {
	f := 2.0

	fontRegular, _ := truetype.Parse(victorMonoRegular)
	fontBold, _ := truetype.Parse(victorMonoBold)
	fontItalic, _ := truetype.Parse(victorMonoItalic)
	fontFaceOptions := &truetype.Options{Size: f * 12, DPI: 144}

	formats := &ansiFormats{}
	formats.init(shelly)

	return ImageRenderer{
		ansiString: content,
		author:     author,
		formats:    formats,

		defaultForegroundColor: &RGB{255, 255, 255},
		defaultBackgroundColor: &RGB{21, 21, 21},

		factor: f,

		columns: 80,
		rows:    25,

		margin:  f * 48,
		padding: f * 24,

		shadowBaseColor: "#10101066",
		shadowRadius:    uint8(math.Min(f*16, 255)),
		shadowOffsetX:   f * 16,
		shadowOffsetY:   f * 16,

		regular:     truetype.NewFace(fontRegular, fontFaceOptions),
		bold:        truetype.NewFace(fontBold, fontFaceOptions),
		italic:      truetype.NewFace(fontItalic, fontFaceOptions),
		lineSpacing: 1.2,

		ansiSequenceRegexMap: map[string]string{
			invertedColor:       `^(?P<STR>(\x1b\[38;2;(?P<BG>(\d+;?){3});49m){1}(\x1b\[7m))`,
			invertedColorSingle: `^(?P<STR>\x1b\[(?P<BG>\d{2,3});49m\x1b\[7m)`,
			fullColor:           `^(?P<STR>(\x1b\[48;2;(?P<BG>(\d+;?){3})m)(\x1b\[38;2;(?P<FG>(\d+;?){3})m))`,
			foreground:          `^(?P<STR>(\x1b\[38;2;(?P<FG>(\d+;?){3})m))`,
			reset:               `^(?P<STR>\x1b\[0m)`,
			bold:                `^(?P<STR>\x1b\[1m)`,
			boldReset:           `^(?P<STR>\x1b\[22m)`,
			italic:              `^(?P<STR>\x1b\[3m)`,
			italicReset:         `^(?P<STR>\x1b\[23m)`,
			underline:           `^(?P<STR>\x1b\[4m)`,
			underlineReset:      `^(?P<STR>\x1b\[24m)`,
			strikethrough:       `^(?P<STR>\x1b\[9m)`,
			strikethroughReset:  `^(?P<STR>\x1b\[29m)`,
			color16:             `^(?P<STR>\x1b\[(?P<FG>\d{2,3})m)`,
			left:                `^(?P<STR>\x1b\[(\d{1,3})D)`,
			osc99:               `^(?P<STR>\x1b\]9;9;(.+)\x1b\\)`,
			lineChange:          `^(?P<STR>\x1b\[(\d)[FB])`,
			title:               `^(?P<STR>\x1b\]0;(.+)\007)`,
			link:                `^(?P<STR>\x1b]8;;file:\/\/(.+)\x1b\\(?P<URL>.+)\x1b]8;;\x1b\\)`,
		},
	}
}

func (s *ImageRenderer) fontHeight() float64 {
	return float64(s.regular.Metrics().Height >> 6)
}

func (s *ImageRenderer) calculateWidth() int {
	longest := 0
	for _, line := range strings.Split(s.ansiString, "\n") {
		length := s.formats.lenWithoutANSI(line)
		if length > longest {
			longest = length
		}
	}
	return longest
}

func (s *ImageRenderer) measureContent() (width, height float64) {
	hasRPrompt := strings.Contains(s.ansiString, "\x1b7")

	RPrompt := "RPROMPT"
	// clean string before render
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b[m", "\x1b[0m")
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b[K", "")
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b7", fmt.Sprintf("_%s", strings.Repeat(" ", 30)))
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b8", "")
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b[1F", "")
	s.ansiString = strings.ReplaceAll(s.ansiString, "\x1b[1000C", RPrompt)
	if !hasRPrompt {
		s.ansiString += fmt.Sprintf("_%s", strings.Repeat(" ", 30))
	}

	s.ansiString += "\n\n\x1b[1mhttps://ohmyposh.dev\x1b[22m"

	if len(s.author) > 0 {
		createdBy := fmt.Sprintf(" by \x1b[1m%s\x1b[22m", s.author)
		s.ansiString += createdBy
	}

	// clean abundance of empty lines
	s.ansiString = strings.Trim(s.ansiString, "\n")
	s.ansiString = "\n" + s.ansiString

	// get the longest line
	linewidth := s.calculateWidth()

	// replace all RPROMPT occurrences
	rPromptLen := len(RPrompt)
	ansiLines := strings.Split(s.ansiString, "\n")
	for i, line := range ansiLines {
		if !strings.Contains(line, RPrompt) {
			continue
		}
		lineLength := s.formats.lenWithoutANSI(line)
		if lineLength >= linewidth {
			line = strings.Replace(ansiLines[i], RPrompt, strings.Repeat(" ", rPromptLen), 1)
			ansiLines[i] = line
			continue
		}
		leftOverLength := linewidth - lineLength - rPromptLen
		leftOverLength = int(float64(leftOverLength) * 1.47) // 1.47 is the magic number to align everything
		line = strings.Replace(ansiLines[i], RPrompt, strings.Repeat(" ", leftOverLength), 1)
		ansiLines[i] = line
	}
	s.ansiString = strings.Join(ansiLines, "\n")

	// width, taken from the longest line
	tmpDrawer := &font.Drawer{Face: s.regular}
	advance := tmpDrawer.MeasureString(strings.Repeat(" ", linewidth))
	width = float64(advance >> 6)

	// height, lines times font height and line spacing
	height = float64(len(ansiLines)) * s.fontHeight() * s.lineSpacing

	return width, height
}

func (s *ImageRenderer) SavePNG(path string) error {
	var f = func(value float64) float64 { return s.factor * value }

	var (
		corner   = f(6)
		radius   = f(9)
		distance = f(25)
	)

	contentWidth, contentHeight := s.measureContent()

	// Make sure the output window is big enough in case no content or very few
	// content will be rendered
	contentWidth = math.Max(contentWidth, 3*distance+3*radius)

	marginX, marginY := s.margin, s.margin
	paddingX, paddingY := s.padding, s.padding

	xOffset := marginX
	yOffset := marginY
	titleOffset := f(40)

	width := contentWidth + 2*marginX + 2*paddingX
	height := contentHeight + 2*marginY + 2*paddingY + titleOffset

	dc := gg.NewContext(int(width), int(height))

	xOffset -= s.shadowOffsetX / 2
	yOffset -= s.shadowOffsetY / 2

	bc := gg.NewContext(int(width), int(height))
	bc.DrawRoundedRectangle(xOffset+s.shadowOffsetX, yOffset+s.shadowOffsetY, width-2*marginX, height-2*marginY, corner)
	bc.SetHexColor(s.shadowBaseColor)
	bc.Fill()

	var done = make(chan struct{}, s.shadowRadius)
	shadow := stackblur.Process(
		bc.Image(),
		uint32(width),
		uint32(height),
		uint32(s.shadowRadius),
		done,
	)

	<-done
	dc.DrawImage(shadow, 0, 0)

	// Draw rounded rectangle with outline and three button to produce the
	// impression of a window with controls and a content area
	dc.DrawRoundedRectangle(xOffset, yOffset, width-2*marginX, height-2*marginY, corner)
	dc.SetHexColor("#151515")
	dc.Fill()

	dc.DrawRoundedRectangle(xOffset, yOffset, width-2*marginX, height-2*marginY, corner)
	dc.SetHexColor("#404040")
	dc.SetLineWidth(f(1))
	dc.Stroke()

	for i, color := range []string{red, yellow, green} {
		dc.DrawCircle(xOffset+paddingX+float64(i)*distance+f(4), yOffset+paddingY+f(4), radius)
		dc.SetHexColor(color)
		dc.Fill()
	}

	// Apply the actual text into the prepared content area of the window
	var x, y float64 = xOffset + paddingX, yOffset + paddingY + titleOffset + s.fontHeight()

	for len(s.ansiString) != 0 {
		if !s.shouldPrint() {
			continue
		}
		runes := []rune(s.ansiString)
		str := string(runes[0:1])
		s.ansiString = string(runes[1:])
		switch s.style {
		case bold:
			dc.SetFontFace(s.bold)
		case italic:
			dc.SetFontFace(s.italic)
		default:
			dc.SetFontFace(s.regular)
		}

		w, h := dc.MeasureString(str)
		if s.backgroundColor != nil {
			dc.SetRGB255(s.backgroundColor.r, s.backgroundColor.g, s.backgroundColor.b)
			dc.DrawRectangle(x, y-h, w, h+12)
			dc.Fill()
		}
		if s.foregroundColor != nil {
			dc.SetRGB255(s.foregroundColor.r, s.foregroundColor.g, s.foregroundColor.b)
		} else {
			dc.SetRGB255(s.defaultForegroundColor.r, s.defaultForegroundColor.g, s.defaultForegroundColor.b)
		}

		str = s.correctString(str)
		if str == "\n" {
			x = xOffset + paddingX
			y += h * s.lineSpacing
			continue
		}

		dc.DrawString(str, x, y)

		if s.style == underline {
			dc.DrawLine(x, y+f(4), x+w, y+f(4))
			dc.SetLineWidth(f(1))
			dc.Stroke()
		}

		x += w
	}

	return dc.SavePNG(path)
}

func (s *ImageRenderer) correctString(str string) string {
	switch str {
	case "✗": // mitigate issue #1 by replacing it with a similar character
		return "×"
	case "\u276F", "\u279C":
		return ">"
	case "\uF449":
		return "\uE26E"
	case "┏", "┖":
		return "-"
	case "●":
		return "o"
	case "\u2593":
		return "\u2588"
	case "\u276E":
		return "<"
	}
	return str
}

func (s *ImageRenderer) shouldPrint() bool {
	for sequence, regex := range s.ansiSequenceRegexMap {
		match := findNamedRegexMatch(regex, s.ansiString)
		if len(match) == 0 {
			continue
		}
		s.ansiString = strings.TrimPrefix(s.ansiString, match[STR])
		switch sequence {
		case invertedColor:
			s.foregroundColor = s.defaultBackgroundColor
			s.backgroundColor = NewRGBColor(match[BG])
			return false
		case invertedColorSingle:
			s.foregroundColor = s.defaultBackgroundColor
			color, _ := strconv.Atoi(match[BG])
			color += 10
			s.setBase16Color(fmt.Sprint(color))
			return false
		case fullColor:
			s.foregroundColor = NewRGBColor(match[FG])
			s.backgroundColor = NewRGBColor(match[BG])
			return false
		case foreground:
			s.foregroundColor = NewRGBColor(match[FG])
			return false
		case reset:
			s.foregroundColor = s.defaultForegroundColor
			s.backgroundColor = nil
			return false
		case bold, italic, underline:
			s.style = sequence
			return false
		case boldReset, italicReset, underlineReset:
			s.style = ""
			return false
		case strikethrough, strikethroughReset, left, osc99, lineChange, title:
			return false
		case color16:
			s.setBase16Color(match[FG])
			return false
		case link:
			s.ansiString = match[URL] + s.ansiString
		}
	}
	return true
}

func (s *ImageRenderer) setBase16Color(colorStr string) {
	color := s.defaultForegroundColor
	colorInt, err := strconv.Atoi(colorStr)
	if err != nil {
		s.foregroundColor = color
	}
	switch colorInt {
	case 30, 40: // Black
		color = &RGB{1, 1, 1}
	case 31, 41: // Red
		color = &RGB{222, 56, 43}
	case 32, 42: // Green
		color = &RGB{57, 181, 74}
	case 33, 43: // Yellow
		color = &RGB{255, 199, 6}
	case 34, 44: // Blue
		color = &RGB{0, 111, 184}
	case 35, 45: // Magenta
		color = &RGB{118, 38, 113}
	case 36, 46: // Cyan
		color = &RGB{44, 181, 233}
	case 37, 47: // White
		color = &RGB{204, 204, 204}
	case 90, 100: // Bright Black (Gray)
		color = &RGB{128, 128, 128}
	case 91, 101: // Bright Red
		color = &RGB{255, 0, 0}
	case 92, 102: // Bright Green
		color = &RGB{0, 255, 0}
	case 93, 103: // Bright Yellow
		color = &RGB{255, 255, 0}
	case 94, 104: // Bright Blue
		color = &RGB{0, 0, 255}
	case 95, 105: // Bright Magenta
		color = &RGB{255, 0, 255}
	case 96, 106: // Bright Cyan
		color = &RGB{101, 194, 205}
	case 97, 107: // Bright White
		color = &RGB{255, 255, 255}
	}
	if colorInt < 40 || (colorInt >= 90 && colorInt < 100) {
		s.foregroundColor = color
		return
	}
	s.backgroundColor = color
}
