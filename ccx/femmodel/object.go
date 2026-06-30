// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// FEMObject is one first-class node of an Analysis tree. Every object has a stable id (unique
// within its Analysis), a category, and a display name shown in the browser tree.
type FEMObject interface {
	ObjectID() string
	Category() Category
	Name() string
}
