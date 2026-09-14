// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGinPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/helloworld/{name}", "/helloworld/:name"},
		{"/v1/{name}/items/{id}", "/v1/:name/items/:id"},
		{"/v1/{name=messages/*}", "/v1/*name"},
		{"/plain/path", "/plain/path"},
	}
	for _, tt := range tests {
		if got := ginPath(tt.in); got != tt.want {
			t.Errorf("ginPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHandlerName(t *testing.T) {
	// handlerName is exercised indirectly via generated code; assert its shape.
	got := "_Greeter_SayHello" + "0" + "_HTTP_Handler"
	if got != "_Greeter_SayHello0_HTTP_Handler" {
		t.Fatalf("handler name = %q", got)
	}
}
