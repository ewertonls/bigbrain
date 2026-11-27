package main

import (
	"context"
	"github.com/ewertonls/bigbrain/internal/daemon"
	"github.com/ewertonls/bigbrain/internal/protocol"
	"time"
)

type frozenHandoff struct {
	Records   []protocol.RecordRequest `json:"records"`
	IDs       []string                 `json:"ids"`
	Link      *protocol.LinkRequest    `json:"link,omitempty"`
	LinkID    string                   `json:"link_id,omitempty"`
	Confirmed time.Time                `json:"confirmed,omitempty"`
}

func freeze(d handoffDraft) *frozenHandoff {
	m := &frozenHandoff{}
	for i, c := range []*proposedMemory{&d.Content.Memory, d.Content.Evidence} {
		if c != nil {
			suffix := "/decision"
			if i == 1 {
				suffix = "/evidence"
			}
			m.Records = append(m.Records, protocol.RecordRequest{Scope: d.Origin.Scope, Key: d.ID + suffix, Text: c.Text, Type: c.Type, Source: c.Source})
			m.IDs = append(m.IDs, "")
		}
	}
	return m
}
func saveHandoff(ctx context.Context, b binding, dir, id string, revision int, autoStart bool) (handoffDraft, error) {
	path := b.sessionFile(dir, "drafts")
	// Serialize changes to the manifest, releasing the lock before each RPC.
	change := func(fn func(*handoffDraft) error) (handoffDraft, error) {
		var d handoffDraft
		err := updateState(ctx, path, func(ds *drafts) error {
			i := ds.index(id)
			if i < 0 {
				return invalid("Unknown draft in this binding.")
			}
			item := &(*ds)[i]
			if item.Origin.Database != b.Database || item.Origin.Scope != b.Scope || item.Revision != revision {
				return invalid("Binding or reviewed revision changed.")
			}
			if err := fn(item); err != nil {
				return err
			}
			d = *item
			return nil
		})
		return d, err
	}
	d, err := change(func(d *handoffDraft) error {
		if d.Status == "dismissed" {
			return invalid("Draft was dismissed.")
		}
		if d.Manifest == nil {
			if !autoStart {
				return invalid("No reviewed operation to retry.")
			}
			d.Manifest = freeze(*d)
			d.Status = "saving"
		}
		return nil
	})
	if err != nil {
		return d, err
	}
	connect := func() (*daemon.Client, error) {
		c, err := daemon.Connect(ctx, d.Origin.Database, autoStart)
		if err == nil && c == nil {
			err = invalid("Daemon unavailable for retry.")
		}
		return c, err
	}
	var client *daemon.Client
	for d.Status != "saved" {
		m := d.Manifest
		idx := -1
		for i, v := range m.IDs {
			if v == "" {
				idx = i
				break
			}
		}
		if idx < 0 && (len(m.IDs) == 1 || m.LinkID != "") {
			d, err = change(func(d *handoffDraft) error {
				if d.Manifest.Confirmed.IsZero() {
					d.Manifest.Confirmed = time.Now().UTC()
				}
				d.Status = "saved"
				return nil
			})
			if err != nil {
				return d, err
			}
			break
		}
		if idx < 0 && m.Link == nil {
			d, err = change(func(d *handoffDraft) error {
				m := d.Manifest
				if m.Link == nil {
					m.Link = &protocol.LinkRequest{Scope: d.Origin.Scope, Key: d.ID + "/link", FromID: m.IDs[0], ToID: m.IDs[1], Kind: "justified_by", Source: d.Content.Evidence.Source, Weight: 1}
				}
				return nil
			})
			if err != nil {
				return d, err
			}
			continue
		}
		if client == nil {
			client, err = connect()
		}
		var res protocol.WriteResult
		if err == nil {
			if idx >= 0 {
				err = client.Call(ctx, "record", m.Records[idx], &res)
			} else {
				err = client.Call(ctx, "link", m.Link, &res)
			}
		}
		if err != nil {
			cause := err
			_, _ = change(func(d *handoffDraft) error {
				if d.Status != "saved" && d.Status != "partial" {
					d.Status = "uncertain"
				}
				return nil
			})
			return d, cause
		}
		d, err = change(func(d *handoffDraft) error {
			if idx >= 0 {
				d.Manifest.IDs[idx] = res.ID
			} else {
				d.Manifest.LinkID = res.ID
			}
			if d.Status != "saved" {
				d.Status = "partial"
			}
			return nil
		})
		if err != nil {
			return d, err
		}
	}
	// Repeating a confirmed save also repairs an interrupted pointer update.
	err = updateState(ctx, b.handoffPointer(dir), func(latest *latestHandoff) error {
		if !latest.At.After(d.Manifest.Confirmed) {
			*latest = latestHandoff{d.Manifest.IDs, d.Origin.Workspace, d.Manifest.Confirmed}
		}
		return nil
	})
	return d, err
}
