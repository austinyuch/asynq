// Copyright 2026 The Asynq Authors. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package asynq

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseRedisURIDoesNotExposeCredentials(t *testing.T) {
	for _, scheme := range []string{"redis", "rediss", "redis-socket", "redis-sentinel"} {
		for _, suffix := range []string{"@localhost:invalid", "%zz@localhost:6379", "@localhost:6379/\n", "@localhost:credential-canary"} {
			t.Run(scheme+"/"+suffix, func(t *testing.T) {
				uri := scheme + "://service:credential-canary" + suffix
				opt, err := ParseRedisURI(uri)
				if err == nil || opt != nil {
					t.Fatal("malformed credential-bearing URI must fail without connection options")
				}
				for _, diagnostic := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
					if strings.Contains(diagnostic, "credential-canary") || strings.Contains(diagnostic, "service:") {
						t.Fatal("parse diagnostic exposes connection credentials")
					}
				}
				if errors.Unwrap(err) != nil {
					t.Fatal("parse diagnostic must not retain a credential-bearing error chain")
				}
			})
		}
	}
}
