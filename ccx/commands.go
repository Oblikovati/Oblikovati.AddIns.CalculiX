// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/api/wire"

// RunStudyCommandID is the host command the add-in registers; firing it (a ribbon click or
// the MCP bridge's execute_command) runs the FEA study on the active part.
const RunStudyCommandID = "CCX.RunStudy"

// AddConstraintCommandID snapshots the current face selection + builder parameters into a new
// study constraint; ClearConstraintsCommandID empties the explicit constraint list.
const (
	AddConstraintCommandID    = "CCX.AddConstraint"
	ClearConstraintsCommandID = "CCX.ClearConstraints"
)

// Setup performs the one-time host-facing initialization: register the study command and show
// the study-parameters panel. It MUST NOT run on the host's session goroutine (host calls
// there block until the frame loop drains the dispatcher, deadlocking the head) — the cgo
// shell runs it on its own goroutine.
func (e *Engine) Setup() error {
	if err := e.RegisterCommands(); err != nil {
		return err
	}
	_, err := e.ShowPanel()
	return err
}

// RegisterCommands registers the FEA study command with the host so it is invokable the same
// way a ribbon click is — including over the MCP bridge's execute_command. The host action is
// a no-op; executing the command fires command.started, which Notify turns into a study run.
func (e *Engine) RegisterCommands() error {
	if _, err := e.api.Commands().Create(wire.CreateCommandArgs{
		ID:          RunStudyCommandID,
		DisplayName: "Run Stress Analysis",
		Category:    "CalculiX",
		Tooltip:     "Mesh, solve, and visualize the stress and displacement of the active part with CalculiX.",
	}); err != nil {
		return err
	}
	if _, err := e.api.Commands().Create(wire.CreateCommandArgs{
		ID: AddConstraintCommandID, DisplayName: "Add Constraint From Selection", Category: "CalculiX",
		Tooltip: "Add the selected face(s) as a study constraint of the chosen type.",
	}); err != nil {
		return err
	}
	_, err := e.api.Commands().Create(wire.CreateCommandArgs{
		ID: ClearConstraintsCommandID, DisplayName: "Clear Constraints", Category: "CalculiX",
		Tooltip: "Remove all study constraints added from selection.",
	})
	return err
}
