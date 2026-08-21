package book

import "testing"

func TestNormalizeDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "entities", input: `Tom &amp; Jerry&nbsp;&nbsp;&lt;3`, want: "Tom & Jerry  <3"},
		{name: "line breaks", input: `one<BR>two<br class="x">three`, want: "one\ntwo\nthree"},
		{name: "block boundaries", input: `<p>one</p><div>two<br>three</div><ul><li>four</li><li>five</li></ul>`, want: "one\ntwo\nthree\nfour\nfive"},
		{name: "strips tags and attributes", input: `<strong>bold</strong><a href="javascript:alert(1)" onclick="alert(2)">link</a>`, want: "boldlink"},
		{name: "preserves text spacing", input: "  first  line\r\n\r\nsecond\tline  ", want: "first  line\n\nsecond\tline"},
		{name: "drops script and style content", input: `<p>safe</p><script>alert(1)</script><style>body{display:none}</style><p>after</p>`, want: "safe\nafter"},
		{name: "drops encoded active markup", input: `safe&amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt;after`, want: "safeafter"},
		{name: "converts encoded block markup", input: `one&amp;lt;br&amp;gt;two`, want: "one\ntwo"},
		{name: "drops encoded event attributes", input: `before&amp;lt;img src=x onerror=alert(1)&amp;gt;after`, want: "beforeafter"},
		{name: "malformed markup stays plain", input: `<p>before <b>bold</p> after`, want: "before bold\nafter"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeDescription(test.input); got != test.want {
				t.Fatalf("NormalizeDescription(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
