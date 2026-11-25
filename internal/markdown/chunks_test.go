package markdown

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

type chunk struct {
	line int
	text string
}

func TestReadChunksSections(t *testing.T) {
	// The expected sections also define the full input after the initial BOM.
	want := []chunk{
		{1, "intro\r\n\r\n"},
		{3, "  # First\r\n" + "####### no heading\n#hashtag\n    # indented code\n" +
			"~~~~\n# inside\n~~~\n# still inside\n~~~~ trailing\n# still inside\n~~~~\n"},
		{14, "###\nRegular text\n"},
		{16, "# Last"},
	}
	var input strings.Builder
	input.WriteRune('\uFEFF')
	for _, c := range want {
		input.WriteString(c.text)
	}
	var got []chunk
	err := ReadChunks(strings.NewReader(input.String()), func(line int, text string) error {
		got = append(got, chunk{line, text})
		return nil
	})
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ReadChunks() = %#v, %v; want %#v", got, err, want)
	}
}

func TestReadChunksBacktickFences(t *testing.T) {
	const input = "~~~go\n# in tilde\n~~~\n"
	const ticks = "```"
	// A backtick in a backtick fence's info string invalidates the opener.
	first := ticks + "a" + ticks + "b\n"
	second := "# Actual heading\n" + ticks + "go\n# code\n" + ticks + "\n" + input
	last := "# Another heading\n"
	var got []string
	err := ReadChunks(strings.NewReader(first+second+last), func(_ int, text string) error {
		got = append(got, text)
		return nil
	})
	if err != nil || !slices.Equal(got, []string{first, second, last}) {
		t.Fatalf("ReadChunks() = %q, %v", got, err)
	}
}

func TestReadChunksLongUTF8Line(t *testing.T) {
	const limit = protocol.MaxMemoryTextBytes
	long := strings.Repeat("a", limit-1) + "🙂" + strings.Repeat("é", limit/2) + "\r\n"
	input := "preamble\n" + long + "# Next\n"
	var texts []string
	var lines []int
	err := ReadChunks(strings.NewReader(input), func(line int, text string) error {
		if len(text) > limit || !utf8.ValidString(text) || strings.TrimSpace(text) == "" {
			t.Fatalf("invalid chunk of %d bytes at line %d", len(text), line)
		}
		lines = append(lines, line)
		texts = append(texts, text)
		return nil
	})
	if err != nil || strings.Join(texts, "") != input || !slices.Equal(lines, []int{1, 2, 2, 2, 3}) {
		t.Fatalf("ReadChunks() emitted %d chunks at %v, error %v", len(texts), lines, err)
	}
}

func TestReadChunksErrorsAndBlankInput(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"\ufeff \r\n\t \n", ""},
		{"line one\ninvalid\xff", "line 2: invalid UTF-8"},
		{"line one\nzero\x00byte", "line 2: unexpected NUL byte"},
	} {
		err := ReadChunks(strings.NewReader(tc.input), func(int, string) error {
			t.Fatal("unexpected chunk")
			return nil
		})
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || err.Error() != tc.want) {
			t.Fatalf("ReadChunks(%q) error = %v; want %q", tc.input, err, tc.want)
		}
	}
	want := errors.New("test failure")
	err := ReadChunks(iotest.ErrReader(want), func(int, string) error { return nil })
	if !errors.Is(err, want) {
		t.Fatalf("reader error = %v", err)
	}
	err = ReadChunks(strings.NewReader("hello\n# heading\n"), func(int, string) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("emit error = %v", err)
	}
}
