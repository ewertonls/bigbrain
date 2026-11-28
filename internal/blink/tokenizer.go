package blink

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/rangetable"
)

type addedToken struct {
	Content                    string
	ID                         int
	Normalized, LStrip, RStrip bool
	SingleWord                 bool `json:"single_word"`
}

type tokenizer struct {
	vocab                                      map[string]int
	added                                      map[rune][]addedToken
	unknown, chunk, maxWord, maxTokens, median int
	longestPiece                               int
	rows                                       int
	prefix                                     string
	clean, chinese, strip, lowercase           bool
}

func parseTokenizer(data []byte, maxTokens int) (tokenizer, error) {
	var rules struct {
		Version    string
		Normalizer struct {
			Type    string
			Clean   *bool `json:"clean_text"`
			Chinese *bool `json:"handle_chinese_chars"`
			Strip   *bool `json:"strip_accents"`
			Lower   *bool `json:"lowercase"`
		}
		Pre struct {
			Type   string
			Stages []struct {
				Type   string
				Length int
			} `json:"pretokenizers"`
		} `json:"pre_tokenizer"`
		Added []addedToken `json:"added_tokens"`
		Model struct {
			Type    string
			Vocab   map[string]int
			Unk     string `json:"unk_token"`
			Prefix  string `json:"continuing_subword_prefix"`
			MaxWord int    `json:"max_input_chars_per_word"`
		}
	}
	var t tokenizer
	if err := json.Unmarshal(data, &rules); err != nil {
		return t, err
	}
	model, norm, pre := rules.Model, rules.Normalizer, rules.Pre
	if rules.Version != "1.0" || model.Type != "WordPiece" || len(model.Vocab) == 0 || model.MaxWord <= 0 || model.Prefix == "" {
		return t, errors.New("expected a v1 WordPiece tokenizer and vocabulary")
	}
	if norm.Type != "BertNormalizer" || norm.Clean == nil || norm.Chinese == nil || norm.Lower == nil {
		return t, errors.New("unsupported normalizer")
	}
	if pre.Type != "Sequence" || len(pre.Stages) != 2 || pre.Stages[0].Type != "BertPreTokenizer" || pre.Stages[1].Type != "FixedLength" || pre.Stages[1].Length <= 0 {
		return t, errors.New("expected BertPreTokenizer followed by FixedLength")
	}
	t.vocab, t.maxWord, t.prefix, t.chunk, t.maxTokens = model.Vocab, model.MaxWord, model.Prefix, pre.Stages[1].Length, maxTokens
	t.clean, t.chinese, t.lowercase = *norm.Clean, *norm.Chinese, *norm.Lower
	t.strip = t.lowercase
	if norm.Strip != nil {
		t.strip = *norm.Strip
	}
	unknown, ok := t.vocab[model.Unk]
	if !ok || model.Unk == "" {
		return t, errors.New("missing unknown token")
	}
	t.unknown = unknown
	seen := make([]bool, len(t.vocab))
	var lengths []int
	if maxTokens > 0 {
		lengths = make([]int, 0, len(t.vocab))
	}
	for word, id := range t.vocab {
		if word == "" || id < 0 || id >= len(seen) || seen[id] {
			return t, errors.New("expected dense, unique WordPiece IDs")
		}
		seen[id] = true
		t.longestPiece = max(t.longestPiece, utf8.RuneCountInString(word))
		if maxTokens > 0 {
			lengths = append(lengths, utf8.RuneCountInString(word))
		}
	}
	if maxTokens > 0 {
		slices.Sort(lengths)
		t.median = lengths[len(lengths)/2]
	}
	t.added = make(map[rune][]addedToken)
	t.rows = len(t.vocab)
	seenAdded := make(map[string]bool, len(rules.Added))
	for _, a := range rules.Added {
		if a.Content == "" || seenAdded[a.Content] || a.Normalized || a.LStrip || a.RStrip || a.SingleWord {
			return t, fmt.Errorf("unsupported or duplicate added token %q", a.Content)
		}
		seenAdded[a.Content] = true
		expected, inVocab := t.vocab[a.Content]
		if !inVocab {
			// Added CJK words have their own embedding rows. They are
			// matched on the original text and are never WordPiece candidates.
			expected = t.rows
			t.rows++
		}
		if a.ID != expected {
			return t, fmt.Errorf("invalid ID %d for added token %q; expected %d", a.ID, a.Content, expected)
		}
		first, _ := utf8.DecodeRuneInString(a.Content)
		t.added[first] = append(t.added[first], a)
	}
	for _, list := range t.added {
		slices.SortFunc(list, func(a, b addedToken) int { return len(b.Content) - len(a.Content) })
	}
	return t, nil
}

// Literal added tokens are extracted before normalization. Disable automatic
// CLS/SEP insertion, as StaticModel encodes with add_special_tokens=false.
func (t *tokenizer) ids(text string) []int {
	if t.maxTokens > 0 && len(text) > t.maxTokens*t.median {
		var n int
		for pos := range text {
			if n == t.maxTokens*t.median {
				text = text[:pos]
				break
			}
			n++
		}
	}
	var ids []int
	start := 0
	for pos, r := range text {
		if pos < start {
			continue
		}
		for _, a := range t.added[r] {
			if strings.HasPrefix(text[pos:], a.Content) {
				ids = t.span(text[start:pos], ids)
				ids = append(ids, a.ID)
				start = pos + len(a.Content)
				break
			}
		}
	}
	ids = t.span(text[start:], ids)
	// Unknown IDs occupy slots in the truncated encoding, then are dropped
	// before pooling. A literal [UNK] follows the same rule as a failed word.
	if t.maxTokens > 0 && len(ids) > t.maxTokens {
		ids = ids[:t.maxTokens]
	}
	return slices.DeleteFunc(ids, func(id int) bool { return id == t.unknown })
}

func (t *tokenizer) span(text string, ids []int) []int {
	if text == "" {
		return ids
	}
	var normalized strings.Builder
	for _, r := range text {
		if t.clean {
			if r == 0 || r == utf8.RuneError || (isControl(r) && r != '\t' && r != '\n' && r != '\r') {
				continue
			}
			if unicode.IsSpace(r) {
				r = ' '
			}
		}
		if t.chinese && isChinese(r) {
			normalized.WriteByte(' ')
			normalized.WriteRune(r)
			normalized.WriteByte(' ')
		} else {
			normalized.WriteRune(r)
		}
	}
	s := normalized.String()
	if t.strip {
		s = stripAccents(s)
	}
	var word []rune
	emit := func() {
		for chunk := range slices.Chunk(word, t.chunk) {
			ids = t.wordPiece(chunk, ids)
		}
		word = word[:0]
	}
	for _, r := range s {
		if t.lowercase {
			// The Unicode full lowercase mapping for dotted capital I expands.
			if r == 'İ' {
				word = append(word, 'i', '\u0307')
				continue
			}
			r = unicode.ToLower(r)
		}
		switch {
		case unicode.IsSpace(r):
			emit()
		case isPunctuation(r):
			emit()
			ids = t.wordPiece([]rune{r}, ids)
		default:
			word = append(word, r)
		}
	}
	emit()
	return ids
}

func (t *tokenizer) wordPiece(word []rune, ids []int) []int {
	if len(word) > t.maxWord {
		return append(ids, t.unknown)
	}
	before := len(ids)
	for start := 0; start < len(word); {
		prefix := ""
		if start != 0 {
			prefix = t.prefix
		}
		end := min(len(word), start+t.longestPiece)
		for ; end > start; end-- {
			if id, ok := t.vocab[prefix+string(word[start:end])]; ok {
				ids = append(ids, id)
				break
			}
		}
		if end == start {
			return append(ids[:before], t.unknown)
		}
		start = end
	}
	return ids
}

// The reference decomposes using Unicode 9 without a stream-safe length
// limit. Buffer surviving non-starters until the next canonical starter;
// nonspacing marks are dropped but can still delimit two combining sequences.
var assigned9 = rangetable.Assigned("9.0.0")

func stripAccents(text string) string {
	type mark struct {
		r     rune
		class uint8
	}
	var marks []mark
	var out strings.Builder
	out.Grow(len(text))
	flush := func() {
		if len(marks) > 1 {
			slices.SortStableFunc(marks, func(a, b mark) int { return int(a.class) - int(b.class) })
		}
		for _, m := range marks {
			out.WriteRune(m.r)
		}
		marks = marks[:0]
	}
	for _, r := range text {
		if r < utf8.RuneSelf || !unicode.Is(assigned9, r) {
			flush()
			out.WriteRune(r)
			continue
		}
		for _, d := range norm.NFD.String(string(r)) {
			class := norm.NFD.PropertiesString(string(d)).CCC()
			if class == 0 {
				flush()
			}
			if isNonspacingMark(d) {
				continue
			}
			if class == 0 {
				out.WriteRune(d)
			} else {
				marks = append(marks, mark{d, class})
			}
		}
	}
	flush()
	return out.String()
}

// tokenizers 0.23.3 and model2vec-rs 0.5.1 use unicode_categories
// 0.1.1 (Unicode 8). Some code points changed category in later releases.
var assigned8 = rangetable.Assigned("8.0.0")

func isControl(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Co, r) ||
		unicode.Is(unicode.Cf, r) && unicode.Is(assigned8, r)
}

func isNonspacingMark(r rune) bool {
	switch r {
	case 0x1734, 0x1171e:
		return true
	case 0x1885, 0x1886, 0xa9bd, 0x111c9:
		return false
	}
	return unicode.Is(unicode.Mn, r) && unicode.Is(assigned8, r)
}

func isPunctuation(r rune) bool {
	if r < 128 {
		return r >= '!' && r <= '/' || r >= ':' && r <= '@' || r >= '[' && r <= 0x60 || r >= '{' && r <= '~'
	}
	return r == 0x166d || r == 0x111c9 || unicode.IsPunct(r) && unicode.Is(assigned8, r)
}

// These are the ranges used by tokenizers' BertNormalizer.
func isChinese(r rune) bool {
	return r >= 0x4e00 && r <= 0x9fff || r >= 0x3400 && r <= 0x4dbf ||
		r >= 0x20000 && r <= 0x2a6df || r >= 0x2a700 && r <= 0x2b73f ||
		r >= 0x2b740 && r <= 0x2b81f || r >= 0x2b820 && r <= 0x2ceaf ||
		r >= 0xf900 && r <= 0xfaff || r >= 0x2f800 && r <= 0x2fa1f
}
