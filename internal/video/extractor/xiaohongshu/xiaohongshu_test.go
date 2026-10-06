package xiaohongshu

import "testing"

func TestResolveURLEscapedShareLink(t *testing.T) {
	const want = "https://www.xiaohongshu.com/discovery/item/6a8a74cd000000003703e040?source=webshare&xhsshare=pc_web&xsec_token=ABCZ_NL58szboKn4AG64ozNlu4r9tX_CrEQ9Y9LNmlAME=&xsec_source=pc_share"
	for _, input := range []string{
		want,
		`https://www.xiaohongshu.com/discovery/item/6a8a74cd000000003703e040\?source\=webshare\&xhsshare\=pc_web\&xsec_token\=ABCZ_NL58szboKn4AG64ozNlu4r9tX_CrEQ9Y9LNmlAME\=\&xsec_source\=pc_share`,
	} {
		got, err := resolveURL(nil, input)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("resolveURL(%q) = %q, want %q", input, got, want)
		}
	}
}
