package oplog_test

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/ishizakahiroshi/doorpost/oplog"
)

// 使う側が自分で定義した語も Str に渡せること。
type code string

func TestLine(t *testing.T) {
	got := oplog.Line(
		oplog.Str("app", "alpha"),
		oplog.Str("env", "production"),
		oplog.Int("items", 12),
		oplog.Int("failed", 0),
		oplog.Bool("truncated", true),
		oplog.Dur("took", 84*time.Millisecond),
	)
	want := "app=alpha env=production items=12 failed=0 truncated=true took=84ms"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestLineAcceptsNamedStringTypes(t *testing.T) {
	if got := oplog.Line(oplog.Str("error", code("invalid_key"))); got != "error=invalid_key" {
		t.Fatalf("got %s", got)
	}
}

// 値に空白や改行を仕込んでも、1 行の中に嘘の項目を足せないこと。
func TestLineQuotesValuesThatWouldForgeFields(t *testing.T) {
	cases := map[string]string{
		"alpha":           "app=alpha",
		"":                `app=""`,
		"two words":       `app="two words"`,
		"alpha failed=0":  `app="alpha failed=0"`,
		"alpha\nenv=prod": `app="alpha\nenv=prod"`,
		"alpha\ttab":      `app="alpha\ttab"`,
		"alpha\rcarriage": `app="alpha\rcarriage"`,
		`alpha"quote`:     `app="alpha\"quote"`,
		"日本語はそのまま":        "app=日本語はそのまま",
	}
	for in, want := range cases {
		t.Run(want, func(t *testing.T) {
			got := oplog.Line(oplog.Str("app", in))
			if got != want {
				t.Fatalf("got  %s\nwant %s", got, want)
			}
			if strings.Count(got, "\n") != 0 {
				t.Fatalf("1 行に収まっていない: %q", got)
			}
		})
	}
}

// 時刻を自分で出さない。出すとサービス管理の仕組みが付ける時刻と 2 つ並ぶ。
func TestNewWritesNoTimestamp(t *testing.T) {
	var buf bytes.Buffer
	oplog.New(&buf).Print(oplog.Str("app", "alpha"))

	if got := buf.String(); got != "app=alpha\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPrintWithNoFieldsWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	oplog.New(&buf).Print()

	if buf.Len() != 0 {
		t.Fatalf("空の行を書いている: %q", buf.String())
	}
}

// 既にある log.Logger をそのまま使えること。前置きは呼び出し側のものが効く。
func TestWrap(t *testing.T) {
	var buf bytes.Buffer
	lg := oplog.Wrap(log.New(&buf, "prefix: ", 0))
	lg.Print(oplog.Str("app", "alpha"))

	if got := buf.String(); got != "prefix: app=alpha\n" {
		t.Fatalf("got %q", got)
	}
	if lg.Std() == nil {
		t.Fatal("Std が nil")
	}
}

func TestWrapNil(t *testing.T) {
	if oplog.Wrap(nil).Std() == nil {
		t.Fatal("nil を渡すと使えない Logger が返る")
	}
}

func TestDurRoundsToMilliseconds(t *testing.T) {
	cases := map[time.Duration]string{
		84 * time.Millisecond:              "took=84ms",
		84_499 * time.Microsecond:          "took=84ms",
		84_500 * time.Microsecond:          "took=85ms",
		2*time.Second + 5*time.Microsecond: "took=2s",
	}
	for d, want := range cases {
		if got := oplog.Line(oplog.Dur("took", d)); got != want {
			t.Errorf("%v: got %s, want %s", d, got, want)
		}
	}
}
