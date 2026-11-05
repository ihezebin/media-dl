package remote

import (
	"net/url"
	"strings"

	musicmodel "github.com/guohuiyuan/music-lib/model"
)

func UnproxyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	target, err := url.Parse(raw)
	if err != nil || (target.Path != proxyPath && target.Path != legacyProxyPath) {
		return raw
	}
	if original := strings.TrimSpace(target.Query().Get("url")); original != "" {
		return original
	}
	return raw
}

func UnproxyMusicSong(song *musicmodel.Song) {
	TransformMusicSong(song, UnproxyURL)
}

func TransformMusicSong(song *musicmodel.Song, transform func(string) string) {
	if song == nil {
		return
	}
	song.URL = transform(song.URL)
	song.Cover = transform(song.Cover)
	song.Link = transform(song.Link)
	for key, value := range song.Extra {
		song.Extra[key] = transform(value)
	}
}

const proxyPath = "/api/proxy"
const legacyProxyPath = "/proxy"
