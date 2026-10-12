package bilibili

import (
	"testing"

	"github.com/hezebin/media-dl/internal/video/model"
)

func TestExtractFormatsPrefersDASHOverLegacyDURL(t *testing.T) {
	play := map[string]any{
		"dash": map[string]any{
			"video": []any{map[string]any{
				"id": 80, "baseUrl": "https://video.example/1080.m4s",
				"width": 1920, "height": 1080, "codecs": "avc1",
			}},
			"audio": []any{map[string]any{
				"id": 30280, "baseUrl": "https://audio.example/audio.m4s", "bandwidth": 128000,
			}},
		},
		"quality": 16,
		"durl": []any{map[string]any{
			"url": "https://video.example/low.mp4", "size": 100,
		}},
	}

	formats := extractFormats(play, nil)
	if len(formats) != 1 || formats[0].FormatID != "dash_80" {
		t.Fatalf("formats = %+v, want only DASH video", formats)
	}
	if best := (&model.VideoInfo{Formats: formats}).BestFormat(); best == nil || best.FormatID != "dash_80" {
		t.Fatalf("best format = %+v, want dash_80", best)
	}
}
