package main

import "testing"

func TestStartupModeDefaultWebExplicitCLI(t *testing.T) {
	for _, c := range []struct {
		args    []string
		cli     bool
		sub     string
		browser bool
		bad     bool
	}{
		{nil, false, "serve", true, false}, {nil, true, "repl", false, false},
		{[]string{"serve"}, false, "serve", false, false}, {[]string{"exec", "task"}, false, "exec", false, false},
		{[]string{"task-worker"}, false, "task-worker", false, false}, {[]string{"repl"}, false, "repl", false, false},
		{[]string{"serve"}, true, "", false, true},
	} {
		sub, browser, err := resolveStartupMode(c.args, c.cli)
		if (err != nil) != c.bad || sub != c.sub || browser != c.browser {
			t.Fatalf("%+v => %s %v %v", c, sub, browser, err)
		}
	}
}
