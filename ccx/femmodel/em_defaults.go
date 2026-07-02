// SPDX-License-Identifier: GPL-2.0-only
package femmodel

// EMDefaults holds the electromagnetic (electric-conduction) field-drive parameters synthesized
// at solve time. It is a study-wide template (not a browser-tree node), mirroring LoadDefaults,
// SupportDefaults, and ThermalDefaults. EMDriveMode is a neutral string here — the ccx layer maps
// it to its EMDrive display enum (voltage vs current).
type EMDefaults struct {
	EMDriveMode    string  // how the study is driven: applied "voltage" vs injected "current"
	VoltageV       float64 // prescribed potential on the first face (V) for the voltage mode
	CurrentDensity float64 // injected current density on the loaded faces for the current mode
}
