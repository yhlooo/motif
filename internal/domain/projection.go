package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func Digest(v any) string { raw, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func DeltaDigest(before, after State) string {
	b, a := map[string]File{}, map[string]File{}
	canonical := func(f File) File {
		if f.ContentID != "" {
			f.ContentRef = f.ContentID
			f.ContentID = ""
		}
		return f
	}
	for p := range ChangedPaths(before, after) {
		if f, ok := before.Lookup(p); ok {
			b[p] = canonical(f)
		}
		if f, ok := after.Lookup(p); ok {
			a[p] = canonical(f)
		}
	}
	return Digest([]State{From(b), From(a)})
}
func SelectProjectionEvents(all []Event, projected map[string]bool, commits []string) ([]Event, error) {
	selected := map[string]bool{}
	for _, oid := range commits {
		selected[oid] = true
	}
	pairCount := map[string]int{}
	for _, e := range all {
		if selected[e.Commit] && e.PairID != "" && projected[e.Commit] == e.Projected {
			pairCount[e.PairID]++
		}
	}
	events := []Event{}
	for _, oid := range commits {
		var list []Event
		var worktree string
		for _, e := range all {
			if e.Commit != oid || projected[oid] != e.Projected {
				continue
			}
			if worktree != "" && worktree != e.WorktreeID {
				return nil, fmt.Errorf("multiple captured worktrees claim commit %s; projection is ambiguous", oid)
			}
			worktree = e.WorktreeID
			if e.PairID != "" && pairCount[e.PairID] == 2 {
				continue
			}
			list = append(list, e)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Sequence < list[j].Sequence })
		events = append(events, list...)
	}
	return events, nil
}

type ProjectionStep struct {
	EventID       string
	Before, After State
	Conflicts     []string
}
type ProjectionInput struct {
	Review       Review
	Events       []Event
	Steps        []ProjectionStep
	Base, Target State
	Warnings     []string
}

// AssembleProjection is pure: all Git, snapshot and merge IO has already ended.
func AssembleProjection(in ProjectionInput) (Review, error) {
	r := in.Review
	events := in.Events
	state, target := in.Base, in.Target
	if len(events) != len(in.Steps) {
		return r, fmt.Errorf("projection inputs are incomplete")
	}
	r.ComparisonMode = "merge_base"
	r.Closure = "verified"
	r.History = "complete"
	r.Diagnostics = []Diagnostic{}
	r.CheckSource = true
	r.Warning = strings.Join(in.Warnings, " ")
	for i := 0; i < len(events); {
		e := events[i]
		if e.Kind == "unknown" {
			r.History = "partial"
			r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: "unknown_source", Stage: "history", EventID: e.ID, Message: "事件来源未知"})
		}
		j := i + 1
		if e.BatchID != "" {
			for j < len(events) && events[j].BatchID == e.BatchID {
				j++
			}
		}
		item := ReviewItem{Commit: e.Commit, SourceBatch: e.BatchID, ID: ID(), Kind: "unbatched_patch", Title: "未分批修改", Decision: "pending", Before: Clone(state), Events: []string{}}
		if e.BatchID != "" {
			item.Kind = "batch"
			item.Title = e.BatchTitle
			item.Description = e.BatchDescription
		} else if e.Kind == "virtual_close" {
			item.Kind = "commit_virtual"
			item.Title = "revert 部分暂不提交内容"
		} else if e.Kind == "virtual_carry" {
			item.Kind = "commit_virtual"
			item.Title = "之前暂存变更"
		}
		for k := i; k < j; k++ {
			event := events[k]
			step := in.Steps[k]
			if step.EventID != event.ID || !Equal(step.Before, state) {
				return r, fmt.Errorf("prepared projection step is inconsistent")
			}
			state = step.After
			conflicts := step.Conflicts
			if len(conflicts) > 0 {
				r.History = "partial"
				r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: "projection_conflict", Stage: "replay", EventID: event.ID, Paths: conflicts, Message: "部分修改无法准确投影"})
				r.Warning += "无法准确投影的修改转为最终未知补齐：" + event.ID + " (" + strings.Join(conflicts, ", ") + ")。"
				item.Description += "\n部分路径不能准确投影，见原始事件与最终未知补齐：" + strings.Join(conflicts, ", ")
			}
			item.Events = append(item.Events, event.ID)
		}
		item.After = Clone(state)
		source := e.ID
		if e.BatchID != "" {
			source = e.BatchID
		}
		item.StableKey = e.Commit + "/" + item.Kind + "/" + source
		if e.Kind == "unknown" {
			item.StableKey += "/" + Digest([]any{item.Events, DeltaDigest(item.Before, item.After)})
		}
		item.Digest = DeltaDigest(item.Before, item.After)
		r.Items = append(r.Items, item)
		i = j
	}
	if !Equal(state, target) {
		r.Integrity = "completed_with_unknown"
		r.History = "partial"
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: "unknown_completion", Stage: "integrity", Message: "最终状态包含未归属的净变化"})
		r.Items = append(r.Items, ReviewItem{StableKey: r.Head + "/integrity_unknown/" + DeltaDigest(state, target), Commit: r.Head, Digest: DeltaDigest(state, target), ID: ID(), Kind: "integrity_unknown", Title: "未知修改：最终差异补齐", Description: "Git 最终状态中没有被上述采集历史覆盖的净变化。", Decision: "pending", Before: Clone(state), After: target, Events: []string{}})
		state = target
	}
	if len(r.Commits) == 0 && len(r.Items) == 0 {
		r.Warning = "当前源/目标分支之间没有待审阅提交。"
	} else if len(events) == 0 {
		r.History = "unavailable"
		r.Warning += "未采集到数据，可能未安装 hook 或 hook 运行失败"
	} else {
		r.Warning += "已有采集记录不代表中间过程完整；最终状态已经验证。"
	}
	if !Equal(state, target) {
		return r, fmt.Errorf("review integrity verification failed")
	}
	identities := []string{}
	for _, item := range r.Items {
		identities = append(identities, item.StableKey+"/"+item.Digest+"/"+item.Title+"/"+item.Description)
	}
	r.ViewDigest = Digest(struct {
		Source, Target, Base, Head string
		Commits, Items             []string
		History, Warning           string
		Diagnostics                []Diagnostic
	}{r.SourceOID, r.TargetOID, r.Base, r.Head, r.Commits, identities, r.History, r.Warning, r.Diagnostics})
	return r, nil
}
