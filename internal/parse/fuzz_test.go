package parse

import "testing"

// FuzzParseEnv hammers the one hand-rolled parser. Any input may be skipped
// with warnings but must never panic, and a nil error implies a usable tree.
func FuzzParseEnv(f *testing.F) {
	seeds := []string{
		"KEY=value\n",
		"export FOO=\"bar\\nbaz\"\n",
		"A='literal'\n# comment\nB=x # trailing\n",
		"=novalue\nNOEQUALS\nK==double\n",
		"K=\"unterminated\nK2='\n",
		"\\\n\x00\xff\n",
		"K=\"esc\\\\\\\"q\\t\"\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tree, _, err := parseEnv(data)
		if err == nil && tree == nil {
			t.Error("nil tree with nil error")
		}
	})
}
