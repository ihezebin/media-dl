package httpserver

import (
	"testing"
	"time"
)

func TestBehaviorCaptchaStoreVerify(t *testing.T) {
	store := &behaviorCaptchaStore{
		challenges: map[string]behaviorCaptcha{
			"slide":  {kind: "slide", expiresAt: time.Now().Add(time.Minute), targetX: 120, targetY: 80},
			"rotate": {kind: "rotate", expiresAt: time.Now().Add(time.Minute), rotateDiff: 90},
		},
		tokens: make(map[string]time.Time),
	}

	if _, _, ok := store.verify(behaviorCaptchaVerifyRequest{ID: "slide", Type: "slide", X: 1, Y: 1}); ok {
		t.Fatal("an incorrect slide position was accepted")
	}
	token, expiresAt, ok := store.verify(behaviorCaptchaVerifyRequest{ID: "slide", Type: "slide", X: 120, Y: 80})
	if !ok || token == "" || !expiresAt.After(time.Now()) {
		t.Fatalf("valid slide position was rejected: token=%q expires=%v", token, expiresAt)
	}
	if !store.validToken(token) {
		t.Fatal("issued captcha token is not valid")
	}
	if _, _, ok := store.verify(behaviorCaptchaVerifyRequest{ID: "slide", Type: "slide", X: 120, Y: 80}); ok {
		t.Fatal("a challenge was accepted more than once")
	}

	if _, _, ok := store.verify(behaviorCaptchaVerifyRequest{ID: "rotate", Type: "rotate", Angle: 0}); ok {
		t.Fatal("initial rotate angle was accepted")
	}
	if _, _, ok := store.verify(behaviorCaptchaVerifyRequest{ID: "rotate", Type: "rotate", Angle: 360 - 90}); !ok {
		t.Fatal("valid rotate angle was rejected")
	}
}

func TestBehaviorCaptchaStoreGenerate(t *testing.T) {
	store, err := newBehaviorCaptchaStore()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for i := 0; i < 24; i++ {
		challenge, err := store.generate()
		if err != nil {
			t.Fatal(err)
		}
		if challenge.ID == "" || challenge.Image == "" || challenge.Thumb == "" || !challenge.ExpiresAt.After(time.Now()) {
			t.Fatalf("incomplete generated challenge: %+v", challenge)
		}
		if challenge.Type == "rotate" && challenge.Angle != 0 {
			t.Fatalf("rotate challenge must start at zero angle: %+v", challenge)
		}
		seen[challenge.Type] = true
	}
	for _, kind := range []string{"slide", "drag", "rotate"} {
		if !seen[kind] {
			t.Fatalf("random generator did not produce %q in test sample", kind)
		}
	}
}
