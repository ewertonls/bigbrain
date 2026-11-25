// Package markdown splits local Markdown documents into memory-sized sections.
package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

// ReadChunks emits nonblank Markdown sections with their 1-based starting lines.
// It preserves line endings and indentation. Only an initial UTF-8 BOM is omitted.
func ReadChunks(r io.Reader, emit func(line int, text string) error) error {
	scan := bufio.NewScanner(r)
	scan.Buffer(nil, protocol.MaxMemoryTextBytes+1)
	scan.Split(lineFragment)

	var chunk strings.Builder
	line, first := 1, 1
	flush := func() error {
		text := chunk.String()
		chunk.Reset()
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return emit(first, text)
	}

	var open, marker fence
	lineStart := true
	for scan.Scan() {
		part := scan.Bytes()
		if line == 1 && lineStart {
			part = bytes.TrimPrefix(part, []byte("\xef\xbb\xbf"))
		}
		if !utf8.Valid(part) {
			return fmt.Errorf("line %d: invalid UTF-8", line)
		}
		if bytes.IndexByte(part, 0) != -1 {
			return fmt.Errorf("line %d: unexpected NUL byte", line)
		}

		newSection := false
		if lineStart {
			block := bytes.TrimLeft(part, " ")
			marker = fence{}
			// Four leading spaces start indented code rather than an ATX heading or fence.
			if len(part)-len(block) <= 3 && len(block) > 0 {
				newSection = open.char == 0 && isHeading(block)
				if block[0] == '`' || block[0] == '~' {
					marker.char = block[0]
					marker.read(block)
				}
			}
		} else if marker.char != 0 {
			marker.read(part)
		}
		if newSection || chunk.Len()+len(part) > protocol.MaxMemoryTextBytes {
			if err := flush(); err != nil {
				return err
			}
		}
		if chunk.Len() == 0 {
			first = line
		}
		chunk.Write(part)
		lineStart = bytes.HasSuffix(part, []byte("\n"))
		if lineStart {
			if open.char == 0 && marker.width >= 3 && !marker.invalidInfo {
				open = marker
			} else if open.char != 0 && marker.char == open.char && marker.width >= open.width && !marker.hasInfo {
				open = fence{}
			}
			line++
		}
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return flush()
}

func isHeading(block []byte) bool {
	title := bytes.TrimLeft(block, "#")
	hashes := len(block) - len(title)
	return hashes >= 1 && hashes <= 6 && (len(title) == 0 || strings.ContainsRune(" \t\r\n", rune(title[0])))
}

// A fence delimiter may span multiple fragments (including its info string).
type fence struct {
	char        byte
	width       int
	pastRun     bool
	hasInfo     bool
	invalidInfo bool
}

func (f *fence) read(part []byte) {
	if !f.pastRun {
		i := 0
		for i < len(part) && part[i] == f.char {
			i++
		}
		f.width += i
		part = part[i:]
		f.pastRun = len(part) > 0
	}
	f.hasInfo = f.hasInfo || len(bytes.Trim(part, " \t\r\n")) != 0
	f.invalidInfo = f.invalidInfo || (f.char == '`' && bytes.IndexByte(part, '`') != -1)
}

// Split on physical lines when possible. For a longer line, look one byte
// beyond the limit so a UTF-8 code point stays in a single fragment.
func lineFragment(data []byte, atEOF bool) (int, []byte, error) {
	end := min(len(data), protocol.MaxMemoryTextBytes)
	if i := bytes.IndexByte(data[:end], '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if len(data) > end {
		for i := 0; i < utf8.UTFMax-1 && !utf8.RuneStart(data[end]); i++ {
			end--
		}
		return end, data[:end], nil
	}
	if atEOF && end > 0 {
		return end, data[:end], nil
	}
	return 0, nil, nil
}
