package gate_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ishizakahiroshi/doorpost/config"
	"github.com/ishizakahiroshi/doorpost/gate"
)

// フィクスチャは合成データ。IP は RFC 5737 のドキュメント用アドレスと 127.0.0.1 だけ。
const (
	keyAlpha = "key-for-alpha---------------------------"
	keyBravo = "key-for-bravo---------------------------"
)

func load(t *testing.T) *config.Base {
	t.Helper()
	var c config.Base
	src := `
listen = "127.0.0.1:8080"

[[apps]]
name = "alpha"
keys = ["` + keyAlpha + `"]
envs = ["production"]
allow_ips = ["127.0.0.1"]

[[apps]]
name = "bravo"
keys = ["` + keyBravo + `"]
envs = ["production"]
allow_ips = ["203.0.113.0/24"]
`
	if err := config.Decode(strings.NewReader(src), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return &c
}

func request(auth, remoteAddr string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/anything", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	r.RemoteAddr = remoteAddr
	return r
}

func TestAuthorizeAccepts(t *testing.T) {
	apps := load(t)

	cases := map[string]struct {
		auth, addr, want string
	}{
		"鍵と IP が合っている":      {"Bearer " + keyAlpha, "127.0.0.1:51000", "alpha"},
		"前置きの大小は問わない":       {"bearer " + keyAlpha, "127.0.0.1:51000", "alpha"},
		"鍵の前後の空白は落とす":       {"Bearer  " + keyAlpha + " ", "127.0.0.1:51000", "alpha"},
		"CIDR の中から来ている":     {"Bearer " + keyBravo, "203.0.113.77:51000", "bravo"},
		"ポートの無い RemoteAddr": {"Bearer " + keyAlpha, "127.0.0.1", "alpha"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			app := gate.Authorize(request(c.auth, c.addr), apps)
			if app == nil || app.Name != c.want {
				t.Fatalf("app=%v, %s を期待", app, c.want)
			}
		})
	}
}

func TestAuthorizeRejects(t *testing.T) {
	apps := load(t)

	cases := map[string]struct{ auth, addr string }{
		"ヘッダが無い":           {"", "127.0.0.1:51000"},
		"前置きが違う":           {"Token " + keyAlpha, "127.0.0.1:51000"},
		"鍵が空":              {"Bearer ", "127.0.0.1:51000"},
		"知らない鍵":            {"Bearer " + strings.Repeat("x", 40), "127.0.0.1:51000"},
		"鍵が 1 文字足りない":      {"Bearer " + keyAlpha[:len(keyAlpha)-1], "127.0.0.1:51000"},
		"大文字にした鍵":          {"Bearer " + strings.ToUpper(keyAlpha), "127.0.0.1:51000"},
		"鍵は合っているが IP が違う":  {"Bearer " + keyAlpha, "203.0.113.10:51000"},
		"CIDR の外から来ている":    {"Bearer " + keyBravo, "198.51.100.1:51000"},
		"RemoteAddr が読めない": {"Bearer " + keyAlpha, "not-an-address"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if app := gate.Authorize(request(c.auth, c.addr), apps); app != nil {
				t.Fatalf("通してはいけない呼び出しで %s が返った", app.Name)
			}
		})
	}
}

// 偽装できるヘッダを許可判定に使わない。X-Forwarded-For を足しても結果が変わらないこと。
func TestAuthorizeIgnoresForwardedFor(t *testing.T) {
	apps := load(t)

	r := request("Bearer "+keyAlpha, "203.0.113.10:51000") // alpha に許していない接続元
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	if app := gate.Authorize(r, apps); app != nil {
		t.Fatal("X-Forwarded-For を信じて通してしまった")
	}

	r = request("Bearer "+keyAlpha, "127.0.0.1:51000") // 許している接続元
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	if app := gate.Authorize(r, apps); app == nil {
		t.Fatal("X-Forwarded-For を見て弾いてしまった")
	}
}

func TestClientIP(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:51000":     "127.0.0.1",
		"203.0.113.10:51000":  "203.0.113.10",
		"[2001:db8::1]:51000": "2001:db8::1",
		"198.51.100.1":        "198.51.100.1",
		"not-an-address":      "",
		"":                    "",
	}
	for addr, want := range cases {
		t.Run(addr, func(t *testing.T) {
			got := gate.ClientIP(request("", addr))
			if want == "" {
				if got != nil {
					t.Fatalf("got=%v, nil を期待", got)
				}
				return
			}
			if got == nil || got.String() != want {
				t.Fatalf("got=%v, %s を期待", got, want)
			}
		})
	}
}
