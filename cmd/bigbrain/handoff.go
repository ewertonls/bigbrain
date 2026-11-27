package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ewertonls/bigbrain/internal/protocol"
)

const draftBytes = 64 * 1024

type proposedMemory struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Source string `json:"source"`
}
type draftContent struct {
	Producer      string          `json:"producer"`
	Memory        proposedMemory  `json:"memory"`
	Evidence      *proposedMemory `json:"evidence,omitempty"`
	OpenQuestions []string        `json:"open_questions,omitempty"`
	NextSteps     []string        `json:"next_steps,omitempty"`
}
type handoffDraft struct {
	Manifest *frozenHandoff `json:"manifest,omitempty"`
	ID       string         `json:"id"`
	Revision int            `json:"revision"`
	Status   string         `json:"status"`
	Origin   binding        `json:"origin"`
	Content  draftContent   `json:"content"`
}
type drafts []handoffDraft

func (ds drafts) index(id string) int {
	return slices.IndexFunc(ds, func(d handoffDraft) bool { return d.ID == id })
}
func validateDraft(d handoffDraft) error {
	if !strings.HasPrefix(d.Origin.Scope, "project:") || strings.TrimSpace(d.Content.Producer) == "" {
		return invalid("A draft requires its originating project and producer.")
	}
	for _, m := range []*proposedMemory{&d.Content.Memory, d.Content.Evidence} {
		if m == nil {
			continue
		}
		r := protocol.RecordRequest{Scope: d.Origin.Scope, Key: d.ID, Text: m.Text, Source: m.Source, Type: m.Type}
		if err := r.Validate(); err != nil {
			return err
		}
	}
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(data) > draftBytes {
		return invalid("Draft exceeds 64 KiB.")
	}
	return nil
}
func changeDraft(ctx context.Context, path string, b binding, action, id string, revision int, content draftContent) (handoffDraft, error) {
	var result handoffDraft
	err := updateState(ctx, path, func(ds *drafts) error {
		switch action {
		case "stage":
			pending := 0
			for _, d := range *ds {
				if d.Status != "saved" && d.Status != "dismissed" {
					pending++
				}
			}
			if pending >= 50 {
				return invalid("Session already has 50 pending drafts.")
			}
			result = handoffDraft{ID: rand.Text(), Revision: 1, Status: "draft", Origin: b, Content: content}
			if err := validateDraft(result); err != nil {
				return err
			}
			*ds = append(*ds, result)
		case "edit", "dismiss":
			i := ds.index(id)
			if i < 0 {
				return invalid("Unknown draft in this binding.")
			}
			d := &(*ds)[i]
			if d.Status != "draft" || d.Revision != revision {
				return invalid("Draft revision or status changed. Reload before editing.")
			}
			d.Revision++
			if action == "edit" {
				d.Content = content
				if err := validateDraft(*d); err != nil {
					return err
				}
			} else {
				d.Status = "dismissed"
			}
			result = *d
		default:
			return invalid("Unknown draft action.")
		}
		return nil
	})
	return result, err
}
func decodeDraft(r io.Reader) (draftContent, error) {
	var c draftContent
	data, err := io.ReadAll(io.LimitReader(r, draftBytes+1))
	if err != nil {
		return c, err
	}
	if len(data) > draftBytes {
		return c, invalid("Draft exceeds 64 KiB.")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return c, invalid("Expected one JSON draft.")
	}
	return c, nil
}
func handoffCommand(o *options, args []string) (any, error) {
	if len(args) == 0 {
		return nil, invalid("Choose handoff stage, list, edit or dismiss.")
	}
	action := args[0]
	f := newFlags("handoff " + action)
	o.register(f)
	var p pluginOptions
	p.register(f)
	id := f.String("id", "", "draft ID")
	revision := f.Int("revision", 0, "expected draft revision")
	if err := o.parseFlags(f, args[1:]); err != nil {
		return parseError(f, err)
	}
	if p.global || p.session == "" {
		return nil, invalid("A project binding and session ID are required.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, err := p.bind(ctx, o.db)
	if err != nil {
		return nil, err
	}
	dir, err := p.stateDir()
	if err != nil {
		return nil, err
	}
	path := b.sessionFile(dir, "drafts")
	if action == "save" {
		return saveHandoff(ctx, b, dir, *id, *revision, true)
	}
	if action == "list" {
		ds, err := readState[drafts](path)
		if ds == nil {
			ds = drafts{}
		}
		return ds, err
	}
	var c draftContent
	if action == "stage" || action == "edit" {
		c, err = decodeDraft(os.Stdin)
		if err != nil {
			return nil, err
		}
	}
	return changeDraft(ctx, path, b, action, *id, *revision, c)
}
