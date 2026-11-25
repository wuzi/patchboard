package posts_test

import (
	"strings"
	"testing"

	"patchboard/internal/posts"
	"patchboard/internal/testutil"
)

func TestSlugify(t *testing.T) {
	testutil.Eq(t, posts.Slugify("Aumentar tamanho do chat"), "aumentar-tamanho-do-chat")
	testutil.Eq(t, posts.Slugify("Ação rápida 🎮!!"), "acao-rapida")
	testutil.Eq(t, posts.Slugify(`Remoção da "Boots of Haste"`), "remocao-da-boots-of-haste")
	testutil.Eq(t, posts.Slugify("🎮🎮"), "post")
	long := posts.Slugify(strings.Repeat("palavra ", 15))
	if len(long) > 60 || strings.HasSuffix(long, "-") || strings.HasPrefix(long, "-") {
		t.Fatalf("long slug = %q (%d)", long, len(long))
	}
	testutil.Eq(t, posts.Slugify(strings.Repeat("a", 80)), strings.Repeat("a", 60))
}
