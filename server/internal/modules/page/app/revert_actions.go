package app

import "github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"

// revertAction identifies an original step and the restore members of its
// group selected in this batch, in their original order.
type revertAction struct {
	index   int
	members []int
}

func revertActions(steps []revertStep) []revertAction {
	var actions []revertAction
	begun := map[int32]bool{}
	for i, s := range steps {
		action := revertAction{index: i}
		if s.kind == domain.RevertRestore {
			if begun[s.group] {
				continue
			}
			begun[s.group] = true
			for j := i; j < len(steps); j++ {
				if steps[j].kind == domain.RevertRestore && steps[j].group == s.group {
					action.members = append(action.members, j)
				}
			}
		}
		actions = append(actions, action)
	}
	return actions
}
