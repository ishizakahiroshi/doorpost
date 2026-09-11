# doorpost

The door frame around an HTTP intake: configuration, authentication, error vocabulary,
and one log line per request.

A small Go library for a service that exposes an HTTP endpoint to a handful of known
callers inside a network you control. Every such service has to answer the same four
questions before it can do anything useful. doorpost answers those four and then gets
out of the way — it never touches what your endpoint is actually for.

Standard library only, plus [BurntSushi/toml](https://github.com/BurntSushi/toml) to
read the configuration file.

## What it is

1. **Configuration that is checked before the process serves anything.** TOML, with
   unknown keys rejected. A typo in a key name must not quietly fall back to a default
   — that is how a limit you thought you set turns out not to be set
2. **Authentication by per-app key and source address.** Two keys per app so you can
   rotate without downtime, constant-time comparison, and an allow list that refuses to
   mean "everyone" when it is left empty
3. **One vocabulary for errors.** A short machine-readable word and an HTTP status,
   in one response shape, so a caller writes one branch instead of one per service
4. **One log line per accepted request.** `key=value` fields, values quoted when they
   would otherwise break the line

## What it is not

- Not an HTTP client. It never sends anything anywhere
- Not a router, a handler, or a middleware stack. You write those
- Not a store. It holds no database, no cache, no files, no state between requests
- Not a job runner. No retries, no queues, no parallel fan-out
- Not a web framework, and not an identity provider. There are no users, sessions,
  roles, or permissions here — only "is this caller the app it claims to be?"

If you need a framework, use one. doorpost is for the case where a plain
`net/http` service needs a door frame and nothing else.

## Install

```
go get github.com/ishizakahiroshi/doorpost
```

Requires Go 1.22 or later.

## Use

Embed `config.Base` in your own settings type and add whatever your service needs.
One configuration file, one load, one place that refuses to start when something is
wrong.

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ishizakahiroshi/doorpost/config"
	"github.com/ishizakahiroshi/doorpost/gate"
	"github.com/ishizakahiroshi/doorpost/oplog"
	"github.com/ishizakahiroshi/doorpost/respond"
)

// Config is your service's settings: the common part plus your own fields.
type Config struct {
	config.Base
	UpstreamURL string `toml:"upstream_url"`
}

// Validate runs after the common checks pass. Anything you added, you check here.
func (c *Config) Validate() error {
	if c.UpstreamURL == "" {
		return fmt.Errorf("config: upstream_url is empty")
	}
	return nil
}

func main() {
	var cfg Config
	if err := config.Load("config.toml", &cfg); err != nil {
		log.Fatalf("not starting: %v", err) // a half-checked config never starts
	}

	lg := oplog.New(os.Stdout)
	http.HandleFunc("/v1/work", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()

		app := gate.Authorize(r, &cfg)
		if app == nil {
			lg.Print(oplog.Str("error", respond.InvalidKey))
			respond.Error(w, http.StatusUnauthorized, respond.InvalidKey)
			return
		}

		env := r.URL.Query().Get("env")
		if !app.AllowsEnv(env) {
			lg.Print(oplog.Str("app", app.Name), oplog.Str("error", respond.InvalidRequest))
			respond.Error(w, http.StatusBadRequest, respond.InvalidRequest)
			return
		}

		// ... your service does its actual work here ...

		lg.Print(
			oplog.Str("app", app.Name),
			oplog.Str("env", env),
			oplog.Int("items", 12),
			oplog.Dur("took", time.Since(started)),
		)
		respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	log.Fatal(http.ListenAndServe(cfg.Listen, nil))
}
```

Your own error words sit next to the shared ones, because `respond.Code` is a named
string type:

```go
const TooManyItems respond.Code = "too_many_items"

respond.Error(w, http.StatusRequestEntityTooLarge, TooManyItems)
```

## Configuration

The common part is two keys. `config/common.example.toml` is the annotated template;
your own fields go in the same file, and your `Validate` checks them.

```toml
listen = "127.0.0.1:8080"

[[apps]]
name      = "alpha"
keys      = ["replace-me-with-openssl-rand-base64-32--"]
envs      = ["production", "staging"]
allow_ips = ["127.0.0.1"]
```

TOML rather than JSON for one reason: a file full of keys and address ranges needs
comments. "Which key is this, and who asked for it" belongs next to the value.

## Two keys per app, never three

A key gets replaced sooner or later — it leaks, someone leaves, or a schedule says so.
With one slot, the day you replace it is a day the service is down for that caller.
With two, you add the new key, take your time updating the caller, then delete the old
one. Nothing stops.

Three or more would let a forgotten key stay valid forever, which is the failure the
rotation was supposed to prevent. Two is enough to rotate and small enough to audit.

## One error for a wrong key and a wrong address

`invalid_key` comes back whether the key was wrong or the key was right and the
connection came from an address that is not on the list. Saying which one it was would
tell someone guessing keys when they had guessed correctly.

For the same reason the key lookup compares every configured key with
`crypto/subtle` and never returns early — otherwise how long the answer takes tells
you how many characters matched.

## Source addresses and reverse proxies

`X-Forwarded-For` is ignored. A header anyone can set cannot decide who is allowed in;
trusting it turns the allow list into decoration. Behind a reverse proxy, the address
doorpost sees is the proxy's, so that is what belongs in `allow_ips` — restricting the
original caller is the proxy's job, and this list only answers "did this arrive through
the front door".

## License

Apache License 2.0. See [LICENSE](LICENSE).
