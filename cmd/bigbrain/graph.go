package main

import (
	"bufio"
	_ "embed"
	"html/template"
	"io"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

//go:embed graph.html
var graphHTML string

type graphPage struct {
	Request   protocol.QueryRequest
	Count     int
	Fallback  string
	Matches   []graphMatch
	Returned  map[string]bool
	NodeCount int
	EdgeCount int
}

type graphMatch struct {
	protocol.Match
	Steps []graphStep
}

type graphStep struct {
	protocol.Path
	Number     int
	TravelFrom string
	TravelTo   string
}

func writeGraph(w io.Writer, request protocol.QueryRequest, result protocol.QueryResult) error {
	tmpl, err := template.New("graph").Parse(graphHTML)
	if err != nil {
		return err
	}
	page := graphPage{
		Request: request, Count: result.Count, Fallback: result.Fallback,
		Returned: make(map[string]bool, len(result.Matches)),
	}
	nodes := make(map[string]bool)
	edges := make(map[string]bool)
	for _, match := range result.Matches {
		nodes[match.ID] = true
		page.Returned[match.ID] = true
		item := graphMatch{Match: match}
		paths := match.Reason.Steps
		// v1 uses path alone for one hop and repeats the final step for longer paths.
		if len(paths) == 0 && match.Reason.Path != nil {
			paths = []protocol.Path{*match.Reason.Path}
		}
		for i, path := range paths {
			nodes[path.FromID], nodes[path.ToID] = true, true
			edges[path.RelationshipID] = true
			from, to := path.FromID, path.ToID
			if path.Direction == "reverse" {
				from, to = to, from
			}
			item.Steps = append(item.Steps, graphStep{path, i + 1, from, to})
		}
		page.Matches = append(page.Matches, item)
	}
	page.NodeCount, page.EdgeCount = len(nodes), len(edges)
	output := bufio.NewWriter(w)
	if err := tmpl.Execute(output, page); err != nil {
		return err
	}
	return output.Flush()
}
