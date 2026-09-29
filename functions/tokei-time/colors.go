package main

import (
	"hash/fnv"
	"sort"
)

var languageColors = map[string]string{
	"Go":         "#00ADD8",
	"Rust":       "#DEA584",
	"Python":     "#3572A5",
	"JavaScript": "#F7DF1E",
	"TypeScript": "#3178C6",
	"C":          "#555555",
	"C++":        "#F34B7D",
	"C#":         "#178600",
	"Java":       "#B07219",
	"Kotlin":     "#7F52FF",
	"Swift":      "#F05138",
	"Ruby":       "#CC342D",
	"PHP":        "#4F5D95",
	"Shell":      "#89E051",
	"Bash":       "#89E051",
	"Zsh":        "#89E051",
	"Markdown":   "#D787FF",
	"HTML":       "#E34F26",
	"CSS":        "#2965F1",
	"SCSS":       "#C6538C",
	"Sass":       "#A23B72",
	"JSON":       "#4DB6AC",
	"YAML":       "#CB171E",
	"TOML":       "#9C4221",
	"SQL":        "#E38C00",
	"Lua":        "#000080",
	"Zig":        "#EC915C",
	"Dockerfile": "#384D54",
	"Makefile":   "#AF875F",
	"Vim Script": "#019833",
	"Elixir":     "#6E4A7E",
	"Haskell":    "#5E5086",
	"Scala":      "#DC322F",
	"Dart":       "#00B4AB",
	"R":          "#198CE7",
	"Julia":      "#A270BA",
	"Perl":       "#0298C3",
	"Nix":        "#7E7EFF",
	"Fish":       "#4AA3D9",
	"GraphQL":    "#E10098",
	"Protobuf":   "#48768A",
}

var fallbackPalette = []string{
	"#FF5F87", "#00D7D7", "#5FD75F", "#FFAF00",
	"#AF87FF", "#FF875F", "#87D7FF", "#D75F87",
	"#5FAFAF", "#D7AF5F", "#87AFD7", "#AFDF87",
	"#D78787", "#87DFDF", "#FFAFD7", "#AFAFD7",
}

// GetLanguageColor returns the hex color code for a programming language.
func GetLanguageColor(lang string) string {
	if c, ok := languageColors[lang]; ok {
		return c
	}
	h := fnv.New32a()
	h.Write([]byte(lang))
	idx := int(h.Sum32() % uint32(len(fallbackPalette)))
	return fallbackPalette[idx]
}

// BarSegment represents a single colored slice of a stacked bar.
type BarSegment struct {
	Language string
	Value    int
	Chars    int
	Color    string
}

// CalculateHorizontalSegments allocates character width among languages using
// the Largest Remainder Method (Hamilton-Hare) to prevent rounding errors.
func CalculateHorizontalSegments(snapshot *CommitSnapshot, metric Metric, availableWidth int, maxValue int) (int, int, []BarSegment) {
	totalVal := snapshot.Total.Value(metric)
	if totalVal <= 0 || maxValue <= 0 || availableWidth <= 0 {
		return totalVal, 0, nil
	}

	barWidth := int(float64(totalVal)/float64(maxValue)*float64(availableWidth) + 0.5)
	if barWidth < 1 {
		barWidth = 1
	}
	if barWidth > availableWidth {
		barWidth = availableWidth
	}

	type item struct {
		lang string
		val  int
	}
	var items []item
	for lang, stats := range snapshot.Languages {
		v := stats.Value(metric)
		if v > 0 {
			items = append(items, item{lang: lang, val: v})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].val == items[j].val {
			return items[i].lang < items[j].lang
		}
		return items[i].val > items[j].val
	})

	if len(items) == 0 {
		return totalVal, 0, nil
	}

	type remItem struct {
		index int
		rem   float64
	}
	allocated := 0
	segments := make([]BarSegment, len(items))
	remainders := make([]remItem, len(items))

	for i, it := range items {
		exact := float64(it.val) / float64(totalVal) * float64(barWidth)
		base := int(exact)
		allocated += base
		segments[i] = BarSegment{
			Language: it.lang,
			Value:    it.val,
			Chars:    base,
			Color:    GetLanguageColor(it.lang),
		}
		remainders[i] = remItem{index: i, rem: exact - float64(base)}
	}

	missing := barWidth - allocated
	if missing > 0 {
		sort.Slice(remainders, func(i, j int) bool {
			return remainders[i].rem > remainders[j].rem
		})
		for i := range missing {
			idx := remainders[i%len(remainders)].index
			segments[idx].Chars++
		}
	}

	var visibleSegments []BarSegment
	for _, seg := range segments {
		if seg.Chars > 0 {
			visibleSegments = append(visibleSegments, seg)
		}
	}

	return totalVal, barWidth, visibleSegments
}

// CalculateVerticalColumn calculates the language row allocation for a vertical bar column.
// Rows are indexed from 0 (bottom) to chartHeight-1 (top).
func CalculateVerticalColumn(snapshot *CommitSnapshot, metric Metric, chartHeight int, maxValue int) []string {
	col := make([]string, chartHeight) // empty strings mean space
	totalVal := snapshot.Total.Value(metric)
	if totalVal <= 0 || maxValue <= 0 || chartHeight <= 0 {
		return col
	}

	h := int(float64(totalVal)/float64(maxValue)*float64(chartHeight) + 0.5)
	if h < 1 {
		h = 1
	}
	if h > chartHeight {
		h = chartHeight
	}

	type item struct {
		lang string
		val  int
	}
	var items []item
	for lang, stats := range snapshot.Languages {
		v := stats.Value(metric)
		if v > 0 {
			items = append(items, item{lang: lang, val: v})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].val == items[j].val {
			return items[i].lang < items[j].lang
		}
		return items[i].val > items[j].val
	})

	if len(items) == 0 {
		return col
	}

	// Allocate rows to languages
	type remItem struct {
		index int
		rem   float64
	}
	allocated := 0
	rowCounts := make([]int, len(items))
	remainders := make([]remItem, len(items))

	for i, it := range items {
		exact := float64(it.val) / float64(totalVal) * float64(h)
		base := int(exact)
		allocated += base
		rowCounts[i] = base
		remainders[i] = remItem{index: i, rem: exact - float64(base)}
	}

	missing := h - allocated
	if missing > 0 {
		sort.Slice(remainders, func(i, j int) bool {
			return remainders[i].rem > remainders[j].rem
		})
		for i := range missing {
			idx := remainders[i%len(remainders)].index
			rowCounts[idx]++
		}
	}

	// Fill rows from bottom (0) to top (h-1)
	currRow := 0
	for i, it := range items {
		cnt := rowCounts[i]
		color := GetLanguageColor(it.lang)
		for range cnt {
			if currRow < chartHeight {
				col[currRow] = color
				currRow++
			}
		}
	}

	return col
}
