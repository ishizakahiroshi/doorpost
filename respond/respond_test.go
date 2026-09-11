package respond_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ishizakahiroshi/doorpost/respond"
)

// 使う側が自分の語を足せること。共通の語と同じ扱いで通る。
const tooManyItems respond.Code = "too_many_items"

func TestError(t *testing.T) {
	cases := map[string]struct {
		status int
		code   respond.Code
	}{
		"本文が読めない":     {http.StatusBadRequest, respond.InvalidRequest},
		"メソッドが違う":     {http.StatusMethodNotAllowed, respond.InvalidRequest},
		"本人だと確かめられない": {http.StatusUnauthorized, respond.InvalidKey},
		"使う側が足した語":    {http.StatusRequestEntityTooLarge, tooManyItems},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respond.Error(w, c.status, c.code)

			if w.Code != c.status {
				t.Errorf("status=%d, %d を期待", w.Code, c.status)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type=%q", ct)
			}

			var body respond.ErrorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("応答が JSON として読めない: %v (%s)", err, w.Body.String())
			}
			if body.OK {
				t.Error("エラー応答の ok が true")
			}
			if body.Error != c.code {
				t.Errorf("error=%q, %q を期待", body.Error, c.code)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})

	if w.Code != http.StatusOK {
		t.Errorf("status=%d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答が JSON として読めない: %v", err)
	}
	if !body["ok"] {
		t.Errorf("body=%v", body)
	}
}

// 語はそのまま文字列として出る。呼び出し側はこの語で分岐する。
func TestCodeMarshalsAsPlainString(t *testing.T) {
	b, err := json.Marshal(respond.ErrorBody{OK: false, Error: respond.InvalidKey})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ok":false,"error":"invalid_key"}` {
		t.Fatalf("body=%s", b)
	}
}
