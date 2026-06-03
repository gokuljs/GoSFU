package agent

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// sentenceChunker buffers streamed LLM tokens and flushes complete sentences so
// TTS can start speaking before the LLM finishes.
//
// Unlike a naive "split on the first . ! ?" scan, it refuses to break inside:
//   - decimals / versions: "9.99", "v1.2.3"
//   - titles / abbreviations: "Mr.", "Dr.", "etc.", "U.S."
//   - single-letter initials: "J. Smith"
//   - URLs / domains: "example.com"
//   - ellipses: "wait..."
//
// It is also stream-safe: a terminator sitting at the very end of the buffer is
// held back until the next token arrives (or the stream ends), so "9." is never
// split before we can tell it was really "9.99".
type sentenceChunker struct {
	buf      strings.Builder
	maxChars int // hard cap: flush even without a boundary once a run grows past this
	minChars int // soft floor: merge tiny fragments instead of speaking them alone
}

func newSentenceChunker(maxChars int) *sentenceChunker {
	return &sentenceChunker{maxChars: maxChars}
}

// push adds a token delta and returns any ready-to-speak chunks.
func (c *sentenceChunker) push(delta string) []string {
	c.buf.WriteString(delta)
	return c.drain(false)
}

// flush returns whatever remains (call after the LLM stream ends).
func (c *sentenceChunker) flush() []string { return c.drain(true) }

func (c *sentenceChunker) drain(force bool) []string {
	chunks, rest := splitSentences(c.buf.String(), force, c.minChars)

	if force {
		if t := strings.TrimSpace(rest); t != "" {
			chunks = append(chunks, t)
			rest = ""
		}
	} else if c.maxChars > 0 {
		// No sentence boundary in sight but the buffer keeps growing: cut at the
		// last word boundary so punctuation-free text still reaches TTS.
		for {
			chunk, remainder, ok := splitAtMax(rest, c.maxChars)
			if !ok {
				break
			}
			chunks = append(chunks, chunk)
			rest = remainder
		}
	}

	c.buf.Reset()
	c.buf.WriteString(rest)
	return chunks
}

// splitSentences extracts confirmed sentences from s and returns the unconsumed
// remainder. When force is false, a terminator at the very end of s is left
// pending because the next token may turn "9." into "9.99".
func splitSentences(s string, force bool, minChars int) (chunks []string, rest string) {
	r := []rune(s)
	n := len(r)
	start := 0
	i := 0

	for i < n {
		if !isTerminator(r[i]) {
			i++
			continue
		}

		// Consume a run of terminators so "?!" and "..." stay together.
		runStart := i
		j := i
		for j < n && isTerminator(r[j]) {
			j++
		}

		// An ellipsis ("...") is a continuation, not a sentence end — keep
		// accumulating so "Wait... what?" stays one phrase.
		if isEllipsis(r, runStart, j) {
			i = j
			continue
		}

		// Let trailing closing quotes/brackets ride along: she said "Go!"
		end := j
		for end < n && isClosing(r[end]) {
			end++
		}

		if end >= n {
			if !force {
				break // terminator at the edge — wait for the next token
			}
		} else if !unicode.IsSpace(r[end]) {
			i = j // "example.com", "9.99", "3.5x" — not a boundary
			continue
		}

		// A lone period may be an abbreviation or an initial, not a real end.
		if j-runStart == 1 && r[runStart] == '.' && isAbbreviation(r, runStart) {
			i = j
			continue
		}

		candidate := strings.TrimSpace(string(r[start:end]))
		if candidate == "" {
			start = end
			i = end
			continue
		}
		if !force && minChars > 0 && utf8.RuneCountInString(candidate) < minChars {
			i = j // too short — keep accumulating so TTS gets a real phrase
			continue
		}

		chunks = append(chunks, candidate)
		start = end
		i = end
	}

	rest = string(r[start:])
	return chunks, rest
}

// isAbbreviation reports whether the period at index dot terminates a known
// abbreviation or a single-letter initial rather than a sentence.
func isAbbreviation(r []rune, dot int) bool {
	// Walk back over the token attached to the period (letters and inner dots).
	k := dot - 1
	for k >= 0 && (unicode.IsLetter(r[k]) || r[k] == '.') {
		k--
	}
	word := strings.ToLower(strings.Trim(string(r[k+1:dot]), "."))
	if word == "" {
		return false
	}
	if utf8.RuneCountInString(word) == 1 {
		return true // single-letter initial, e.g. "J."
	}
	return sentenceAbbreviations[word]
}

// sentenceAbbreviations holds lowercased tokens that commonly carry a trailing
// period without ending a sentence. Acronyms are stored with their inner dots.
var sentenceAbbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"sr": true, "jr": true, "st": true, "vs": true, "etc": true,
	"inc": true, "ltd": true, "co": true, "corp": true, "dept": true,
	"fig": true, "vol": true, "no": true, "al": true, "approx": true,
	"e.g": true, "i.e": true, "a.m": true, "p.m": true,
	"u.s": true, "u.k": true, "u.n": true, "ph.d": true,
}

func isTerminator(r rune) bool { return r == '.' || r == '!' || r == '?' }

// isEllipsis reports whether r[start:end] is a run of two or more periods.
func isEllipsis(r []rune, start, end int) bool {
	if end-start < 2 {
		return false
	}
	for k := start; k < end; k++ {
		if r[k] != '.' {
			return false
		}
	}
	return true
}

func isClosing(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '}', '»', '”', '’':
		return true
	default:
		return false
	}
}

// splitAtMax force-cuts an over-long remainder at the last word boundary within
// maxChars (falling back to a hard cut if there is no space), so a long run of
// punctuation-free text still gets spoken instead of buffering forever.
func splitAtMax(rest string, maxChars int) (chunk, remainder string, ok bool) {
	r := []rune(rest)
	if len(r) < maxChars {
		return "", rest, false
	}
	cut := -1
	for i := 0; i < len(r) && i <= maxChars; i++ {
		if unicode.IsSpace(r[i]) {
			cut = i
		}
	}
	if cut <= 0 {
		cut = maxChars
	}
	chunk = strings.TrimSpace(string(r[:cut]))
	remainder = strings.TrimLeft(string(r[cut:]), " \t\r\n")
	if chunk == "" {
		return "", rest, false
	}
	return chunk, remainder, true
}
