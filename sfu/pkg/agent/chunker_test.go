package agent

import (
	"reflect"
	"testing"
)

func TestChunkerEdgeCases(t *testing.T) {
	cases := []struct {
		name   string
		tokens []string
		want   []string
	}{
		{
			name:   "decimal not split",
			tokens: []string{"It costs ", "9.99 ", "dollars ", "today. ", "Done."},
			want:   []string{"It costs 9.99 dollars today.", "Done."},
		},
		{
			name:   "abbreviation Mr",
			tokens: []string{"Mr. ", "Smith ", "left the ", "room. ", "Bye."},
			want:   []string{"Mr. Smith left the room.", "Bye."},
		},
		{
			name:   "acronym US",
			tokens: []string{"The U.S. ", "economy ", "is growing. ", "Yes."},
			want:   []string{"The U.S. economy is growing.", "Yes."},
		},
		{
			name:   "url not split",
			tokens: []string{"Visit ", "example.com ", "now. ", "Ok."},
			want:   []string{"Visit example.com now.", "Ok."},
		},
		{
			name:   "ellipsis kept",
			tokens: []string{"Wait... ", "what happened ", "here? ", "Tell me."},
			want:   []string{"Wait... what happened here?", "Tell me."},
		},
		{
			name:   "interrobang",
			tokens: []string{"Really?! ", "I cannot ", "believe it."},
			want:   []string{"Really?!", "I cannot believe it."},
		},
		{
			name:   "single letter initial",
			tokens: []string{"J. ", "R. R. ", "Tolkien ", "wrote it. ", "Nice."},
			want:   []string{"J. R. R. Tolkien wrote it.", "Nice."},
		},
		{
			name:   "decimal split across tokens",
			tokens: []string{"I paid 9.", "99 dollars ", "total. ", "Thanks."},
			want:   []string{"I paid 9.99 dollars total.", "Thanks."},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newSentenceChunker(200)
			var got []string
			for _, tok := range tc.tokens {
				got = append(got, c.push(tok)...)
			}
			got = append(got, c.flush()...)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestChunkerMaxCharsFallback(t *testing.T) {
	c := newSentenceChunker(10)
	got := append(c.push("abcdefghij klmno"), c.flush()...)
	want := []string{"abcdefghij", "klmno"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
