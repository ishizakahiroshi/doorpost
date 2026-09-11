// Package config は受け口の設定を読み、起動前に検証する。
//
// 持つのは、受け口を持つサービスならどれも同じになる部分だけ。待ち受けアドレスと、
// 呼び出し元アプリごとの鍵・環境名・許可 IP の 5 項目である。サービス固有の項目は
// ここへ足さない。使う側が [Base] を自分の構造体へ埋め込み、その外側へ書く。
//
//	type Config struct {
//		config.Base
//		UpstreamURL string `toml:"upstream_url"`
//	}
//
// 検証は起動前に全部通す。穴の空いた設定のまま動き出すより、起動しないほうがよい。
// 使う側が足した項目も同じ扱いにできるよう、[Validator] を実装すれば共通部分の検証の
// あとで呼ばれる。
//
// 設定ファイルは TOML。JSON ではなくこちらなのは、鍵と許可 IP が並ぶファイルに
// 「それが何の鍵か」を書き残せないことの害が、外部依存 1 本より大きいため。
package config

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// MaxKeysPerApp は 1 つのアプリが持てる鍵の本数。
	//
	// 鍵は必ずいつか替える（漏れたとき・担当が替わったとき・定期的に）。1 本しか
	// 持てないと、入れ替えの日にサービスが止まる。2 本あれば「新しい鍵を足す →
	// 呼ぶ側を落ち着いて書き換える → 古い鍵を消す」の順で無停止に替えられる。
	// 3 本以上を許さないのは、消し忘れた古い鍵がいつまでも生き続けるため。
	MaxKeysPerApp = 2

	// MinKeyLen は鍵の最短の長さ。短い鍵は総当たりで破られる。
	// `openssl rand -base64 32` は 44 文字になるので、これを下回ることはない。
	MinKeyLen = 32
)

// App は呼び出し元 1 つぶんの設定。
//
// Name はリクエスト本文ではなく鍵から決まる。本文に書かせると、どのアプリでも
// 他のアプリの名前を騙れてしまう。
type App struct {
	Name string `toml:"name"`

	// Keys は有効な鍵。[MaxKeysPerApp] 本まで持てる。
	Keys []string `toml:"keys"`

	// Envs はこのアプリが名乗れる環境名。ここに無い値で呼ばれたら弾く。
	// 中身は使う側が決める（"prod" でも "staging" でも、日本語でもよい）。
	// 共通部分が見るのは「空でないこと」と「空の要素が無いこと」だけ。
	Envs []string `toml:"envs"`

	// AllowIPs は呼び出し元として許す IP。単独の IP と CIDR のどちらも書ける。
	AllowIPs []string `toml:"allow_ips"`

	// allowNets は AllowIPs を解析したもの。単独の IP は /32・/128 として持つ。
	allowNets []*net.IPNet
}

// AllowsEnv は env が登録済みかを返す。
func (a *App) AllowsEnv(env string) bool {
	for _, e := range a.Envs {
		if e == env {
			return true
		}
	}
	return false
}

// AllowsIP は呼び出し元 IP が許可範囲かを返す。
//
// 許可リストが空の設定は検証で弾いてあるので、ここで「空なら全許可」にはしない。
// 伏せる対象を列挙する方式（denylist）は、載っていないものが素通りする。
func (a *App) AllowsIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range a.allowNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Base は受け口を持つサービスに共通の設定。使う側の構造体へ埋め込んで使う。
type Base struct {
	// Listen は待ち受けアドレス。前段に逆プロキシを置くなら 127.0.0.1 に閉じる。
	Listen string `toml:"listen"`

	// Apps は呼び出し元アプリ。1 つも無い設定は起動時に弾く。
	Apps []App `toml:"apps"`
}

// base は [Settings] を満たすための実装。
func (b *Base) base() *Base { return b }

// AppByKey は鍵に対応するアプリを返す。無ければ nil。
//
// 一致したところで抜けず、必ず全件を比較する。早く抜けると、鍵の何文字目まで
// 合っていたかが所要時間に出る。比較そのものも subtle.ConstantTimeCompare を使う。
func (b *Base) AppByKey(key string) *App {
	var found *App
	for i := range b.Apps {
		a := &b.Apps[i]
		for _, k := range a.Keys {
			if subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
				found = a
			}
		}
	}
	return found
}

// Settings は [Load] と [Decode] が受け取れる設定。
//
// メソッドが非公開なのは、[Base] を埋め込んでいない型をここへ渡せないようにするため。
// 共通部分の検証を素通りする経路を作らない。
type Settings interface {
	base() *Base
}

// Validator は使う側が自分で足した項目を検証するために実装する。
// 共通部分の検証がすべて通ったあとで 1 回だけ呼ばれる。
type Validator interface {
	Validate() error
}

// Load は path の設定ファイルを読んで検証し、settings へ書き込む。
func Load(path string, settings Settings) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := Decode(f, settings); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Decode は r から TOML を読んで検証し、settings へ書き込む。
//
// 知らないキーはエラーにする。綴り間違いが黙って既定値になると、設定したつもりの
// 制限が効いていない状態で動き出す。TOML の読み手には encoding/json の
// DisallowUnknownFields に当たる設定が無いので、読めなかったキーを自分で見る。
func Decode(r io.Reader, settings Settings) error {
	md, err := toml.NewDecoder(r).Decode(settings)
	if err != nil {
		return fmt.Errorf("config: parse: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		// 出すのはキーの名前だけ。値は出さない（鍵が混じりうるため）。
		names := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			names = append(names, k.String())
		}
		return fmt.Errorf("config: unknown key(s): %s", strings.Join(names, ", "))
	}
	if err := settings.base().validate(); err != nil {
		return err
	}
	if v, ok := settings.(Validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (b *Base) validate() error {
	if strings.TrimSpace(b.Listen) == "" {
		return fmt.Errorf("config: listen is empty")
	}
	if len(b.Apps) == 0 {
		return fmt.Errorf("config: apps is empty; no caller could reach this service")
	}

	seenKey := map[string]bool{}
	seenName := map[string]bool{}
	for i := range b.Apps {
		a := &b.Apps[i]
		if strings.TrimSpace(a.Name) == "" {
			return fmt.Errorf("config: apps[%d].name is empty", i)
		}
		if seenName[a.Name] {
			return fmt.Errorf("config: apps[%d].name is a duplicate: %s", i, a.Name)
		}
		seenName[a.Name] = true

		if len(a.Keys) == 0 {
			return fmt.Errorf("config: apps[%d](%s).keys is empty", i, a.Name)
		}
		if len(a.Keys) > MaxKeysPerApp {
			return fmt.Errorf("config: apps[%d](%s).keys has %d entries; at most %d", i, a.Name, len(a.Keys), MaxKeysPerApp)
		}
		for j, k := range a.Keys {
			if strings.TrimSpace(k) == "" {
				return fmt.Errorf("config: apps[%d](%s).keys[%d] is empty", i, a.Name, j)
			}
			if len(k) < MinKeyLen {
				// 値そのものはエラー文に出さない。起動失敗のログは人の目に残る。
				return fmt.Errorf("config: apps[%d](%s).keys[%d] is shorter than %d characters", i, a.Name, j, MinKeyLen)
			}
			if seenKey[k] {
				// 鍵が重複すると、どのアプリとして扱うかが決まらなくなる。
				return fmt.Errorf("config: apps[%d](%s).keys[%d] is a duplicate of another key", i, a.Name, j)
			}
			seenKey[k] = true
		}

		if len(a.Envs) == 0 {
			return fmt.Errorf("config: apps[%d](%s).envs is empty", i, a.Name)
		}
		for _, e := range a.Envs {
			if strings.TrimSpace(e) == "" {
				return fmt.Errorf("config: apps[%d](%s).envs has an empty entry", i, a.Name)
			}
		}

		if len(a.AllowIPs) == 0 {
			// 空を「全許可」と読ませない。書き忘れが穴になるのを防ぐ。
			return fmt.Errorf("config: apps[%d](%s).allow_ips is empty; an empty list is not \"allow everyone\"", i, a.Name)
		}
		a.allowNets = a.allowNets[:0]
		for _, s := range a.AllowIPs {
			n, err := parseIPOrCIDR(s)
			if err != nil {
				return fmt.Errorf("config: apps[%d](%s).allow_ips: %q is neither an IP address nor a CIDR block", i, a.Name, s)
			}
			a.allowNets = append(a.allowNets, n)
		}
	}
	return nil
}

// parseIPOrCIDR は "203.0.113.10" と "203.0.113.0/24" の両方を受ける。
func parseIPOrCIDR(s string) (*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		return n, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, fmt.Errorf("not an IP address")
	}
	bits := 32
	if ip.To4() == nil {
		bits = 128
	} else {
		ip = ip.To4()
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, nil
}
