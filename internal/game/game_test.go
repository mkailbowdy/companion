package game

import (
	"math"
	"testing"
	"time"

	"bmo.pushiro.com/internal/expression"
)

func TestTransitionProgress(t *testing.T) {
	tests := []struct {
		elapsed time.Duration
		want    float64
	}{
		{-time.Second, 0},
		{0, 0},
		{transitionDuration / 2, 0.5},
		{transitionDuration, 1},
		{time.Second, 1},
	}
	for _, test := range tests {
		if got := transitionProgress(test.elapsed); got != test.want {
			t.Errorf("transitionProgress(%v) = %v, want %v", test.elapsed, got, test.want)
		}
	}
}

func TestEveryExpressionProducesFinitePose(t *testing.T) {
	for _, emotion := range expression.Emotions {
		for _, activity := range expression.Activities {
			pose := poseFor(expression.ExpressionCommand{Emotion: emotion, Activity: activity})
			values := []float64{
				pose.eyeOpen, pose.eyeScale, pose.browTilt, pose.mouthWidth,
				pose.mouthOpen, pose.mouthCurve, pose.gazeX, pose.gazeY,
			}
			for _, value := range values {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Fatalf("%s/%s produced invalid pose: %+v", emotion, activity, pose)
				}
			}
		}
	}
}

func TestSetCommandStartsFromCurrentInterpolatedPose(t *testing.T) {
	now := time.Unix(100, 0)
	clock := func() time.Time { return now }
	game := NewGame(expression.NewExpressionInbox(), clock, 1)
	game.setCommand(expression.ExpressionCommand{Emotion: expression.EmotionHappy, Activity: expression.ActivityNeutral}, now)

	now = now.Add(transitionDuration / 2)
	expected := interpolatePose(
		poseFor(expression.ExpressionCommand{Emotion: expression.EmotionNeutral, Activity: expression.ActivityNeutral}),
		poseFor(expression.ExpressionCommand{Emotion: expression.EmotionHappy, Activity: expression.ActivityNeutral}),
		0.5,
	)
	game.setCommand(expression.ExpressionCommand{Emotion: expression.EmotionSad, Activity: expression.ActivityNeutral}, now)

	if game.from != expected {
		t.Fatalf("transition started from %+v, want %+v", game.from, expected)
	}
}
