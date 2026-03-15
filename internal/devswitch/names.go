package devswitch

import (
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
)

// randomName は Docker 風の adjective_noun 形式でランダムな名前を返す。
func randomName() string {
	adj := strings.ToLower(gofakeit.Adjective())
	noun := strings.ToLower(gofakeit.Noun())
	return fmt.Sprintf("%s_%s", adj, noun)
}
