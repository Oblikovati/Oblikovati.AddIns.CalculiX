// SPDX-License-Identifier: GPL-2.0-only

package ccx

// RunStudyCommandID is the host command the add-in registers; firing it (a ribbon click or
// the MCP bridge's execute_command) runs the FEA study on the active part.
const RunStudyCommandID = "CCX.RunStudy"

// AddConstraintCommandID snapshots the current face selection + builder parameters into a new
// study constraint; ClearConstraintsCommandID empties the explicit constraint list.
const (
	AddConstraintCommandID    = "CCX.AddConstraint"
	ClearConstraintsCommandID = "CCX.ClearConstraints"
)

// ShowPanelCommandID / ShowTreeCommandID re-open the study panel / Analysis tree from the ribbon.
const (
	ShowPanelCommandID = "CCX.ShowPanel"
	ShowTreeCommandID  = "CCX.ShowTree"
)

// ccxCommands is the exhaustive command list; RegisterCommands places each on the FEA tab.
var ccxCommands = []struct{ id, name, tip string }{
	{RunStudyCommandID, "Run Stress Analysis", "Mesh, solve, and visualize the stress and displacement of the active part with CalculiX."},
	{AddConstraintCommandID, "Add Constraint From Selection", "Add the selected face(s) as a study constraint of the chosen type."},
	{ClearConstraintsCommandID, "Clear Constraints", "Remove all study constraints added from selection."},
	{ShowPanelCommandID, "Study Panel", "Open the CalculiX study-parameters panel."},
	{ShowTreeCommandID, "Analysis Tree", "Open the CalculiX Analysis browser tree."},
}

// Setup performs the one-time host-facing initialization: register the study command, show
// the study-parameters panel, and declare the Analysis browser tree. It MUST NOT run on the
// host's session goroutine (host calls there block until the frame loop drains the dispatcher,
// deadlocking the head) — the cgo shell runs it on its own goroutine.
func (e *Engine) Setup() error {
	if err := e.RegisterCommands(); err != nil {
		return err
	}
	if _, err := e.ShowPanel(); err != nil {
		return err
	}
	_, err := e.ShowAnalysisTree()
	return err
}

// RegisterCommands registers every CalculiX command on the FEA ribbon tab (also invokable over
// the MCP bridge's execute_command). Command actions fire command.started, which Notify dispatches.
func (e *Engine) RegisterCommands() error {
	for _, c := range ccxCommands {
		if _, err := e.api.Commands().Create(commandArgs(c.id, c.name, c.tip)); err != nil {
			return err
		}
	}
	return nil
}
