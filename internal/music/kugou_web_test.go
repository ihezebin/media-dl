package music

import (
	"encoding/json"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/internal/httpx"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestKugouWebParamsPreserveAlbumAndSession(t *testing.T) {
	song := &musicmodel.Song{ID: "hash", AlbumID: "44813429", Extra: map[string]string{"album_audio_id": "314795279"}}
	p := kugouWebParams(song, "kg_mid=device; kg_dfid=fingerprint; t=token; KugooID=42")
	for key, want := range map[string]string{"hash": "hash", "album_id": "44813429", "album_audio_id": "314795279", "mid": "device", "dfid": "fingerprint", "token": "token", "userid": "42"} {
		if p.Get(key) != want {
			t.Fatalf("missing %s", key)
		}
	}
	guest := kugouWebParams(song, "")
	if guest.Get("userid") != "0" || guest.Get("signature") == "" {
		t.Fatal("guest request not signed")
	}
	song.Extra["encode_album_audio_id"] = "encoded"
	p = kugouWebParams(song, "")
	if p.Get("encode_album_audio_id") != "encoded" || p.Has("hash") {
		t.Fatal("encoded identity not preferred")
	}
}

func TestKugouWebResponseTrialAndVerification(t *testing.T) {
	song := &musicmodel.Song{}
	u, err := applyKugouWebResponse(song, []byte(`{"status":1,"data":{"play_backup_url":"https://example.com/audio.mp3","is_free_part":1,"extname":"mp3"}}`))
	if err != nil || u == "" || song.Extra["is_trial"] != "1" {
		t.Fatal("preview must remain identified", err)
	}
	_, err = applyKugouWebResponse(song, []byte(`{"status":1,"data":{"play_url":"https://example.com/full.mp3","is_free_part":0}}`))
	if err != nil || song.Extra["is_trial"] != "" {
		t.Fatal("full track still marked preview")
	}
	_, err = applyKugouWebResponse(song, []byte(`{"status":0,"err_code":30020,"data":{}}`))
	if err == nil || !strings.Contains(err.Error(), "安全验证") {
		t.Fatal("challenge must not be reported as unavailable song")
	}
}

func TestKugouSearchTransParam(t *testing.T) {
	var response kugouSearchResponse
	err := json.Unmarshal([]byte(`{"data":{"lists":[{"FileHash":"hash","trans_param":{"ogg_128_hash":"ogg","ogg_128_filesize":123}}]}}`), &response)
	if err != nil || response.Data.Lists[0].TransParam.Ogg128Hash != "ogg" {
		t.Fatal("lowercase trans_param lost")
	}
}

type kugouTestTransport func(*http.Request) (*http.Response, error)

func (f kugouTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKugouPreviewCannotDownloadAsFullTrack(t *testing.T) {
	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	http.DefaultTransport = kugouTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "wwwapi.kugou.com" {
			t.Fatalf("unexpected upstream request: %s", r.URL.Host)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"status":1,"data":{"play_url":"https://example.com/preview.mp3","is_free_part":1}}`))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	song := &musicmodel.Song{Source: "kugou", ID: "hash", Name: "preview", URL: "https://example.com/preview.mp3", Extra: map[string]string{"is_trial": "1"}}
	_, err = New("").Download(client, song, DownloadOptions{OutputDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "仅可试听") {
		t.Fatalf("expected preview restriction, got %v", err)
	}
}

func TestKugouTrialDurationUsesDeclaredBounds(t *testing.T) {
	song := &musicmodel.Song{Duration: 189}
	_, err := applyKugouWebResponse(song, []byte(`{"status":1,"data":{"play_url":"https://example.com/trial.mp3","is_free_part":1,"trans_param":{"hash_offset":{"start_ms":30000,"end_ms":75500}}}}`))
	if err != nil || song.Extra["trial_duration"] != "45.5" || song.Duration != 189 {
		t.Fatalf("preview bounds or catalog duration incorrect: %+v, %v", song, err)
	}
	_, err = applyKugouWebResponse(song, []byte(`{"status":1,"data":{"play_url":"https://example.com/full.mp3","is_free_part":0}}`))
	if err != nil || song.Extra["trial_duration"] != "" {
		t.Fatal("full track kept preview bounds")
	}
	for _, bounds := range [][2]int64{{0, 0}, {1000, 500}, {-1, 60000}} {
		if kugouTrialDuration(bounds[0], bounds[1]) != "" {
			t.Fatal("invalid preview bounds accepted")
		}
	}
}

func TestKugouMemberPlaybackUsesEncodedIdentity(t *testing.T) {
	song := &musicmodel.Song{ID: "hash", AlbumID: "44813429"}
	calls := 0
	got, err := resolveKugouWebRequests(song, "KuGoo=KugooID=42&t=member-session&a_id=1014", func(params url.Values) ([]byte, error) {
		calls++
		if params.Get("token") != "member-session" || params.Get("userid") != "42" {
			t.Fatal("session not propagated")
		}
		if calls == 1 {
			if params.Get("hash") != "hash" {
				t.Fatal("expected hash lookup")
			}
			return []byte(`{"status":1,"data":{"encode_album_audio_id":"encoded","play_url":"https://example.com/preview.mp3","is_free_part":1}}`), nil
		}
		if params.Get("encode_album_audio_id") != "encoded" || params.Has("hash") {
			t.Fatal("expected encoded playback lookup")
		}
		return []byte(`{"status":1,"data":{"play_url":"https://example.com/full.mp3","is_free_part":0}}`), nil
	})
	if err != nil || got != "https://example.com/full.mp3" || calls != 2 || song.Extra["is_trial"] != "" {
		t.Fatalf("member playback = %q, %v (%d calls)", got, err, calls)
	}
}

func TestKugouPlaybackRefreshPreservesPreviewOnFailure(t *testing.T) {
	song := &musicmodel.Song{ID: "hash"}
	calls := 0
	got, err := resolveKugouWebRequests(song, "", func(params url.Values) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte(`{"status":1,"data":{"encode_album_audio_id":"encoded","play_url":"https://example.com/preview.mp3","is_free_part":1}}`), nil
		}
		return []byte(`{"status":0,"err_code":30020,"data":{}}`), nil
	})
	if err != nil || got != "https://example.com/preview.mp3" || song.Extra["is_trial"] != "1" || !strings.Contains(song.Extra["access_reason"], "安全验证") {
		t.Fatal("preview lost, challenge hidden, or preview promoted to full playback", err)
	}
}

func TestKugouWebMemberIdentityFromCompoundCookie(t *testing.T) {
	song := &musicmodel.Song{ID: "hash"}
	params := kugouWebParams(song, "kg_mid=device; KuGoo=KugooID=42&t=member-token&a_id=1014&NickName=test; kg_dfid=fingerprint")
	if params.Get("userid") != "42" || params.Get("token") != "member-token" {
		t.Fatalf("compound login lost: guest=%t token_present=%t", params.Get("userid") == "0", params.Get("token") != "")
	}
}

func TestKugouCompoundIdentityOverridesStaleFlatCookies(t *testing.T) {
	song := &musicmodel.Song{ID: "hash"}
	params := kugouWebParams(song, "KugooID=99; t=old; KuGoo=KugooID=42&t=new%2Btoken%3D&a_id=1014")
	if params.Get("userid") != "42" || params.Get("token") != "new+token=" {
		t.Fatal("compound identity not used consistently")
	}
	params = kugouWebParams(song, "KugooID=99; t=old; KuGoo=KugooID=0&t=")
	if params.Get("userid") != "0" || params.Get("token") != "" {
		t.Fatal("guest compound cookie mixed with old session")
	}
}
