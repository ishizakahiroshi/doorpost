// Package respond は受け口が返す JSON の形と、エラーの語彙を 1 か所に決める。
//
// 受け口が増えるたびに応答の形が少しずつ違うと、呼び出し側は相手ごとに分岐を書く。
// 形は 1 つにして、変わるのは中身の語だけにする。
//
// ここが持つのは、受け口を持つサービスならどれも同じになる語だけ
// （[InvalidRequest] と [InvalidKey]）。サービス固有の語は使う側が自分で足す。
//
//	const TooManyItems respond.Code = "too_many_items"
//	respond.Error(w, http.StatusRequestEntityTooLarge, TooManyItems)
//
// 語は短く、機械が見るもの。人が読む説明を入れない。中身を足すほど、呼び出し側の
// ログや画面へそのまま出るようになり、いつのまにか外向きの文言になる。
package respond

import (
	"encoding/json"
	"net/http"
)

// Code は応答に出すエラーの語。使う側が自分の語を足せるよう名前付きの文字列にしてある。
type Code string

const (
	// InvalidRequest は呼び出しの形が違う。本文・メソッド・引数のどれかが受けられない。
	InvalidRequest Code = "invalid_request"

	// InvalidKey は名乗ったアプリ本人だと確かめられなかった。
	//
	// 鍵が違うときも、鍵は合っていて接続元が許可外のときも、同じこの語を返す。
	// 区別して返すと、鍵の総当たりに当たり判定を与えることになる。
	InvalidKey Code = "invalid_key"
)

// ErrorBody はエラー応答の本文。成功応答は使う側が自分で決めるが、
// エラーだけはこの形に揃える。呼び出し側が 1 つの分岐で読めるようにするため。
type ErrorBody struct {
	OK    bool `json:"ok"`
	Error Code `json:"error"`
}

// JSON は status と v を JSON で返す。
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// ここで書き込みが失敗するのは相手が切れたときで、こちらに打つ手が無い。
	_ = json.NewEncoder(w).Encode(v)
}

// Error は status と語を JSON で返す。
//
// 同じ語を違う status で返してよい。たとえば [InvalidRequest] は、本文が読めなければ
// 400、メソッドが違えば 405 になる。語は「何が起きたか」、status は「HTTP としてどう
// 扱うか」で、対応は 1 対 1 にならない。
func Error(w http.ResponseWriter, status int, code Code) {
	JSON(w, status, ErrorBody{OK: false, Error: code})
}
