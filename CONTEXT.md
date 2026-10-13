# h-m-m.go

A keyboard-centric terminal mind map editor. A map is a single tree of text nodes that grows out from a central root to the left and right.

## Language

### Structure

**Map**:
The whole tree of nodes being edited, stored as one file.
_Avoid_: Document, outline, mindmap file

**Node**:
One piece of text in the map, drawn as a box.
_Avoid_: Item, entry, outline

**Root**:
The single node at the centre of the map that every other node descends from.

**Branch**:
A child of the root and everything below it; each branch has its own colour and grows on one side.
_Avoid_: Top-level node, category

**Subtree**:
A node together with all of its descendants.

### Collapsing

**Collapsed**:
A node whose descendants are hidden, while the node itself stays visible; a view state, not saved with the map.
_Avoid_: Folded, closed, minimised

**Expanded**:
A node that isn't collapsed; the default state.
_Avoid_: Unfolded, open

**Hidden**:
A node with a collapsed ancestor, so it isn't drawn and can't be reached by navigating or clicking, but is still part of the map.
_Avoid_: Invisible, removed

**Reveal**:
To expand just the ancestors of a hidden node that are needed to make it visible.
_Avoid_: Unhide, expand to

## Relationships

- A **Map** has exactly one **Root**
- The **Root** has zero or more **Branches**
- Every **Node** is either **Collapsed** or **Expanded**, except the **Root**, which is always **Expanded**
- A **Node** without children is always **Expanded**
- A **Node** is **Hidden** when any of its ancestors is **Collapsed**
- Selecting a **Hidden** node always **Reveals** it first
- For navigation, a **Collapsed** node is a leaf
- A **Map** always opens with every **Node** **Expanded**
- A **Hidden** node keeps its own **Collapsed**/**Expanded** state, which applies again once it's no longer hidden

## Example dialogue

> **Dev:** "If I collapse a **Branch** and then expand it again, does a **Node** I'd collapsed inside it come back expanded?"
> **Domain expert:** "No. That **Node** was only **Hidden**, and it's still **Collapsed**. Expanding the **Branch** shows it again as a collapsed box."

## Flagged ambiguities

- "collapse" can mean either a per-node state or a view-wide visible depth. Resolved: **Collapsed** is a state on a single **Node**. A depth-based view would be a command that sets many nodes to **Collapsed**, not a separate concept.
- "fold" was used interchangeably with "collapse". Resolved: **Collapsed**/**Expanded** are the canonical terms.
