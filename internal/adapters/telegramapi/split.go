package telegramapi

import "strings"

// split breaks text into chunks of at most limit characters, on a block
// boundary — a blank line between paragraphs — and never inside a fenced
// code block. A single block bigger than limit (a huge unbroken code fence,
// say) is returned whole rather than cut mid-content: Telegram will reject
// it, which is an honest failure Send reports, not a corrupted message this
// package produced by slicing through the middle of it.
func split(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	blocks := splitBlocks(text)
	var chunks []string
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
	}

	for _, b := range blocks {
		sep := ""
		if cur.Len() > 0 {
			sep = "\n\n"
		}
		if cur.Len()+len(sep)+len(b) > limit {
			flush()
			if len(b) > limit {
				// One block alone exceeds the limit — nothing to break it on
				// without risking a mid-fence cut. Ship it as its own chunk.
				chunks = append(chunks, b)
				continue
			}
		} else {
			cur.WriteString(sep)
		}
		cur.WriteString(b)
	}
	flush()
	return chunks
}

// splitBlocks divides text on blank lines, except inside a ``` fence, which
// is kept intact regardless of how many blank lines it contains.
func splitBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var cur []string
	inFence := false

	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			cur = append(cur, line)
			continue
		}
		if !inFence && strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return blocks
}
