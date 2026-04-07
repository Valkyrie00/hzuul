package views

import "strings"

func colorizeLogChunk(text string) string {
	var out strings.Builder
	out.Grow(len(text) + len(text)/4)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		if line == "" {
			continue
		}
		out.WriteString(colorizeLogLine(line))
	}
	return out.String()
}

func colorizeLogLine(line string) string {
	// Zuul log format: "TIMESTAMP | HOST | ANSIBLE_CONTENT"
	// Find first and second " | " to separate the three parts.
	first := strings.Index(line, " | ")
	if first < 0 || first > 35 {
		return colorizePlainLine(line)
	}

	rest := line[first+3:]
	second := strings.Index(rest, " | ")

	var prefix, content string
	if second >= 0 {
		host := rest[:second]
		content = rest[second+3:]
		prefix = "[::d]" + line[:first+3] + host + " | " + "[-:-:-]"
	} else {
		content = rest
		prefix = "[::d]" + line[:first+3] + "[-:-:-]"
	}

	trimmed := strings.TrimSpace(content)

	switch {
	case strings.HasPrefix(trimmed, "PLAY RECAP"):
		return prefix + "[#3884f4::b]" + content + "[-:-:-]"
	case strings.HasPrefix(trimmed, "PLAY ["):
		return prefix + "[#3884f4::b]" + content + "[-:-:-]"
	case strings.HasPrefix(trimmed, "TASK ["):
		return prefix + "[white::b]" + content + "[-:-:-]"
	case strings.HasPrefix(trimmed, "ok:"):
		return prefix + "[green]" + content + "[-]"
	case strings.HasPrefix(trimmed, "changed:"):
		return prefix + "[yellow]" + content + "[-]"
	case strings.HasPrefix(trimmed, "fatal:"),
		strings.HasPrefix(trimmed, "FAILED"):
		return prefix + "[red::b]" + content + "[-:-:-]"
	case strings.HasPrefix(trimmed, "skipping:"),
		strings.HasPrefix(trimmed, "included:"),
		strings.HasPrefix(trimmed, "...ignoring"):
		return prefix + "[::d]" + content + "[-:-:-]"
	case strings.Contains(trimmed, "SKIPPED:"):
		return prefix + "[::d]" + content + "[-:-:-]"
	case isRecapLine(trimmed):
		return prefix + colorizeRecapLine(content)
	case strings.HasSuffix(trimmed, "... ok"):
		return prefix + content[:len(content)-2] + "[green]ok[-]"
	case strings.HasSuffix(trimmed, "... FAILED"):
		return prefix + content[:len(content)-6] + "[red::b]FAILED[-:-:-]"
	default:
		return prefix + content
	}
}

func colorizePlainLine(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "PLAY RECAP"):
		return "[#3884f4::b]" + line + "[-:-:-]"
	case strings.HasPrefix(trimmed, "PLAY ["):
		return "[#3884f4::b]" + line + "[-:-:-]"
	case strings.HasPrefix(trimmed, "TASK ["):
		return "[white::b]" + line + "[-:-:-]"
	case strings.HasPrefix(trimmed, "fatal:"),
		strings.HasPrefix(trimmed, "FAILED"):
		return "[red::b]" + line + "[-:-:-]"
	default:
		return line
	}
}

func isRecapLine(s string) bool {
	return strings.Contains(s, "ok=") && strings.Contains(s, "failed=")
}

func colorizeRecapLine(line string) string {
	var out strings.Builder
	parts := strings.Fields(line)
	for i, p := range parts {
		if i > 0 {
			out.WriteByte(' ')
		}
		switch {
		case strings.HasPrefix(p, "ok=") && !strings.HasSuffix(p, "=0"):
			out.WriteString("[green]" + p + "[-]")
		case strings.HasPrefix(p, "changed=") && !strings.HasSuffix(p, "=0"):
			out.WriteString("[yellow]" + p + "[-]")
		case strings.HasPrefix(p, "failed=") && !strings.HasSuffix(p, "=0"):
			out.WriteString("[red::b]" + p + "[-:-:-]")
		case strings.HasPrefix(p, "unreachable=") && !strings.HasSuffix(p, "=0"):
			out.WriteString("[red]" + p + "[-]")
		case strings.HasPrefix(p, "rescued=") && !strings.HasSuffix(p, "=0"):
			out.WriteString("[#e5c07b]" + p + "[-]")
		default:
			out.WriteString(p)
		}
	}
	return out.String()
}
