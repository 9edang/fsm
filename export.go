package fsm

import (
	"fmt"
	"strings"
)

// ToMermaid returns a Mermaid stateDiagram-v2 string representing all
// registered transitions. The output can be embedded directly in Markdown.
func (f *FSM) ToMermaid() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("stateDiagram-v2\n")
	fmt.Fprintf(&sb, "    [*] --> %s\n", f.def.initial)
	for _, t := range f.def.transitions {
		for _, from := range t.from {
			fmt.Fprintf(&sb, "    %s --> %s : %s\n", from, t.to, t.event)
		}
	}
	return sb.String()
}

// ToDOT returns a Graphviz DOT string representing all registered transitions.
func (f *FSM) ToDOT() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("digraph fsm {\n")
	sb.WriteString("    __start__ [shape=point];\n")
	fmt.Fprintf(&sb, "    __start__ -> %s;\n", f.def.initial)
	for _, t := range f.def.transitions {
		for _, from := range t.from {
			fmt.Fprintf(&sb, "    %s -> %s [label=%q];\n", from, t.to, t.event)
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}
