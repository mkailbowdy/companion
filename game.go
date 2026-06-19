package main

import (
	"image/color"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const transitionDuration = 250 * time.Millisecond

var (
	faceColor = color.RGBA{R: 166, G: 224, B: 205, A: 255}
	inkColor  = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	tearColor = color.RGBA{R: 72, G: 177, B: 224, A: 220}
)

type facePose struct {
	eyeOpen, eyeScale, browTilt float64
	mouthWidth, mouthOpen       float64
	mouthCurve, gazeX, gazeY    float64
}

type Game struct {
	inbox *ExpressionInbox
	now   func() time.Time
	rng   *rand.Rand

	command      ExpressionCommand
	pose         facePose
	from, target facePose
	transitionAt time.Time
	startedAt    time.Time
	nextBlink    time.Time
	blinkAt      time.Time

	emotionIndex, activityIndex int
}

func NewGame(inbox *ExpressionInbox, now func() time.Time, seed int64) *Game {
	t := now()
	cmd := ExpressionCommand{Emotion: EmotionNeutral, Activity: ActivityNeutral}
	pose := poseFor(cmd)
	g := &Game{
		inbox: inbox, now: now, rng: rand.New(rand.NewSource(seed)),
		command: cmd, pose: pose, from: pose, target: pose,
		transitionAt: t, startedAt: t,
	}
	g.scheduleBlink(t)
	return g
}

func (g *Game) Update() error {
	now := g.now()
	g.handleKeyboard()
	if cmd, ok := g.inbox.Latest(); ok {
		g.setCommand(cmd, now)
	}
	g.pose = interpolatePose(g.from, g.target, transitionProgress(now.Sub(g.transitionAt)))
	if !now.Before(g.nextBlink) && g.blinkAt.IsZero() {
		g.blinkAt = now
	}
	if !g.blinkAt.IsZero() && now.Sub(g.blinkAt) > 180*time.Millisecond {
		g.blinkAt = time.Time{}
		g.scheduleBlink(now)
	}
	return nil
}

func (g *Game) setCommand(cmd ExpressionCommand, now time.Time) {
	cmd = normalizeCommand(cmd)
	if cmd == g.command {
		return
	}
	g.pose = interpolatePose(g.from, g.target, transitionProgress(now.Sub(g.transitionAt)))
	g.from = g.pose
	g.target = poseFor(cmd)
	g.transitionAt = now
	g.command = cmd
}

func (g *Game) handleKeyboard() {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.emotionIndex = (g.emotionIndex + 1) % len(Emotions)
		g.inbox.Submit(ExpressionCommand{Emotion: Emotions[g.emotionIndex], Activity: g.command.Activity})
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.emotionIndex = (g.emotionIndex - 1 + len(Emotions)) % len(Emotions)
		g.inbox.Submit(ExpressionCommand{Emotion: Emotions[g.emotionIndex], Activity: g.command.Activity})
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.activityIndex = (g.activityIndex + 1) % len(Activities)
		g.inbox.Submit(ExpressionCommand{Emotion: g.command.Emotion, Activity: Activities[g.activityIndex]})
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.activityIndex = (g.activityIndex - 1 + len(Activities)) % len(Activities)
		g.inbox.Submit(ExpressionCommand{Emotion: g.command.Emotion, Activity: Activities[g.activityIndex]})
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.inbox.Submit(ExpressionCommand{Emotion: EmotionNeutral, Activity: ActivityNeutral})
		g.emotionIndex, g.activityIndex = 0, 0
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(faceColor)
	t := g.now().Sub(g.startedAt).Seconds()
	pose := g.animatedPose(t)
	bob := math.Sin(t*2.1) * 2
	if g.command.Activity == ActivityLaughing {
		bob = math.Sin(t*12) * 7
	}
	g.drawEyes(screen, pose, bob, t)
	g.drawBrows(screen, pose, bob)
	g.drawMouth(screen, pose, bob)
	if g.command.Activity == ActivityCrying {
		g.drawTears(screen, t, bob)
	}
}

func (g *Game) Layout(_, _ int) (int, int) { return screenWidth, screenHeight }

func (g *Game) animatedPose(t float64) facePose {
	p := g.pose
	p.gazeX += math.Sin(t*0.7) * 0.025
	p.gazeY += math.Sin(t*0.9) * 0.012

	if g.command.Activity == ActivityBlinking || g.naturalBlinkAmount(g.now()) > 0 {
		amount := g.naturalBlinkAmount(g.now())
		if g.command.Activity == ActivityBlinking {
			// A short close/open cycle every 1.2 seconds.
			phase := math.Mod(t, 1.2)
			amount = 0
			if phase < 0.22 {
				amount = math.Sin((phase / 0.22) * math.Pi)
			}
		}
		p.eyeOpen *= 1 - amount
	}
	switch g.command.Activity {
	case ActivityTalking:
		p.mouthOpen = 0.2 + 0.75*math.Abs(math.Sin(t*9.5))
		p.mouthWidth *= 0.9 + 0.1*math.Sin(t*4)
	case ActivityLaughing:
		p.eyeOpen = 0.08
		p.mouthOpen = 0.9 + 0.1*math.Sin(t*12)
		p.mouthCurve = 1
	case ActivityCrying:
		p.mouthCurve = -0.85
		p.mouthOpen = 0.1 + 0.12*math.Abs(math.Sin(t*13))
	case ActivityThinking:
		p.gazeX, p.gazeY = 0.28, -0.28
		p.mouthWidth *= 0.65
	case ActivityListening:
		p.eyeScale *= 1 + 0.06*math.Sin(t*4)
		p.gazeX = 0.08 * math.Sin(t*1.5)
	}
	return p
}

func (g *Game) drawEyes(dst *ebiten.Image, p facePose, bob, t float64) {
	for _, x := range []float64{430, 850} {
		y := 275 + bob
		w := 40 * p.eyeScale
		h := math.Max(5, 40*p.eyeOpen*p.eyeScale)
		fillEllipse(dst, float32(x), float32(y), float32(w), float32(h), inkColor)
	}
}

func (g *Game) drawBrows(dst *ebiten.Image, p facePose, bob float64) {
	if math.Abs(p.browTilt) < 0.05 {
		return
	}
	for index, x := range []float64{430, 850} {
		direction := 1.0
		if index == 1 {
			direction = -1
		}
		y := 165 + bob
		dy := p.browTilt * direction * 28
		vector.StrokeLine(dst, float32(x-75), float32(y-dy), float32(x+75), float32(y+dy), 17, inkColor, true)
	}
}

func (g *Game) drawMouth(dst *ebiten.Image, p facePose, bob float64) {
	cx, cy := float32(640), float32(490+bob)
	halfWidth := float32(165 * p.mouthWidth)
	open := float32(100 * p.mouthOpen)
	curve := float32(105 * p.mouthCurve)

	if open > 18 {
		var filled vector.Path
		filled.MoveTo(cx-halfWidth, cy)
		filled.QuadTo(cx, cy+curve+open, cx+halfWidth, cy)
		filled.QuadTo(cx, cy+curve-open, cx-halfWidth, cy)
		filled.Close()
		vector.FillPath(dst, &filled, nil, &vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScale(inkColor)})
		return
	}
	var path vector.Path
	path.MoveTo(cx-halfWidth, cy)
	path.QuadTo(cx, cy+curve, cx+halfWidth, cy)
	vector.StrokePath(dst, &path, &vector.StrokeOptions{Width: 22, LineCap: vector.LineCapRound, LineJoin: vector.LineJoinRound}, &vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScale(inkColor)})
}

func (g *Game) drawTears(dst *ebiten.Image, t, bob float64) {
	for index, x := range []float64{430, 850} {
		offset := math.Mod(t*190+float64(index)*85, 230)
		y := 325 + offset + bob
		r := 13 + offset/24
		vector.DrawFilledCircle(dst, float32(x), float32(y), float32(r), tearColor, true)
	}
}

func (g *Game) scheduleBlink(now time.Time) {
	delay := 2300 + g.rng.Intn(3300)
	g.nextBlink = now.Add(time.Duration(delay) * time.Millisecond)
}

func (g *Game) naturalBlinkAmount(now time.Time) float64 {
	if g.blinkAt.IsZero() {
		return 0
	}
	x := float64(now.Sub(g.blinkAt)) / float64(180*time.Millisecond)
	if x < 0 || x > 1 {
		return 0
	}
	return math.Sin(x * math.Pi)
}

func poseFor(cmd ExpressionCommand) facePose {
	p := facePose{eyeOpen: 1, eyeScale: 1, mouthWidth: 1, mouthCurve: 0.05}
	switch cmd.Emotion {
	case EmotionHappy:
		p.eyeOpen, p.mouthCurve = 0.78, 0.85
	case EmotionSad:
		p.eyeOpen, p.browTilt, p.mouthCurve = 0.82, -0.65, -0.7
	case EmotionAngry:
		p.eyeOpen, p.browTilt, p.mouthCurve = 0.62, 0.9, -0.35
	case EmotionSurprised:
		p.eyeScale, p.mouthWidth, p.mouthOpen = 1.2, 0.42, 0.85
	case EmotionScared:
		p.eyeScale, p.browTilt, p.mouthWidth, p.mouthOpen = 1.25, -0.35, 0.55, 0.7
	case EmotionConfused:
		p.eyeOpen, p.browTilt, p.mouthWidth, p.mouthCurve = 0.82, 0.45, 0.72, -0.12
		p.gazeX = 0.18
	case EmotionSleepy:
		p.eyeOpen, p.eyeScale, p.mouthWidth = 0.22, 0.95, 0.6
	case EmotionExcited:
		p.eyeScale, p.mouthCurve, p.mouthOpen = 1.25, 0.9, 0.55
	}
	return p
}

func interpolatePose(a, b facePose, t float64) facePose {
	t = t * t * (3 - 2*t)
	lerp := func(x, y float64) float64 { return x + (y-x)*t }
	return facePose{
		eyeOpen: lerp(a.eyeOpen, b.eyeOpen), eyeScale: lerp(a.eyeScale, b.eyeScale),
		browTilt: lerp(a.browTilt, b.browTilt), mouthWidth: lerp(a.mouthWidth, b.mouthWidth),
		mouthOpen: lerp(a.mouthOpen, b.mouthOpen), mouthCurve: lerp(a.mouthCurve, b.mouthCurve),
		gazeX: lerp(a.gazeX, b.gazeX), gazeY: lerp(a.gazeY, b.gazeY),
	}
}

func transitionProgress(elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= transitionDuration {
		return 1
	}
	return float64(elapsed) / float64(transitionDuration)
}

func colorScale(c color.RGBA) ebiten.ColorScale {
	var scale ebiten.ColorScale
	scale.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
	return scale
}

func fillEllipse(dst *ebiten.Image, cx, cy, rx, ry float32, fill color.RGBA) {
	// Four cubic Bézier segments approximate an ellipse closely and scale
	// cleanly down to the thin shapes used during blinks.
	const k = float32(0.5522847498)
	var path vector.Path
	path.MoveTo(cx+rx, cy)
	path.CubicTo(cx+rx, cy+k*ry, cx+k*rx, cy+ry, cx, cy+ry)
	path.CubicTo(cx-k*rx, cy+ry, cx-rx, cy+k*ry, cx-rx, cy)
	path.CubicTo(cx-rx, cy-k*ry, cx-k*rx, cy-ry, cx, cy-ry)
	path.CubicTo(cx+k*rx, cy-ry, cx+rx, cy-k*ry, cx+rx, cy)
	path.Close()
	vector.FillPath(dst, &path, nil, &vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScale(fill)})
}
