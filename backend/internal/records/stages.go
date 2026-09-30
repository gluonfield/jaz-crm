package records

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type StageEdit struct {
	Action      string
	Stage       string
	Name        string
	Before      string
	Replacement string
}

func (s *Service) EditStage(ctx context.Context, actor auth.Actor, object, attribute string, edit StageEdit) error {
	edit.Stage = strings.TrimSpace(edit.Stage)
	edit.Name = strings.TrimSpace(edit.Name)
	edit.Before = strings.TrimSpace(edit.Before)
	edit.Replacement = strings.TrimSpace(edit.Replacement)
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return err
	}
	o, err := sc.object(object)
	if err != nil {
		return err
	}
	a, err := sc.attribute(o, attribute)
	if err != nil {
		return err
	}
	if a.Type != Status {
		return errs.Invalidf("%s is not a status attribute", attribute)
	}
	err = s.store.EditStatus(ctx, actor.WorkspaceID, a.ID, func(options []string) (storage.StatusChanges, error) {
		out, err := editStage(options, edit)
		if actor.UserID != "" {
			out.ActorID = &actor.UserID
		}
		return out, err
	})
	if errors.Is(err, storage.ErrConflict) {
		return errs.Invalidf("%s contains records; choose a replacement stage", edit.Stage)
	}
	return err
}

func editStage(options []string, edit StageEdit) (storage.StatusChanges, error) {
	index := func(name string) int {
		return slices.IndexFunc(options, func(stage string) bool { return strings.EqualFold(stage, name) })
	}
	i := index(edit.Stage)
	if i < 0 {
		return storage.StatusChanges{}, errs.Invalidf("no stage %q", edit.Stage)
	}
	old := options[i]
	out := storage.StatusChanges{Options: slices.Clone(options)}
	switch edit.Action {
	case "rename":
		if edit.Name == "" || len([]rune(edit.Name)) > 80 {
			return out, errs.Invalidf("a stage name is 1 to 80 characters")
		}
		if other := index(edit.Name); other >= 0 && other != i {
			return out, errs.Invalidf("stage %q already exists", edit.Name)
		}
		out.Options[i] = edit.Name
		if old != edit.Name {
			out.From, out.To = old, edit.Name
		}
	case "delete":
		if len(options) == 1 {
			return out, errs.Invalidf("keep at least one pipeline stage")
		}
		out.From = old
		if edit.Replacement != "" {
			j := index(edit.Replacement)
			if j < 0 || j == i {
				return out, errs.Invalidf("choose another stage for the records")
			}
			out.To = options[j]
		}
		out.Options = slices.Delete(out.Options, i, i+1)
	case "move":
		j := len(options)
		if edit.Before != "" {
			j = index(edit.Before)
			if j < 0 {
				return out, errs.Invalidf("no stage %q", edit.Before)
			}
		}
		if j > i {
			j--
		}
		out.Options = slices.Insert(slices.Delete(out.Options, i, i+1), j, old)
	default:
		return out, errs.Invalidf("action must be rename, delete or move")
	}
	return out, nil
}
