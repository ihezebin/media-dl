package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"

	olympus "github.com/ihezebin/olympus/httpserver"
)

const (
	behaviorCaptchaTTL     = 5 * time.Minute
	behaviorCaptchaPadding = 5
)

type behaviorCaptchaStore struct {
	mu         sync.Mutex
	challenges map[string]behaviorCaptcha
	tokens     map[string]time.Time
	slide      slide.Captcha
	dragDrop   slide.Captcha
	rotate     rotate.Captcha
}

type behaviorCaptcha struct {
	kind       string
	expiresAt  time.Time
	targetX    int
	targetY    int
	rotateDiff int
}

type behaviorCaptchaResponse struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Image       string    `json:"image"`
	Thumb       string    `json:"thumb"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbX      int       `json:"thumb_x,omitempty"`
	ThumbY      int       `json:"thumb_y,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	Angle       int       `json:"angle"`
}

type behaviorCaptchaVerifyRequest struct {
	ID    string `json:"id" openapi:"required"`
	Type  string `json:"type" openapi:"required"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Angle int    `json:"angle"`
}

type behaviorCaptchaVerifyResponse struct {
	Verified  bool      `json:"verified"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func newBehaviorCaptchaStore() (*behaviorCaptchaStore, error) {
	backgrounds, err := imagesv2.GetImages()
	if err != nil {
		return nil, fmt.Errorf("加载验证码背景失败: %w", err)
	}
	assetGraphs, err := tiles.GetTiles()
	if err != nil {
		return nil, fmt.Errorf("加载验证码拼图资源失败: %w", err)
	}
	graphs := make([]*slide.GraphImage, 0, len(assetGraphs))
	for _, graph := range assetGraphs {
		graphs = append(graphs, &slide.GraphImage{
			OverlayImage: graph.OverlayImage,
			ShadowImage:  graph.ShadowImage,
			MaskImage:    graph.MaskImage,
		})
	}

	slideBuilder := slide.NewBuilder()
	slideBuilder.SetResources(slide.WithBackgrounds(backgrounds), slide.WithGraphImages(graphs))
	dragBuilder := slide.NewBuilder(slide.WithGenGraphNumber(2), slide.WithEnableGraphVerticalRandom(true))
	dragBuilder.SetResources(slide.WithBackgrounds(backgrounds), slide.WithGraphImages(graphs))
	rotateBuilder := rotate.NewBuilder(rotate.WithRangeAnglePos([]option.RangeVal{{Min: 20, Max: 330}}))
	rotateBuilder.SetResources(rotate.WithImages(backgrounds))

	return &behaviorCaptchaStore{
		challenges: make(map[string]behaviorCaptcha),
		tokens:     make(map[string]time.Time),
		slide:      slideBuilder.Make(),
		dragDrop:   dragBuilder.MakeDragDrop(),
		rotate:     rotateBuilder.Make(),
	}, nil
}

func (s *behaviorCaptchaStore) generate() (*behaviorCaptchaResponse, error) {
	kind, err := randomCaptchaKind()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	challenge := behaviorCaptcha{kind: kind, expiresAt: now.Add(behaviorCaptchaTTL)}
	response := &behaviorCaptchaResponse{Type: kind, ExpiresAt: challenge.expiresAt}

	switch kind {
	case "slide", "drag":
		captcha := s.slide
		if kind == "drag" {
			captcha = s.dragDrop
		}
		data, err := captcha.Generate()
		if err != nil {
			return nil, fmt.Errorf("生成验证码失败: %w", err)
		}
		block := data.GetData()
		if block == nil {
			return nil, fmt.Errorf("验证码数据为空")
		}
		response.Image, err = data.GetMasterImage().ToBase64()
		if err != nil {
			return nil, fmt.Errorf("编码验证码主图失败: %w", err)
		}
		response.Thumb, err = data.GetTileImage().ToBase64()
		if err != nil {
			return nil, fmt.Errorf("编码验证码拼图失败: %w", err)
		}
		response.ThumbX, response.ThumbY = block.DX, block.DY
		response.ThumbWidth, response.ThumbHeight = block.Width, block.Height
		challenge.targetX, challenge.targetY = block.X, block.Y
	case "rotate":
		data, err := s.rotate.Generate()
		if err != nil {
			return nil, fmt.Errorf("生成验证码失败: %w", err)
		}
		block := data.GetData()
		if block == nil {
			return nil, fmt.Errorf("验证码数据为空")
		}
		response.Image, err = data.GetMasterImage().ToBase64()
		if err != nil {
			return nil, fmt.Errorf("编码验证码主图失败: %w", err)
		}
		response.Thumb, err = data.GetThumbImage().ToBase64()
		if err != nil {
			return nil, fmt.Errorf("编码验证码缩略图失败: %w", err)
		}
		response.ThumbSize = block.Width
		// go-captcha-react reports an angle in the range [initial angle, 360°].
		// Start at zero so the user must rotate to 360-target; the initial state
		// must not already satisfy rotate.Validate(angle, target, padding).
		response.Angle = 0
		challenge.rotateDiff = block.Angle
	}

	id, err := randomCaptchaID()
	if err != nil {
		return nil, err
	}
	response.ID = id
	s.mu.Lock()
	s.cleanupLocked(now)
	s.challenges[id] = challenge
	s.mu.Unlock()
	return response, nil
}

func (s *behaviorCaptchaStore) verify(req behaviorCaptchaVerifyRequest) (string, time.Time, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	challenge, ok := s.challenges[req.ID]
	if !ok || challenge.kind != req.Type || now.After(challenge.expiresAt) {
		return "", time.Time{}, false
	}

	valid := false
	switch challenge.kind {
	case "slide", "drag":
		valid = slide.Validate(req.X, req.Y, challenge.targetX, challenge.targetY, behaviorCaptchaPadding)
	case "rotate":
		valid = rotate.Validate(req.Angle, challenge.rotateDiff, behaviorCaptchaPadding)
	}
	if !valid {
		return "", time.Time{}, false
	}

	token, err := randomCaptchaID()
	if err != nil {
		return "", time.Time{}, false
	}
	delete(s.challenges, req.ID)
	expiresAt := now.Add(behaviorCaptchaTTL)
	s.tokens[token] = expiresAt
	return token, expiresAt, true
}

func (s *behaviorCaptchaStore) validToken(token string) bool {
	if token == "" {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	expiresAt, ok := s.tokens[token]
	return ok && now.Before(expiresAt)
}

func (s *behaviorCaptchaStore) cleanupLocked(now time.Time) {
	for id, challenge := range s.challenges {
		if !now.Before(challenge.expiresAt) {
			delete(s.challenges, id)
		}
	}
	for token, expiresAt := range s.tokens {
		if !now.Before(expiresAt) {
			delete(s.tokens, token)
		}
	}
}

func randomCaptchaKind() (string, error) {
	var byteValue [1]byte
	if _, err := rand.Read(byteValue[:]); err != nil {
		return "", fmt.Errorf("生成验证码类型失败: %w", err)
	}
	return []string{"slide", "drag", "rotate"}[int(byteValue[0])%3], nil
}

func randomCaptchaID() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("生成验证码标识失败: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (s *Server) captcha(c *gin.Context, _ olympus.EmptyType) (*behaviorCaptchaResponse, error) {
	return s.captchaStore.generate()
}

func (s *Server) captchaVerify(_ *gin.Context, req behaviorCaptchaVerifyRequest) (*behaviorCaptchaVerifyResponse, error) {
	token, expiresAt, ok := s.captchaStore.verify(req)
	if !ok {
		return nil, badRequest(fmt.Errorf("验证码校验失败，请重新操作"))
	}
	return &behaviorCaptchaVerifyResponse{Verified: true, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Server) requireCaptcha(c *gin.Context) error {
	token := c.GetHeader("X-Captcha-Token")
	if s.captchaStore.validToken(token) {
		return nil
	}
	return olympus.NewError(olympus.CodeUnauthorized, "验证码已失效，请先完成验证")
}
