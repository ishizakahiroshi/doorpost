// Package oplog は受け付けた呼び出し 1 件につき 1 行を書く。
//
// 1 件 1 行にするのは、あとから数えるため。行を分けると、1 件の呼び出しが何行に
// なるかがその日の処理内容で変わり、grep の結果が件数と一致しなくなる。
//
// 形は key=value を空白で区切ったもの。値に空白や改行が入るときだけ引用符で囲む。
// 囲まずに出すと、値へ空白を仕込むだけで、1 行の中に嘘の項目を足せてしまう。
//
//	log.Print(oplog.Str("app", app.Name), oplog.Int("items", n), oplog.Dur("took", d))
//	app=alpha items=12 took=84ms
//
// 書いてよいのは、あとで数えるもの（どのアプリから・いくつ・成功したか・かかった時間）
// だけ。誰の何を扱ったかは書かない。運用ログは人が読む前提で残り続けるので、
// 1 行に入れた個人の情報は、消す口が無いまま溜まっていく。
package oplog

import (
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

// Logger は 1 行を組み立てて書き出す。
type Logger struct {
	out *log.Logger
}

// New は w へ書く Logger を作る。
//
// 時刻を自分で出さない。常駐サービスはふつうサービス管理の仕組みから起動され、
// その仕組みが 1 行ごとにホストの現地時刻を前置する。ここで別に時刻を足すと 1 行に
// 2 つ並び、片方がずれたときに気づけない（タイムゾーンの設定はプロセスごとに違う）。
// 2 つの時計を合わせるより、1 つに減らすほうが後から狂わない。
//
// サービス管理の外で動かしたときは時刻が出なくなるが、そのときは手元で見ている。
func New(w io.Writer) *Logger {
	return &Logger{out: log.New(w, "", 0)}
}

// Wrap は既にある log.Logger をそのまま使う。
// 前置きや時刻の設定は呼び出し側のものが効くので、[New] の説明を読んで決める。
func Wrap(l *log.Logger) *Logger {
	if l == nil {
		l = log.Default()
	}
	return &Logger{out: l}
}

// Std は下にある log.Logger を返す。起動時や終了時の自由形式の 1 行はこちらで書く。
func (l *Logger) Std() *log.Logger { return l.out }

// Print は項目を 1 行にして書く。項目が 1 つも無ければ何も書かない。
func (l *Logger) Print(fields ...Field) {
	if len(fields) == 0 {
		return
	}
	l.out.Print(Line(fields...))
}

// Line は項目を 1 行の文字列にする。書き出し先を自分で持っている場合に使う。
func Line(fields ...Field) string {
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(f.key)
		b.WriteByte('=')
		b.WriteString(f.value)
	}
	return b.String()
}

// Field は 1 行の中の 1 項目。キーは書く人が決めるもので、外から来た値は入れない。
type Field struct {
	key   string
	value string
}

// Str は文字列の項目。エラーの語のような、文字列を下地にした型もそのまま渡せる。
func Str[T ~string](key string, value T) Field {
	return Field{key: key, value: quote(string(value))}
}

// Int は整数の項目。件数を書くのに使う。
func Int(key string, value int) Field {
	return Field{key: key, value: strconv.Itoa(value)}
}

// Bool は真偽の項目。
func Bool(key string, value bool) Field {
	return Field{key: key, value: strconv.FormatBool(value)}
}

// Dur は時間の項目。ミリ秒より細かい桁は落とす。
// 落とさないと、同じ処理でも毎回違う桁数で出て、行を見比べるときに邪魔になる。
func Dur(key string, value time.Duration) Field {
	return Field{key: key, value: value.Round(time.Millisecond).String()}
}

// quote は 1 行の形を壊しうる値だけを引用符で囲む。
func quote(v string) string {
	if v == "" {
		return `""`
	}
	for _, r := range v {
		if r <= ' ' || r == '"' || !strconv.IsPrint(r) {
			return strconv.Quote(v)
		}
	}
	return v
}
