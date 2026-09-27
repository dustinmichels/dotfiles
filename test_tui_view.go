package main
import (
	"fmt"
	"strings"
	"github.com/muesli/termenv"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	lipgloss.SetColorProfile(termenv.TrueColor)
	items, err := loadPackages()
	if err != nil {
		panic(err)
	}
	m := initialModel(items, FilterAll)
	view := m.View()
	lines := strings.Split(view, "\n")
	fmt.Printf("Total lines in View(): %d\n", len(lines))
	caskLinesFound := 0
	for i, line := range lines {
		if strings.Contains(line, "cask") && strings.Contains(line, "\x1b[38;2;236;72;153m") {
			caskLinesFound++
			if caskLinesFound <= 3 {
				fmt.Printf("Cask line %d: %q\n", i, line[:min(len(line), 70)])
			}
		}
	}
	fmt.Printf("Cask styled lines found: %d\n", caskLinesFound)
}
