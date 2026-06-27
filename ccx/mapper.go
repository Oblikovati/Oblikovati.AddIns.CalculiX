// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/api/wire"

// stressMapperName is the registered color mapper the stress flood plot uses.
const stressMapperName = "ccx.vonMises"

// stressColorStops is the blue→cyan→green→yellow→red ramp (rgba) the von Mises field is
// painted with: low stress blue, high stress red.
var stressColorStops = [][4]float32{
	{0.0, 0.0, 1.0, 1.0}, // blue
	{0.0, 1.0, 1.0, 1.0}, // cyan
	{0.0, 1.0, 0.0, 1.0}, // green
	{1.0, 1.0, 0.0, 1.0}, // yellow
	{1.0, 0.0, 0.0, 1.0}, // red
}

// stressMapper builds a color mapper spanning [0, peak] MPa across the ramp. A
// non-positive peak collapses to a unit range so the mapper stays valid.
func stressMapper(peakMPa float64) wire.GraphicsColorMapper {
	if peakMPa <= 0 {
		peakMPa = 1
	}
	n := len(stressColorStops)
	values := make([]float64, n)
	colors := make([]float32, 0, n*4)
	for i, stop := range stressColorStops {
		values[i] = peakMPa * float64(i) / float64(n-1)
		colors = append(colors, stop[0], stop[1], stop[2], stop[3])
	}
	return wire.GraphicsColorMapper{Values: values, Colors: colors}
}
