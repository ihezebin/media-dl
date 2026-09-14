package music

import "testing"

func TestIsKugouExtendedURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://m.kugou.com/share/song.html?chain=2wKoD7fG5V2", true},
		{"https://www.kugou.com/mixsong/bzstdddc.html", true},
		{"https://www.kugou.com/mixsong/7283tjfe.html?fromsearch=x", true},
		{"https://www.kugou.com/song/#hash=852CFF4E65CAA9799A61B52498824428", false},
		{"https://music.163.com/#/song?id=123", false},
	}
	for _, tt := range tests {
		if got := isKugouExtendedURL(tt.url); got != tt.want {
			t.Fatalf("isKugouExtendedURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestExtractKugouHashFromHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "dataFromSmarty",
			html: `var dataFromSmarty = [{"hash":"852CFF4E65CAA9799A61B52498824428","song_name":"月儿湾"}],// song`,
			want: "852CFF4E65CAA9799A61B52498824428",
		},
		{
			name: "nested array",
			html: `var other = {"hash":"11111111111111111111111111111111"}; var dataFromSmarty = [{"meta":{"id":1},"hash":"DC76241EE9EB0E082D2FBF5FF14C24B7"}],`,
			want: "DC76241EE9EB0E082D2FBF5FF14C24B7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractKugouHashFromHTML(tt.html)
			if err != nil {
				t.Fatalf("extractKugouHashFromHTML() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("extractKugouHashFromHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}
