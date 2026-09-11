// テストは外部パッケージから書く。使う側と同じ見え方でしか触れないようにするため。
// Base を埋め込んだ別パッケージの構造体が Settings を満たすことも、これで一緒に押さえる。
package config_test

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ishizakahiroshi/doorpost/config"
)

// フィクスチャはすべて合成データ。実在のホスト名・鍵・IP は書かない。
// IP は RFC 5737 のドキュメント用アドレスと 127.0.0.1 だけを使う。

// key は seed から最短長を満たす鍵を作る。数え間違いで検証が空振りするのを防ぐ。
func key(seed string) string {
	if len(seed) >= config.MinKeyLen {
		return seed
	}
	return seed + strings.Repeat("-", config.MinKeyLen+8-len(seed))
}

// mk は TOML の雛形へ鍵を差し込む。
func mk(format string, seeds ...string) string {
	args := make([]any, len(seeds))
	for i, s := range seeds {
		args[i] = key(s)
	}
	return fmt.Sprintf(format, args...)
}

// service は使う側の設定。共通部分を埋め込み、自分の項目を外側へ足す。
type service struct {
	config.Base
	UpstreamURL string `toml:"upstream_url"`

	validateErr error
}

func (s *service) Validate() error { return s.validateErr }

const validTOML = `
listen = "127.0.0.1:8080"
upstream_url = "https://upstream.example.com/api/"

[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production", "staging"]
allow_ips = ["203.0.113.10"]

[[apps]]
name = "bravo"
keys = ["%s"]
envs = ["production"]
allow_ips = ["198.51.100.0/24"]
`

func decode(t *testing.T, src string) *service {
	t.Helper()
	var s service
	if err := config.Decode(strings.NewReader(src), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return &s
}

// 共通部分と、使う側が足した項目が同じ 1 ファイルから読めること。
// これが成り立たないと、使う側は設定ファイルを 2 つに割ることになる。
func TestDecodeReadsCommonAndServiceFields(t *testing.T) {
	s := decode(t, mk(validTOML, "alpha", "bravo"))

	if s.Listen != "127.0.0.1:8080" {
		t.Errorf("listen=%q", s.Listen)
	}
	if s.UpstreamURL != "https://upstream.example.com/api/" {
		t.Errorf("upstream_url=%q", s.UpstreamURL)
	}
	if len(s.Apps) != 2 || s.Apps[0].Name != "alpha" || s.Apps[1].Name != "bravo" {
		t.Fatalf("apps=%+v", s.Apps)
	}
}

// 設定にコメントが書けること。TOML を選んだ理由そのものなので、テストで押さえる。
func TestDecodeAllowsComments(t *testing.T) {
	s := decode(t, mk(`
# このサービスの受け口。
listen = "127.0.0.1:8080" # 前段の逆プロキシからだけ届けばよい
upstream_url = "https://upstream.example.com/api/"

[[apps]]
name = "alpha"
# 2026-01-01 に入れ替えた鍵。
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]
`, "alpha"))

	if len(s.Apps) != 1 || s.Apps[0].Name != "alpha" {
		t.Fatalf("apps=%+v", s.Apps)
	}
}

// 雛形がそのまま通ること。共通部分の形を変えたときに雛形だけ古いまま残るのを防ぐ。
func TestLoadExampleTemplate(t *testing.T) {
	var s config.Base
	if err := config.Load(filepath.Join("common.example.toml"), &s); err != nil {
		t.Fatalf("common.example.toml が通らない: %v", err)
	}
	if len(s.Apps) == 0 {
		t.Fatal("雛形に apps が無い")
	}
}

func TestLoadMissingFile(t *testing.T) {
	var s config.Base
	if err := config.Load(filepath.Join("testdata", "does-not-exist.toml"), &s); err == nil {
		t.Fatal("無いファイルを読めてしまった")
	}
}

// 検証で止めるものは起動前に全部落とす。穴の空いた状態で動かさない。
func TestDecodeRejects(t *testing.T) {
	cases := map[string]string{
		"TOML として読めない": `listen = "127.0.0.1:8080`,

		"listen が無い": mk(`
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`, "alpha"),

		"listen が空": mk(`
listen = "   "
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`, "alpha"),

		"apps が無い": `
listen = "127.0.0.1:8080"`,

		"name が空": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = ""
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`, "alpha"),

		"name が重複": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.11"]`, "alpha", "bravo"),

		"keys が空": `
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = []
envs = ["production"]
allow_ips = ["203.0.113.10"]`,

		"keys に空の要素": `
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["                                   "]
envs = ["production"]
allow_ips = ["203.0.113.10"]`,

		"key が短い": `
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["too-short"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`,

		"key が他のアプリと重複": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]
[[apps]]
name = "bravo"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.11"]`, "same", "same"),

		"envs が空": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = []
allow_ips = ["203.0.113.10"]`, "alpha"),

		"envs に空の要素": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production", " "]
allow_ips = ["203.0.113.10"]`, "alpha"),

		"allow_ips が空": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = []`, "alpha"),

		"allow_ips が読めない": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["not-an-address"]`, "alpha"),

		"allow_ips の CIDR が読めない": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.0/99"]`, "alpha"),

		"知らないキーがある": mk(`
listen = "127.0.0.1:8080"
lissten = "typo"
[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`, "alpha"),

		"apps の中に知らないキーがある": mk(`
listen = "127.0.0.1:8080"
[[apps]]
name = "alpha"
key = ["typo"]
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]`, "alpha"),
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var s service
			if err := config.Decode(strings.NewReader(src), &s); err == nil {
				t.Fatal("エラーになるはず")
			}
		})
	}
}

// 鍵の入れ替え。2 本目でも同じアプリが引けること、3 本目は受けないこと。
func TestKeysAtMostTwo(t *testing.T) {
	newKey, oldKey, extra := key("the-new-key"), key("the-old-key"), key("a-third-key")

	base := `
listen = "127.0.0.1:8080"

[[apps]]
name = "alpha"
keys = [%s]
envs = ["production"]
allow_ips = ["203.0.113.10"]
`
	quote := func(keys ...string) string {
		return `"` + strings.Join(keys, `","`) + `"`
	}

	var two config.Base
	if err := config.Decode(strings.NewReader(fmt.Sprintf(base, quote(newKey, oldKey))), &two); err != nil {
		t.Fatalf("2 本を受けない: %v", err)
	}
	for _, k := range []string{newKey, oldKey} {
		if a := two.AppByKey(k); a == nil || a.Name != "alpha" {
			t.Fatalf("入れ替え中の鍵で引けない: %v", a)
		}
	}

	var three config.Base
	err := config.Decode(strings.NewReader(fmt.Sprintf(base, quote(newKey, oldKey, extra))), &three)
	if err == nil {
		t.Fatal("3 本目を受けてしまった")
	}
	for _, k := range []string{newKey, oldKey, extra} {
		if strings.Contains(err.Error(), k) {
			t.Fatalf("エラー文に鍵が出ている: %v", err)
		}
	}
}

// 鍵の値そのものをエラー文へ出さない（起動失敗のログは人の目にも会話ログにも残る）。
func TestErrorsNeverContainKeyValues(t *testing.T) {
	secret := key("a-secret-looking-value")
	src := fmt.Sprintf(`
listen = "127.0.0.1:8080"

[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.10"]

[[apps]]
name = "bravo"
keys = ["%s"]
envs = ["production"]
allow_ips = ["203.0.113.11"]
`, secret, secret)

	var s config.Base
	err := config.Decode(strings.NewReader(src), &s)
	if err == nil {
		t.Fatal("エラーになるはず")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("エラー文に鍵が出ている: %v", err)
	}
}

// 使う側の検証は共通部分が通ったあとに呼ばれ、失敗すれば設定全体が通らない。
func TestValidatorRuns(t *testing.T) {
	src := mk(validTOML, "alpha", "bravo")

	s := &service{validateErr: fmt.Errorf("upstream_url が URL として読めない")}
	err := config.Decode(strings.NewReader(src), s)
	if err == nil || !strings.Contains(err.Error(), "upstream_url") {
		t.Fatalf("使う側の検証が効いていない: %v", err)
	}

	ok := &service{}
	if err := config.Decode(strings.NewReader(src), ok); err != nil {
		t.Fatalf("通るはずの設定が落ちた: %v", err)
	}
}

// 共通部分が落ちた設定で、使う側の検証を呼ばない。
// 呼ぶと、まだ検証していない値を使う側が触ることになる。
func TestValidatorNotCalledWhenCommonPartFails(t *testing.T) {
	called := false
	s := &recorder{onValidate: func() { called = true }}
	if err := config.Decode(strings.NewReader(`listen = "127.0.0.1:8080"`), s); err == nil {
		t.Fatal("apps が無いのでエラーになるはず")
	}
	if called {
		t.Fatal("共通部分が落ちたのに使う側の検証が呼ばれている")
	}
}

type recorder struct {
	config.Base
	onValidate func()
}

func (r *recorder) Validate() error {
	r.onValidate()
	return nil
}

func TestAppByKey(t *testing.T) {
	s := decode(t, mk(validTOML, "alpha", "bravo"))

	alpha := key("alpha")
	if a := s.AppByKey(alpha); a == nil || a.Name != "alpha" {
		t.Fatalf("alpha を引けない: %v", a)
	}
	for _, k := range []string{"", alpha[:len(alpha)-1], alpha + "x", strings.ToUpper(alpha)} {
		if a := s.AppByKey(k); a != nil {
			t.Fatalf("%q で %s が引けてしまった", k, a.Name)
		}
	}
}

func TestAllowsEnv(t *testing.T) {
	s := decode(t, mk(validTOML, "alpha", "bravo"))
	alpha := s.AppByKey(key("alpha"))

	if !alpha.AllowsEnv("production") || !alpha.AllowsEnv("staging") {
		t.Error("登録済みの env を弾いている")
	}
	if alpha.AllowsEnv("development") || alpha.AllowsEnv("") {
		t.Error("登録していない env を通している")
	}
}

func TestAllowsIP(t *testing.T) {
	s := decode(t, mk(validTOML, "alpha", "bravo"))

	alpha := s.AppByKey(key("alpha")) // 単独の IP
	if !alpha.AllowsIP(net.ParseIP("203.0.113.10")) {
		t.Error("許可 IP を弾いている")
	}
	if alpha.AllowsIP(net.ParseIP("203.0.113.11")) || alpha.AllowsIP(nil) {
		t.Error("許可外の IP を通している")
	}

	bravo := s.AppByKey(key("bravo")) // CIDR
	if !bravo.AllowsIP(net.ParseIP("198.51.100.77")) {
		t.Error("CIDR 内の IP を弾いている")
	}
	if bravo.AllowsIP(net.ParseIP("192.0.2.1")) {
		t.Error("CIDR 外の IP を通している")
	}
}

// IPv6 も単独表記と CIDR の両方を受ける。
func TestAllowsIPv6(t *testing.T) {
	var s config.Base
	if err := config.Decode(strings.NewReader(mk(`
listen = "[::1]:8080"

[[apps]]
name = "alpha"
keys = ["%s"]
envs = ["production"]
allow_ips = ["2001:db8::1", "2001:db8:1::/48"]
`, "alpha")), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}

	alpha := s.AppByKey(key("alpha"))
	for _, ok := range []string{"2001:db8::1", "2001:db8:1::ff"} {
		if !alpha.AllowsIP(net.ParseIP(ok)) {
			t.Errorf("%s を弾いている", ok)
		}
	}
	if alpha.AllowsIP(net.ParseIP("2001:db8:2::1")) {
		t.Error("範囲外の IPv6 を通している")
	}
}
