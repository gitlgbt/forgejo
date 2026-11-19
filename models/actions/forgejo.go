// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/util"

	gouuid "github.com/google/uuid"
)

func RegisterRunner(ctx context.Context, ownerID, repoID int64, token string, labels *[]string, name, version string, ephemeral bool) (*ActionRunner, error) {
	uuid, err := gouuid.FromBytes([]byte(token[:16]))
	if err != nil {
		return nil, fmt.Errorf("gouuid.FromBytes %v", err)
	}
	uuidString := uuid.String()

	var runner ActionRunner

	has, err := db.GetEngine(ctx).Where("uuid=?", uuidString).Get(&runner)
	if err != nil {
		return nil, fmt.Errorf("GetRunner %v", err)
	}

	var mustUpdateSecret bool
	if has {
		//
		// The runner exists, check if the rest of the token has changed.
		//
		mustUpdateSecret = !runner.VerifyToken(token)
	} else {
		//
		// The runner does not exist yet, create it
		//
		runner = ActionRunner{
			UUID:        uuidString,
			AgentLabels: []string{},
		}

		runner.UpdateSecret(token)

		if err := CreateRunner(ctx, &runner); err != nil {
			return &runner, fmt.Errorf("can't create new runner %w", err)
		}
	}

	//
	// Update the existing runner
	//
	name, _ = util.SplitStringAtByteN(name, 255)

	cols := []string{"name", "owner_id", "repo_id", "version", "ephemeral"}
	runner.Name = name
	runner.OwnerID = ownerID
	runner.RepoID = repoID
	runner.Version = version
	runner.Ephemeral = ephemeral
	if labels != nil {
		runner.AgentLabels = *labels
		cols = append(cols, "agent_labels")
	}
	if mustUpdateSecret {
		runner.UpdateSecret(token)
		cols = append(cols, "token_hash", "token_salt")
	}

	if err := UpdateRunner(ctx, &runner, cols...); err != nil {
		return &runner, fmt.Errorf("can't update the runner %+v %w", runner, err)
	}

	return &runner, nil
}
