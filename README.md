<a href="https://jgltechnologies.com/discord">
<img src="https://discord.com/api/guilds/844418702430175272/embed.png">
</a>

# gin-rate-limit

gin-rate-limit is a rate limiter for the <a href="https://github.com/gin-gonic/gin">gin framework</a>. By default, it
can only store rate limit info in memory and with redis. If you want to store it somewhere else you can make your own
store or use third party stores. The library is relatively new so there are no third party stores yet. 
Contributions would be appreciated.

Install

 ```shell
 go get github.com/JGLTechnologies/gin-rate-limit
```

<br>

Redis Example

```go
package main

import (
	"github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"time"
)

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func errorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

func main() {
	server := gin.Default()
	// This makes it so each ip can only make 5 requests per second
	store := ratelimit.RedisStore(&ratelimit.RedisOptions{
		RedisClient: redis.NewClient(&redis.Options{
			Addr: "localhost:7680",
		}),
		Rate:  time.Second,
		Limit: 5,
	})
	mw := ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: errorHandler,
		KeyFunc: keyFunc,
    })
	server.GET("/", mw, func(c *gin.Context) {
		c.String(200, "Hello World")
	})
	server.Run(":8080")
}
```

<br>

Basic Setup

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/JGLTechnologies/gin-rate-limit"
	"time"
)

func keyFunc(c *gin.Context) string {
	return c.ClientIP()
}

func errorHandler(c *gin.Context, info ratelimit.Info) {
	c.String(429, "Too many requests. Try again in "+time.Until(info.ResetTime).String())
}

func main() {
	server := gin.Default()
	// This makes it so each ip can only make 5 requests per second
	store := ratelimit.InMemoryStore(&ratelimit.InMemoryOptions{
		Rate:  time.Second,
		Limit: 5,
	})
	mw := ratelimit.RateLimiter(store, &ratelimit.Options{
		ErrorHandler: errorHandler,
		KeyFunc: keyFunc,
	})
	server.GET("/", mw, func(c *gin.Context) {
		c.String(200, "Hello World")
	})
	server.Run(":8080")
}
```

<br>


Response Headers

By default, the package emits the de-facto `X-Rate-Limit-*` headers on every
response (set by `BeforeResponse`) and on rate-limited responses (set by the
default `ErrorHandler`):

| Header | Set on | Description |
|--------|--------|-------------|
| `X-Rate-Limit-Limit` | every response | Total request limit per window |
| `X-Rate-Limit-Remaining` | every response | Remaining requests in window |
| `X-Rate-Limit-Reset` | every response | Unix timestamp when window resets |

If you prefer the standard `Retry-After` header (RFC 9110 §10.2.3) instead,
opt in by passing both `RFCErrorHandler` and `RFCBeforeResponse`. The two
go together — using only one would mix conventions on the same response.

```go
mw := ratelimit.RateLimiter(store, &ratelimit.Options{
    ErrorHandler:   ratelimit.RFCErrorHandler,
    BeforeResponse: ratelimit.RFCBeforeResponse,
})
```

This emits only `Retry-After: <seconds>` on 429 responses and no rate-limit
headers on successful responses.

If you want a custom 429 body (for example, JSON to match your API
conventions) while still using the standard `Retry-After` header, write
your own `ErrorHandler` and use the public `RetryAfterSeconds` helper:

```go
func errorHandler(c *gin.Context, info ratelimit.Info) {
    c.Header("Retry-After", strconv.FormatInt(ratelimit.RetryAfterSeconds(info.ResetTime), 10))
    c.JSON(429, gin.H{"error": "too many requests"})
}

mw := ratelimit.RateLimiter(store, &ratelimit.Options{
    ErrorHandler:   errorHandler,
    BeforeResponse: ratelimit.RFCBeforeResponse, // suppress X-Rate-Limit-* on success too
})
```

`RetryAfterSeconds` returns `ceil(time.Until(resetTime).Seconds())` floored
at 1, so the client never retries before the window has elapsed.

<br>


Custom Store Example

```go
package main

import (
	"github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
)

type CustomStore struct {
}

// Your store must have a method called Limit that takes a key, *gin.Context and returns ratelimit.Info
func (s *CustomStore) Limit(key string, c *gin.Context) Info {
	if UserWentOverLimit {
		return Info{
			Limit:         100,
			RateLimited:   true,
			ResetTime:     reset,
			RemainingHits: 0,
		}
	}
	return Info{
		Limit:         100,
		RateLimited:   false,
		ResetTime:     reset,
		RemainingHits: remaining,
	}
}
```