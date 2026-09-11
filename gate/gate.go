// Package gate は受け口に来た呼び出しが、名乗っているアプリ本人かを確かめる。
//
// 見るのは 2 つだけ。Authorization ヘッダの鍵と、接続元の IP である。
// どちらも設定（[config]）に書かれたものと突き合わせる。ここで行うのは認証だけで、
// 「その呼び出しが何をしてよいか」は扱わない。それは使う側の業務の話になる。
//
// 通らなかった理由は返さない。「鍵は合っていたが IP で落ちた」と読み取れると、
// 鍵の総当たりに当たり判定を与えることになる。
package gate

import (
	"net"
	"net/http"
	"strings"

	"github.com/ishizakahiroshi/doorpost/config"
)

// BearerPrefix は Authorization ヘッダの前置き。
const BearerPrefix = "Bearer "

// Apps は鍵からアプリを引ける設定。[config.Base] を埋め込んだ型が満たす。
type Apps interface {
	AppByKey(key string) *config.App
}

// Authorize は鍵と接続元 IP を見て、対応するアプリを返す。通らなければ nil。
//
// 呼び出し側は nil を 1 つの応答へ落とす。落ちた理由で応答を変えない。
func Authorize(r *http.Request, apps Apps) *config.App {
	key, ok := BearerKey(r)
	if !ok {
		return nil
	}
	app := apps.AppByKey(key)
	if app == nil {
		return nil
	}
	if !app.AllowsIP(ClientIP(r)) {
		return nil
	}
	return app
}

// BearerKey は Authorization ヘッダから鍵を取り出す。
//
// 鍵をクエリ文字列で受けない。クエリ文字列は逆プロキシのアクセスログにも
// ブラウザの履歴にも残る。ヘッダならそこには出ない。
func BearerKey(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) <= len(BearerPrefix) || !strings.EqualFold(h[:len(BearerPrefix)], BearerPrefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(BearerPrefix):]), true
}

// ClientIP は接続元の IP を返す。読めなければ nil。
//
// X-Forwarded-For は見ない。偽装できるヘッダを許可判定に使うと、許可リストが
// 意味を失う。前段に逆プロキシを置く構成では、ここに見えるのは逆プロキシの IP に
// なるので、allow_ips にはその逆プロキシを書く。呼び出し元そのものの IP で絞るのは
// 前段の仕事である。
//
// 前段の値を信じる作りが要るなら、信頼するプロキシを設定へ足したうえでここを直す。
// いま素通しにすると、直し忘れが穴として残る。
func ClientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}
