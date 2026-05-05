package ratelimit

import (
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"
)

type Info struct {
	Limit         uint
	RateLimited   bool
	ResetTime     time.Time
	RemainingHits uint
}

type Store interface {
	// Limit takes in a key and *gin.Context and should return whether that key is allowed to make another request
	Limit(key string, c *gin.Context) Info
}

type Options struct {
	ErrorHandler func(*gin.Context, Info)
	KeyFunc      func(*gin.Context) string
	// a function that lets you check the rate limiting info and modify the response
	BeforeResponse func(c *gin.Context, info Info)
}

// RetryAfterSeconds returns the number of whole seconds until resetTime,
// rounded up so the client never retries before the window has elapsed.
// Always returns at least 1. Useful when building a custom ErrorHandler
// that emits the standard Retry-After header (RFC 9110).
func RetryAfterSeconds(resetTime time.Time) int64 {
	s := int64(math.Ceil(time.Until(resetTime).Seconds()))
	if s < 1 {
		return 1
	}
	return s
}

// RFCErrorHandler is an alternative ErrorHandler that emits the RFC 9110
// Retry-After header instead of the de-facto X-Rate-Limit-* headers used
// by the default. Pair with RFCBeforeResponse to avoid mixing the two
// header conventions on the same response.
//
//	ratelimit.RateLimiter(store, &ratelimit.Options{
//	    ErrorHandler:   ratelimit.RFCErrorHandler,
//	    BeforeResponse: ratelimit.RFCBeforeResponse,
//	})
func RFCErrorHandler(c *gin.Context, info Info) {
	c.Header("Retry-After", fmt.Sprintf("%d", RetryAfterSeconds(info.ResetTime)))
	c.String(429, "Too many requests")
}

// RFCBeforeResponse is a no-op BeforeResponse callback. Use it together
// with RFCErrorHandler when you want only RFC 9110 Retry-After headers
// on rate-limited responses, with no X-Rate-Limit-* headers on successful
// responses.
func RFCBeforeResponse(c *gin.Context, info Info) {}

// RateLimiter is a function to get gin.HandlerFunc
func RateLimiter(s Store, options *Options) gin.HandlerFunc {
	if options == nil {
		options = &Options{}
	}
	if options.ErrorHandler == nil {
		options.ErrorHandler = func(c *gin.Context, info Info) {
			c.Header("X-Rate-Limit-Limit", fmt.Sprintf("%d", info.Limit))
			c.Header("X-Rate-Limit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
			c.String(429, "Too many requests")
		}
	}
	if options.BeforeResponse == nil {
		options.BeforeResponse = func(c *gin.Context, info Info) {
			c.Header("X-Rate-Limit-Limit", fmt.Sprintf("%d", info.Limit))
			c.Header("X-Rate-Limit-Remaining", fmt.Sprintf("%v", info.RemainingHits))
			c.Header("X-Rate-Limit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
		}
	}
	if options.KeyFunc == nil {
		options.KeyFunc = func(c *gin.Context) string {
			return c.ClientIP() + c.FullPath()
		}
	}
	return func(c *gin.Context) {
		key := options.KeyFunc(c)
		info := s.Limit(key, c)
		options.BeforeResponse(c, info)
		if c.IsAborted() {
			return
		}
		if info.RateLimited {
			options.ErrorHandler(c, info)
			c.Abort()
		} else {
			c.Next()
		}
	}
}
